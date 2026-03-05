package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/QA-DNA/gowright/pkg/cdp"
)

type Frame struct {
	page     *Page
	session  *cdp.Session
	id       string
	parentID string
	name     string
	url      string

	mu             sync.Mutex
	children       map[string]*Frame
	lifecycle      map[string]bool
	inflight       int
	idleTimer      *time.Timer
	idleCh         chan struct{}
	executionCtxID int
}

func newFrame(page *Page, id, parentID, name, url string) *Frame {
	return &Frame{
		page:      page,
		session:   page.session,
		id:        id,
		parentID:  parentID,
		name:      name,
		url:       url,
		children:  make(map[string]*Frame),
		lifecycle: make(map[string]bool),
		idleCh:    make(chan struct{}, 1),
	}
}

func (f *Frame) ID() string { return f.id }
func (f *Frame) Name() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.name
}
func (f *Frame) URL() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.url
}

func (f *Frame) ParentFrame() *Frame {
	if f.parentID == "" {
		return nil
	}
	return f.page.frameByID(f.parentID)
}

func (f *Frame) ChildFrames() []*Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	frames := make([]*Frame, 0, len(f.children))
	for _, child := range f.children {
		frames = append(frames, child)
	}
	return frames
}

func (f *Frame) IsDetached() bool {
	return f.page.frameByID(f.id) == nil
}

func (f *Frame) Goto(url string) error {
	ctx := f.page.browser.ctx
	result, err := f.session.Call(ctx, "Page.navigate", map[string]any{
		"url":     url,
		"frameId": f.id,
	})
	if err != nil {
		return fmt.Errorf("frame navigate to %s: %w", url, err)
	}
	var resp struct {
		ErrorText string `json:"errorText"`
	}
	json.Unmarshal(result, &resp)
	if resp.ErrorText != "" {
		return fmt.Errorf("frame navigation error: %s", resp.ErrorText)
	}
	f.mu.Lock()
	f.url = url
	f.mu.Unlock()
	return nil
}

func (f *Frame) Evaluate(expression string) (json.RawMessage, error) {
	ctx := f.page.browser.ctx
	params := map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	}
	if cid := f.executionContextID(); cid != 0 {
		params["contextId"] = cid
	}
	result, err := f.session.Call(ctx, "Runtime.evaluate", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	json.Unmarshal(result, &resp)
	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}
	return resp.Result.Value, nil
}

func (f *Frame) executionContextID() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.executionCtxID
}

func (f *Frame) Locator(selector string) *Locator {
	js := fmt.Sprintf(`document.querySelector(%s)`, jsQuote(selector))
	return &Locator{page: f.page, selector: js, desc: selector, frame: f}
}

func (f *Frame) GetByText(text string, exact ...bool) *Locator {
	loc := f.page.GetByText(text, exact...)
	loc.frame = f
	return loc
}

func (f *Frame) GetByRole(role string, opts ...ByRoleOption) *Locator {
	loc := f.page.GetByRole(role, opts...)
	loc.frame = f
	return loc
}

func (f *Frame) GetByTestId(testID string) *Locator {
	loc := f.page.GetByTestId(testID)
	loc.frame = f
	return loc
}

func (f *Frame) GetByLabel(text string) *Locator {
	loc := f.page.GetByLabel(text)
	loc.frame = f
	return loc
}

func (f *Frame) GetByPlaceholder(text string) *Locator {
	loc := f.page.GetByPlaceholder(text)
	loc.frame = f
	return loc
}

func (f *Frame) GetByAltText(text string, exact ...bool) *Locator {
	loc := f.page.GetByAltText(text, exact...)
	loc.frame = f
	return loc
}

func (f *Frame) GetByTitle(text string, exact ...bool) *Locator {
	loc := f.page.GetByTitle(text, exact...)
	loc.frame = f
	return loc
}

func (f *Frame) Title() (string, error) {
	val, err := f.Evaluate("document.title")
	if err != nil {
		return "", err
	}
	var title string
	json.Unmarshal(val, &title)
	return title, nil
}

func (f *Frame) Content() (string, error) {
	val, err := f.Evaluate("document.documentElement.outerHTML")
	if err != nil {
		return "", err
	}
	var content string
	json.Unmarshal(val, &content)
	return content, nil
}

func (f *Frame) SetContent(htmlContent string) error {
	_, err := f.Evaluate(fmt.Sprintf("document.documentElement.innerHTML = %s", jsQuote(htmlContent)))
	return err
}

func (f *Frame) WaitForSelector(selector string, opts ...WaitForSelectorOptions) error {
	loc := f.Locator(selector)
	if len(opts) > 0 {
		return loc.WaitFor(WaitForOptions{State: opts[0].State, Timeout: opts[0].Timeout})
	}
	return loc.WaitFor()
}

func (f *Frame) WaitForFunction(expression string, opts ...WaitForFunctionOptions) (json.RawMessage, error) {
	return f.page.WaitForFunction(expression, opts...)
}

func (f *Frame) Page() *Page {
	return f.page
}

func (f *Frame) AddScriptTag(opts AddScriptTagOptions) error {
	if opts.Content != "" {
		_, err := f.Evaluate(fmt.Sprintf(`(function(){ var s = document.createElement('script'); s.textContent = %s; document.head.appendChild(s); })()`, jsQuote(opts.Content)))
		return err
	}
	if opts.URL != "" {
		_, err := f.Evaluate(fmt.Sprintf(`new Promise(function(resolve, reject) { var s = document.createElement('script'); s.src = %s; s.onload = resolve; s.onerror = reject; document.head.appendChild(s); })`, jsQuote(opts.URL)))
		return err
	}
	return nil
}

func (f *Frame) AddStyleTag(opts AddStyleTagOptions) error {
	if opts.Content != "" {
		_, err := f.Evaluate(fmt.Sprintf(`(function(){ var s = document.createElement('style'); s.textContent = %s; document.head.appendChild(s); })()`, jsQuote(opts.Content)))
		return err
	}
	if opts.URL != "" {
		_, err := f.Evaluate(fmt.Sprintf(`new Promise(function(resolve, reject) { var l = document.createElement('link'); l.rel = 'stylesheet'; l.href = %s; l.onload = resolve; l.onerror = reject; document.head.appendChild(l); })`, jsQuote(opts.URL)))
		return err
	}
	return nil
}

func (f *Frame) FrameLocator(selector string) *FrameLocator {
	return &FrameLocator{page: f.page, parentFrame: f, selector: selector}
}

func (f *Frame) WaitForLoadState(state string) error {
	ctx := f.page.browser.ctx
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	switch state {
	case "load":
		return f.waitForLifecycle(ctx, "load")
	case "domcontentloaded":
		return f.waitForLifecycle(ctx, "DOMContentLoaded")
	case "networkidle":
		return f.waitForNetworkIdle(ctx)
	default:
		return fmt.Errorf("unknown load state: %s", state)
	}
}

func (f *Frame) waitForLifecycle(ctx context.Context, event string) error {
	f.mu.Lock()
	if f.lifecycle[event] {
		f.mu.Unlock()
		return nil
	}
	f.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("frame %s: waitForLifecycle(%s) timed out", f.id, event)
		case <-time.After(50 * time.Millisecond):
			f.mu.Lock()
			if f.lifecycle[event] {
				f.mu.Unlock()
				return nil
			}
			f.mu.Unlock()
		}
	}
}

func (f *Frame) waitForNetworkIdle(ctx context.Context) error {
	for {
		f.mu.Lock()
		idle := f.inflight == 0
		f.mu.Unlock()
		if idle {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(500 * time.Millisecond):
				f.mu.Lock()
				stillIdle := f.inflight == 0
				f.mu.Unlock()
				if stillIdle {
					return nil
				}
			}
		} else {
			select {
			case <-ctx.Done():
				return fmt.Errorf("frame %s: waitForNetworkIdle timed out", f.id)
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

func (f *Frame) onLifecycleEvent(name string) {
	f.mu.Lock()
	f.lifecycle[name] = true
	f.mu.Unlock()
}

func (f *Frame) onNavigated(url, name string) {
	f.mu.Lock()
	f.url = url
	if name != "" {
		f.name = name
	}
	f.lifecycle = make(map[string]bool)
	f.mu.Unlock()
}

func (f *Frame) onRequestStarted() {
	f.mu.Lock()
	f.inflight++
	f.mu.Unlock()
}

func (f *Frame) onRequestFinished() {
	f.mu.Lock()
	f.inflight--
	if f.inflight < 0 {
		f.inflight = 0
	}
	f.mu.Unlock()
}

func (p *Page) initFrameTree() {
	ctx := p.browser.ctx
	result, err := p.session.Call(ctx, "Page.getFrameTree", nil)
	if err != nil {
		return
	}
	var resp struct {
		FrameTree frameTreeNode `json:"frameTree"`
	}
	json.Unmarshal(result, &resp)
	p.addFrameTree(&resp.FrameTree, "")
}

type frameTreeNode struct {
	Frame struct {
		ID       string `json:"id"`
		ParentID string `json:"parentId"`
		Name     string `json:"name"`
		URL      string `json:"url"`
	} `json:"frame"`
	ChildFrames []frameTreeNode `json:"childFrames"`
}

func (p *Page) addFrameTree(node *frameTreeNode, parentID string) {
	f := newFrame(p, node.Frame.ID, parentID, node.Frame.Name, node.Frame.URL)
	p.frameMu.Lock()
	p.frames[node.Frame.ID] = f
	if parentID == "" {
		p.mainFrame = f
	}
	p.frameMu.Unlock()

	if parentID != "" {
		parent := p.frameByID(parentID)
		if parent != nil {
			parent.mu.Lock()
			parent.children[node.Frame.ID] = f
			parent.mu.Unlock()
		}
	}

	for i := range node.ChildFrames {
		p.addFrameTree(&node.ChildFrames[i], node.Frame.ID)
	}
}

func (p *Page) frameByID(id string) *Frame {
	p.frameMu.RLock()
	defer p.frameMu.RUnlock()
	return p.frames[id]
}

func (p *Page) MainFrame() *Frame {
	p.frameMu.RLock()
	defer p.frameMu.RUnlock()
	return p.mainFrame
}

func (p *Page) Frames() []*Frame {
	p.frameMu.RLock()
	defer p.frameMu.RUnlock()
	frames := make([]*Frame, 0, len(p.frames))
	for _, f := range p.frames {
		frames = append(frames, f)
	}
	return frames
}

func (p *Page) Frame(nameOrURL string) *Frame {
	p.frameMu.RLock()
	defer p.frameMu.RUnlock()
	for _, f := range p.frames {
		if f.name == nameOrURL || f.url == nameOrURL || strings.Contains(f.url, nameOrURL) {
			return f
		}
	}
	return nil
}

func (p *Page) FrameLocator(selector string) *FrameLocator {
	return &FrameLocator{page: p, selector: selector}
}

func (p *Page) setupFrameEvents() {
	p.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Page.frameAttached":
			var payload struct {
				FrameID       string `json:"frameId"`
				ParentFrameID string `json:"parentFrameId"`
			}
			json.Unmarshal(params, &payload)
			f := newFrame(p, payload.FrameID, payload.ParentFrameID, "", "")
			p.frameMu.Lock()
			p.frames[payload.FrameID] = f
			p.frameMu.Unlock()
			parent := p.frameByID(payload.ParentFrameID)
			if parent != nil {
				parent.mu.Lock()
				parent.children[payload.FrameID] = f
				parent.mu.Unlock()
			}
		case "Page.frameDetached":
			var payload struct {
				FrameID string `json:"frameId"`
			}
			json.Unmarshal(params, &payload)
			f := p.frameByID(payload.FrameID)
			if f != nil && f.parentID != "" {
				parent := p.frameByID(f.parentID)
				if parent != nil {
					parent.mu.Lock()
					delete(parent.children, payload.FrameID)
					parent.mu.Unlock()
				}
			}
			p.frameMu.Lock()
			delete(p.frames, payload.FrameID)
			p.frameMu.Unlock()
		case "Page.frameNavigated":
			var payload struct {
				Frame struct {
					ID   string `json:"id"`
					URL  string `json:"url"`
					Name string `json:"name"`
				} `json:"frame"`
			}
			json.Unmarshal(params, &payload)
			f := p.frameByID(payload.Frame.ID)
			if f != nil {
				f.onNavigated(payload.Frame.URL, payload.Frame.Name)
			}
		case "Page.lifecycleEvent":
			var payload struct {
				FrameID string `json:"frameId"`
				Name    string `json:"name"`
			}
			json.Unmarshal(params, &payload)
			f := p.frameByID(payload.FrameID)
			if f != nil {
				f.onLifecycleEvent(payload.Name)
			}
		}
	})
}

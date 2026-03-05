package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

type Page struct {
	session  *cdp.Session
	targetID string
	url      string
	browser  *Browser
	context  *BrowserContext

	mu              sync.Mutex
	eventHandlers   map[string][]eventHandler
	dialogHandler   func(*Dialog)
	routeMgr        *routeManager
	locatorHandlers map[string]func(*Locator)

	frameMu   sync.RWMutex
	frames    map[string]*Frame
	mainFrame *Frame

	closed                  bool
	defaultTimeout          time.Duration
	defaultNavigationTimeout time.Duration
	viewportWidth           int
	viewportHeight          int
	networkRequests         sync.Map

	keyboard      *Keyboard
	mouse         *Mouse
	touchscreen   *Touchscreen
	clock         *Clock
	workers       []*Worker
	tracing       *Tracing
	coverage      *Coverage
	accessibility *Accessibility
	video         *Video
}

type eventHandler struct {
	fn   func(any)
	once bool
}

// TargetID returns the CDP target ID for this page.
func (p *Page) TargetID() string {
	return p.targetID
}

// URL returns the current page URL.
func (p *Page) URL() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.url
}

func (p *Page) setURL(url string) {
	p.mu.Lock()
	p.url = url
	p.mu.Unlock()
}

// Session returns the underlying CDP session.
func (p *Page) Session() *cdp.Session {
	return p.session
}

// Goto navigates the page to the given URL.
func (p *Page) Goto(url string) error {
	ctx := p.browser.ctx

	// Enable page events so we can track navigation
	_, err := p.session.Call(ctx, "Page.enable", nil)
	if err != nil {
		return fmt.Errorf("enable page events: %w", err)
	}

	// Register load event listener BEFORE navigating to avoid race with fast-loading pages
	loadCh := make(chan struct{}, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.loadEventFired" {
			select {
			case loadCh <- struct{}{}:
			default:
			}
		}
	})

	result, err := p.session.Call(ctx, "Page.navigate", map[string]any{
		"url": url,
	})
	if err != nil {
		return fmt.Errorf("navigate to %s: %w", url, err)
	}

	var resp struct {
		FrameID   string `json:"frameId"`
		ErrorText string `json:"errorText"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return err
	}

	if resp.ErrorText != "" {
		return fmt.Errorf("navigation error: %s", resp.ErrorText)
	}

	p.setURL(url)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-loadCh:
		return nil
	}
}

// Evaluate runs JavaScript in the page and returns the result.
func (p *Page) Evaluate(expression string) (json.RawMessage, error) {
	ctx := p.browser.ctx

	result, err := p.session.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}

	var resp struct {
		Result struct {
			Type  string          `json:"type"`
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}

	return resp.Result.Value, nil
}

// Title returns the page title.
func (p *Page) Title() (string, error) {
	val, err := p.Evaluate("document.title")
	if err != nil {
		return "", err
	}

	var title string
	if err := json.Unmarshal(val, &title); err != nil {
		return "", err
	}
	return title, nil
}

// Screenshot captures a screenshot of the page as PNG bytes.
func (p *Page) Screenshot() ([]byte, error) {
	ctx := p.browser.ctx

	result, err := p.session.Call(ctx, "Page.captureScreenshot", map[string]any{
		"format": "png",
	})
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}

	var resp struct {
		Data string `json:"data"` // base64 encoded
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	return base64.StdEncoding.DecodeString(resp.Data)
}

// Close closes this page/tab.
func (p *Page) Close() error {
	_, err := p.browser.conn.RootSession().Call(p.browser.ctx, "Target.closeTarget", map[string]any{
		"targetId": p.targetID,
	})
	if err == nil {
		p.mu.Lock()
		p.closed = true
		p.mu.Unlock()
	}
	return err
}

// waitForLoadEvent waits for the Page.loadEventFired CDP event.
func (p *Page) waitForLoadEvent(ctx context.Context) error {
	ch := make(chan struct{}, 1)

	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.loadEventFired" {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	})

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// --- Navigation ---

// Reload reloads the current page.
func (p *Page) Reload() error {
	ctx := p.browser.ctx

	loadCh := make(chan struct{}, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.loadEventFired" {
			select {
			case loadCh <- struct{}{}:
			default:
			}
		}
	})

	_, err := p.session.Call(ctx, "Page.reload", nil)
	if err != nil {
		return fmt.Errorf("reload: %w", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-loadCh:
		return nil
	}
}

// GoBack navigates to the previous history entry.
func (p *Page) GoBack() error {
	ctx := p.browser.ctx
	result, err := p.session.Call(ctx, "Page.getNavigationHistory", nil)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}
	var nav struct {
		CurrentIndex int `json:"currentIndex"`
		Entries      []struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(result, &nav); err != nil {
		return err
	}
	if nav.CurrentIndex <= 0 {
		return nil
	}
	entry := nav.Entries[nav.CurrentIndex-1]

	loadCh := make(chan struct{}, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.loadEventFired" {
			select {
			case loadCh <- struct{}{}:
			default:
			}
		}
	})

	_, err = p.session.Call(ctx, "Page.navigateToHistoryEntry", map[string]any{"entryId": entry.ID})
	if err != nil {
		return fmt.Errorf("go back: %w", err)
	}
	p.setURL(entry.URL)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-loadCh:
		return nil
	}
}

// GoForward navigates to the next history entry.
func (p *Page) GoForward() error {
	ctx := p.browser.ctx
	result, err := p.session.Call(ctx, "Page.getNavigationHistory", nil)
	if err != nil {
		return fmt.Errorf("get history: %w", err)
	}
	var nav struct {
		CurrentIndex int `json:"currentIndex"`
		Entries      []struct {
			ID  int    `json:"id"`
			URL string `json:"url"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(result, &nav); err != nil {
		return err
	}
	if nav.CurrentIndex >= len(nav.Entries)-1 {
		return nil
	}
	entry := nav.Entries[nav.CurrentIndex+1]

	loadCh := make(chan struct{}, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.loadEventFired" {
			select {
			case loadCh <- struct{}{}:
			default:
			}
		}
	})

	_, err = p.session.Call(ctx, "Page.navigateToHistoryEntry", map[string]any{"entryId": entry.ID})
	if err != nil {
		return fmt.Errorf("go forward: %w", err)
	}
	p.setURL(entry.URL)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-loadCh:
		return nil
	}
}

// Content returns the full HTML content of the page.
func (p *Page) Content() (string, error) {
	result, err := p.session.Call(p.browser.ctx, "DOM.getDocument", map[string]any{"depth": 0})
	if err != nil {
		return "", fmt.Errorf("get document: %w", err)
	}
	var doc struct {
		Root struct {
			NodeID int `json:"nodeId"`
		} `json:"root"`
	}
	if err := json.Unmarshal(result, &doc); err != nil {
		return "", err
	}
	result, err = p.session.Call(p.browser.ctx, "DOM.getOuterHTML", map[string]any{
		"nodeId": doc.Root.NodeID,
	})
	if err != nil {
		return "", fmt.Errorf("get outer html: %w", err)
	}
	var html struct {
		OuterHTML string `json:"outerHTML"`
	}
	if err := json.Unmarshal(result, &html); err != nil {
		return "", err
	}
	return html.OuterHTML, nil
}

// SetContent sets the page HTML content directly.
func (p *Page) SetContent(htmlContent string) error {
	_, err := p.session.Call(p.browser.ctx, "Page.setDocumentContent", map[string]any{
		"frameId": p.mainFrameID(),
		"html":    htmlContent,
	})
	if err != nil {
		return fmt.Errorf("set content: %w", err)
	}
	return nil
}

// mainFrameID returns the main frame ID for this page.
func (p *Page) mainFrameID() string {
	result, err := p.session.Call(p.browser.ctx, "Page.getFrameTree", nil)
	if err != nil {
		return ""
	}
	var resp struct {
		FrameTree struct {
			Frame struct {
				ID string `json:"id"`
			} `json:"frame"`
		} `json:"frameTree"`
	}
	json.Unmarshal(result, &resp)
	return resp.FrameTree.Frame.ID
}

// SetViewportSize sets the page viewport dimensions.
// Matches Playwright: resizes the browser window (in headed mode) AND sets
// the emulated device metrics so the content area matches the requested size.
func (p *Page) SetViewportSize(width, height int) error {
	ctx := p.browser.ctx

	// Resize the OS window to accommodate the viewport (headed mode).
	// Playwright does: Browser.getWindowForTarget → Browser.setWindowBounds
	// with platform-specific chrome insets. In headless mode this is a no-op.
	result, err := p.session.Call(ctx, "Browser.getWindowForTarget", nil)
	if err == nil {
		var resp struct {
			WindowID int `json:"windowId"`
		}
		json.Unmarshal(result, &resp)
		if resp.WindowID != 0 {
			// Platform-specific browser chrome insets (macOS defaults)
			insetW, insetH := 2, 80
			p.session.Call(ctx, "Browser.setWindowBounds", map[string]any{
				"windowId": resp.WindowID,
				"bounds": map[string]any{
					"width":  width + insetW,
					"height": height + insetH,
				},
			})
		}
	}

	_, err = p.session.Call(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             width,
		"height":            height,
		"deviceScaleFactor": 1,
		"mobile":            false,
	})
	if err != nil {
		return fmt.Errorf("set viewport: %w", err)
	}
	p.mu.Lock()
	p.viewportWidth = width
	p.viewportHeight = height
	p.mu.Unlock()
	return nil
}

// --- Wait methods ---

// WaitForLoadState waits for the specified load lifecycle event.
func (p *Page) WaitForLoadState(state string) error {
	ctx := p.browser.ctx
	switch state {
	case "load":
		return p.waitForLoadEvent(ctx)
	case "domcontentloaded":
		return p.waitForCDPEvent(ctx, "Page.domContentEventFired")
	case "networkidle":
		return p.waitForNetworkIdle(ctx, 500*time.Millisecond)
	default:
		return fmt.Errorf("unknown load state: %s", state)
	}
}

// WaitForURL waits until the page URL contains the given pattern.
type WaitForURLOptions struct {
	Timeout time.Duration
	Regex   *regexp.Regexp
}

func (p *Page) WaitForURL(pattern string, opts ...WaitForURLOptions) error {
	ctx := p.browser.ctx
	timeout := 30 * time.Second
	var re *regexp.Regexp
	if len(opts) > 0 {
		if opts[0].Timeout > 0 {
			timeout = opts[0].Timeout
		}
		re = opts[0].Regex
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		val, err := p.Evaluate("window.location.href")
		if err != nil {
			return err
		}
		var url string
		json.Unmarshal(val, &url)

		matched := false
		if re != nil {
			matched = re.MatchString(url)
		} else {
			matched = matchPattern(pattern, url)
		}

		if matched {
			p.setURL(url)
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("waitForURL(%q) timed out, current: %s", pattern, url)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// WaitForSelector waits for a CSS selector to reach the given state.
func (p *Page) WaitForSelector(selector string, opts ...WaitForSelectorOptions) error {
	state := "visible"
	timeout := 30 * time.Second
	if len(opts) > 0 {
		if opts[0].State != "" {
			state = opts[0].State
		}
		if opts[0].Timeout > 0 {
			timeout = opts[0].Timeout
		}
	}

	ctx, cancel := context.WithTimeout(p.browser.ctx, timeout)
	defer cancel()

	for {
		var js string
		switch state {
		case "attached":
			js = fmt.Sprintf("!!document.querySelector(%s)", jsQuote(selector))
		case "detached":
			js = fmt.Sprintf("!document.querySelector(%s)", jsQuote(selector))
		case "visible":
			js = fmt.Sprintf(`(function() {
				const el = document.querySelector(%s);
				if (!el) return false;
				const s = window.getComputedStyle(el);
				return s.display !== 'none' && s.visibility !== 'hidden' && el.offsetWidth > 0;
			})()`, jsQuote(selector))
		case "hidden":
			js = fmt.Sprintf(`(function() {
				const el = document.querySelector(%s);
				if (!el) return true;
				const s = window.getComputedStyle(el);
				return s.display === 'none' || s.visibility === 'hidden' || el.offsetWidth === 0;
			})()`, jsQuote(selector))
		default:
			return fmt.Errorf("unknown state: %s", state)
		}

		val, err := p.Evaluate(js)
		if err == nil {
			var ok bool
			json.Unmarshal(val, &ok)
			if ok {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waitForSelector(%q, state=%s) timed out", selector, state)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// WaitForSelectorOptions configures WaitForSelector.
type WaitForSelectorOptions struct {
	State   string
	Timeout time.Duration
}

// --- Events ---

// On registers a persistent event listener.
func (p *Page) On(event string, handler func(any)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.eventHandlers == nil {
		p.eventHandlers = make(map[string][]eventHandler)
	}
	p.eventHandlers[event] = append(p.eventHandlers[event], eventHandler{fn: handler})
}

// Once registers a one-time event listener.
func (p *Page) Once(event string, handler func(any)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.eventHandlers == nil {
		p.eventHandlers = make(map[string][]eventHandler)
	}
	p.eventHandlers[event] = append(p.eventHandlers[event], eventHandler{fn: handler, once: true})
}

// WaitForEvent waits for a named event and returns its payload.
func (p *Page) WaitForEvent(event string, timeout ...time.Duration) (any, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	ch := make(chan any, 1)
	p.Once(event, func(payload any) {
		select {
		case ch <- payload:
		default:
		}
	})
	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForEvent(%q) timed out", event)
	}
}

type WaitForEventOptions struct {
	Predicate func(any) bool
	Timeout   time.Duration
}

func (p *Page) WaitForEventWith(event string, opts WaitForEventOptions) (any, error) {
	t := opts.Timeout
	if t == 0 {
		t = 30 * time.Second
	}
	ch := make(chan any, 1)
	p.On(event, func(payload any) {
		if opts.Predicate == nil || opts.Predicate(payload) {
			select {
			case ch <- payload:
			default:
			}
		}
	})
	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForEvent(%q) timed out", event)
	}
}

// emitEvent fires handlers for the given event.
func (p *Page) emitEvent(event string, payload any) {
	p.mu.Lock()
	handlers := make([]eventHandler, len(p.eventHandlers[event]))
	copy(handlers, p.eventHandlers[event])
	remaining := p.eventHandlers[event][:0]
	for _, h := range p.eventHandlers[event] {
		if !h.once {
			remaining = append(remaining, h)
		}
	}
	p.eventHandlers[event] = remaining
	p.mu.Unlock()

	for _, h := range handlers {
		h.fn(payload)
	}
}

// --- Dialog handling ---

// Dialog represents a JavaScript dialog (alert, confirm, prompt, beforeunload).
type Dialog struct {
	session      *cdp.Session
	ctx          context.Context
	dialogType   string
	message      string
	defaultValue string
}

// Type returns the dialog type.
func (d *Dialog) Type() string { return d.dialogType }

// Message returns the dialog message.
func (d *Dialog) Message() string { return d.message }

// DefaultValue returns the default prompt value.
func (d *Dialog) DefaultValue() string { return d.defaultValue }

// Accept accepts the dialog, optionally providing text for prompts.
func (d *Dialog) Accept(promptText ...string) error {
	params := map[string]any{"accept": true}
	if len(promptText) > 0 {
		params["promptText"] = promptText[0]
	}
	_, err := d.session.Call(d.ctx, "Page.handleJavaScriptDialog", params)
	return err
}

// Dismiss dismisses the dialog.
func (d *Dialog) Dismiss() error {
	_, err := d.session.Call(d.ctx, "Page.handleJavaScriptDialog", map[string]any{"accept": false})
	return err
}

// OnDialog registers a handler for JavaScript dialogs.
func (p *Page) OnDialog(handler func(*Dialog)) {
	p.mu.Lock()
	p.dialogHandler = handler
	p.mu.Unlock()
}

// setupPageEvents enables CDP events and hooks up dialog/console handlers.
func (p *Page) setupPageEvents() {
	p.session.Call(p.browser.ctx, "Runtime.enable", nil)
	p.session.Call(p.browser.ctx, "Network.enable", nil)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Network.requestWillBeSent":
			var payload struct {
				RequestID string `json:"requestId"`
				Request   struct {
					URL      string            `json:"url"`
					Method   string            `json:"method"`
					Headers  map[string]string `json:"headers"`
					PostData string            `json:"postData"`
				} `json:"request"`
				Type      string  `json:"type"`
				Timestamp float64 `json:"timestamp"`
			}
			json.Unmarshal(params, &payload)
			req := &Request{
				URL:          payload.Request.URL,
				Method:       payload.Request.Method,
				Headers:      payload.Request.Headers,
				PostData:     payload.Request.PostData,
				resourceType: payload.Type,
				frame:        p.mainFrame,
				timing:       &RequestTiming{StartTime: payload.Timestamp},
			}
			p.networkRequests.Store(payload.RequestID, req)
			p.emitEvent("request", req)
		case "Network.responseReceived":
			var payload struct {
				RequestID string `json:"requestId"`
				Response  struct {
					URL        string            `json:"url"`
					Status     int               `json:"status"`
					StatusText string            `json:"statusText"`
					Headers    map[string]string `json:"headers"`
					Timing     struct {
						DNSStart  float64 `json:"dnsStart"`
						DNSEnd    float64 `json:"dnsEnd"`
						ConnStart float64 `json:"connectStart"`
						SSLStart  float64 `json:"sslStart"`
						ConnEnd   float64 `json:"connectEnd"`
						SendStart float64 `json:"sendStart"`
						SendEnd   float64 `json:"sendEnd"`
						RecvStart float64 `json:"receiveHeadersStart"`
					} `json:"timing"`
					FromServiceWorker bool `json:"fromServiceWorker"`
					FromDiskCache     bool `json:"fromDiskCache"`
				} `json:"response"`
			}
			json.Unmarshal(params, &payload)
			var req *Request
			if v, ok := p.networkRequests.Load(payload.RequestID); ok {
				req = v.(*Request)
				if req.timing != nil {
					req.timing.DomainLookupStart = payload.Response.Timing.DNSStart
					req.timing.DomainLookupEnd = payload.Response.Timing.DNSEnd
					req.timing.ConnectStart = payload.Response.Timing.ConnStart
					req.timing.SecureConnectionStart = payload.Response.Timing.SSLStart
					req.timing.ConnectEnd = payload.Response.Timing.ConnEnd
					req.timing.RequestStart = payload.Response.Timing.SendStart
					req.timing.ResponseStart = payload.Response.Timing.RecvStart
				}
			}
			resp := &Response{
				URL:               payload.Response.URL,
				Status:            payload.Response.Status,
				Headers:           payload.Response.Headers,
				session:           p.session,
				ctx:               p.browser.ctx,
				reqID:             payload.RequestID,
				request:           req,
				statusText:        payload.Response.StatusText,
				fromServiceWorker: payload.Response.FromServiceWorker,
				fromCache:         payload.Response.FromDiskCache,
			}
			if req != nil {
				req.response = resp
			}
			p.emitEvent("response", resp)
		case "Network.loadingFinished":
			var payload struct {
				RequestID         string  `json:"requestId"`
				EncodedDataLength float64 `json:"encodedDataLength"`
				Timestamp         float64 `json:"timestamp"`
			}
			json.Unmarshal(params, &payload)
			if v, ok := p.networkRequests.Load(payload.RequestID); ok {
				req := v.(*Request)
				if req.timing != nil {
					req.timing.ResponseEnd = payload.Timestamp
				}
				if req.sizes == nil {
					req.sizes = &RequestSizes{}
				}
				req.sizes.ResponseBodySize = int(payload.EncodedDataLength)
				p.networkRequests.Delete(payload.RequestID)
			}
			p.emitEvent("requestfinished", payload.RequestID)
		case "Network.loadingFailed":
			var payload struct {
				RequestID    string `json:"requestId"`
				ErrorText    string `json:"errorText"`
			}
			json.Unmarshal(params, &payload)
			if v, ok := p.networkRequests.Load(payload.RequestID); ok {
				req := v.(*Request)
				req.failure = payload.ErrorText
				p.networkRequests.Delete(payload.RequestID)
			}
			p.emitEvent("requestfailed", payload.RequestID)
		case "Page.javascriptDialogOpening":
			var payload struct {
				Type         string `json:"type"`
				Message      string `json:"message"`
				DefaultValue string `json:"defaultPrompt"`
			}
			json.Unmarshal(params, &payload)
			dialog := &Dialog{
				session:      p.session,
				ctx:          p.browser.ctx,
				dialogType:   payload.Type,
				message:      payload.Message,
				defaultValue: payload.DefaultValue,
			}
			p.mu.Lock()
			handler := p.dialogHandler
			p.mu.Unlock()
			if handler != nil {
				handler(dialog)
			} else {
				dialog.Dismiss()
			}
			p.emitEvent("dialog", dialog)
		case "Runtime.consoleAPICalled":
			var payload struct {
				Type string `json:"type"`
				Args []struct {
					Type  string          `json:"type"`
					Value json.RawMessage `json:"value"`
				} `json:"args"`
				StackTrace *struct {
					CallFrames []struct {
						URL    string `json:"url"`
						Line   int    `json:"lineNumber"`
						Column int    `json:"columnNumber"`
					} `json:"callFrames"`
				} `json:"stackTrace"`
			}
			json.Unmarshal(params, &payload)
			args := make([]json.RawMessage, len(payload.Args))
			textParts := make([]string, 0, len(payload.Args))
			for i, a := range payload.Args {
				args[i] = a.Value
				var s string
				if json.Unmarshal(a.Value, &s) == nil {
					textParts = append(textParts, s)
				} else {
					textParts = append(textParts, string(a.Value))
				}
			}
			msg := &ConsoleMessage{
				typ:  payload.Type,
				args: args,
				page: p,
			}
			if len(textParts) > 0 {
				msg.text = strings.Join(textParts, " ")
			}
			if payload.StackTrace != nil && len(payload.StackTrace.CallFrames) > 0 {
				cf := payload.StackTrace.CallFrames[0]
				msg.location = ConsoleMessageLocation{URL: cf.URL, Line: cf.Line, Column: cf.Column}
			}
			p.emitEvent("console", msg)
		case "Runtime.exceptionThrown":
			var payload struct {
				ExceptionDetails struct {
					Text string `json:"text"`
				} `json:"exceptionDetails"`
			}
			json.Unmarshal(params, &payload)
			p.emitEvent("pageerror", payload.ExceptionDetails.Text)
		case "Runtime.executionContextCreated":
			var payload struct {
				Context struct {
					ID      int `json:"id"`
					AuxData struct {
						FrameID   string `json:"frameId"`
						IsDefault bool   `json:"isDefault"`
					} `json:"auxData"`
				} `json:"context"`
			}
			json.Unmarshal(params, &payload)
			if payload.Context.AuxData.IsDefault && payload.Context.AuxData.FrameID != "" {
				f := p.frameByID(payload.Context.AuxData.FrameID)
				if f != nil {
					f.mu.Lock()
					f.executionCtxID = payload.Context.ID
					f.mu.Unlock()
				}
			}
		case "Runtime.executionContextDestroyed":
			var payload struct {
				ExecutionContextID int `json:"executionContextId"`
			}
			json.Unmarshal(params, &payload)
			p.frameMu.RLock()
			for _, f := range p.frames {
				f.mu.Lock()
				if f.executionCtxID == payload.ExecutionContextID {
					f.executionCtxID = 0
				}
				f.mu.Unlock()
			}
			p.frameMu.RUnlock()
		case "Runtime.executionContextsCleared":
			p.frameMu.RLock()
			for _, f := range p.frames {
				f.mu.Lock()
				f.executionCtxID = 0
				f.mu.Unlock()
			}
			p.frameMu.RUnlock()
		}
	})
}

// --- Screenshot with options ---

// ScreenshotOptions configures page screenshots.
type ScreenshotOptions struct {
	FullPage bool
	Type     string // "png" | "jpeg"
	Quality  int
	Clip     *Rect
}

// Rect defines a rectangular region.
type Rect struct {
	X, Y, Width, Height float64
}

// ScreenshotWithOptions captures a screenshot with options.
func (p *Page) ScreenshotWithOptions(opts ScreenshotOptions) ([]byte, error) {
	ctx := p.browser.ctx
	format := opts.Type
	if format == "" {
		format = "png"
	}

	params := map[string]any{"format": format}
	if format == "jpeg" && opts.Quality > 0 {
		params["quality"] = opts.Quality
	}
	if opts.FullPage {
		params["captureBeyondViewport"] = true
		val, err := p.Evaluate(`JSON.stringify({width: document.documentElement.scrollWidth, height: document.documentElement.scrollHeight})`)
		if err == nil {
			var dimsStr string
			json.Unmarshal(val, &dimsStr)
			var dims struct {
				Width  float64 `json:"width"`
				Height float64 `json:"height"`
			}
			json.Unmarshal([]byte(dimsStr), &dims)
			params["clip"] = map[string]any{
				"x": 0, "y": 0, "width": dims.Width, "height": dims.Height, "scale": 1,
			}
		}
	}
	if opts.Clip != nil {
		params["clip"] = map[string]any{
			"x": opts.Clip.X, "y": opts.Clip.Y,
			"width": opts.Clip.Width, "height": opts.Clip.Height, "scale": 1,
		}
	}

	result, err := p.session.Call(ctx, "Page.captureScreenshot", params)
	if err != nil {
		return nil, fmt.Errorf("screenshot: %w", err)
	}
	var resp struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	return base64.StdEncoding.DecodeString(resp.Data)
}

// --- Evaluate with argument ---

// EvaluateWithArg runs JavaScript with a Go value passed as an argument.
func (p *Page) EvaluateWithArg(expression string, arg any) (json.RawMessage, error) {
	ctx := p.browser.ctx
	argJSON, err := json.Marshal(arg)
	if err != nil {
		return nil, fmt.Errorf("marshal arg: %w", err)
	}
	result, err := p.session.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    fmt.Sprintf("(%s)(%s)", expression, string(argJSON)),
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, fmt.Errorf("evaluate: %w", err)
	}
	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}
	return resp.Result.Value, nil
}

// --- Internal helpers ---

// waitForCDPEvent waits for a specific CDP event method.
func (p *Page) waitForCDPEvent(ctx context.Context, method string) error {
	ch := make(chan struct{}, 1)
	p.session.OnEvent(func(m string, params json.RawMessage) {
		if m == method {
			select {
			case ch <- struct{}{}:
			default:
			}
		}
	})
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// waitForNetworkIdle waits until no network requests for the given duration.
func (p *Page) waitForNetworkIdle(ctx context.Context, idleDuration time.Duration) error {
	p.session.Call(ctx, "Network.enable", nil)

	inflight := 0
	lastActivity := time.Now()
	ch := make(chan struct{}, 1)

	p.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Network.requestWillBeSent":
			inflight++
			lastActivity = time.Now()
		case "Network.loadingFinished", "Network.loadingFailed":
			inflight--
			if inflight < 0 {
				inflight = 0
			}
			lastActivity = time.Now()
			if inflight == 0 {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	})

	timer := time.NewTimer(idleDuration)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ch:
			timer.Reset(idleDuration)
		case <-timer.C:
			if inflight == 0 && time.Since(lastActivity) >= idleDuration {
				return nil
			}
			timer.Reset(100 * time.Millisecond)
		}
	}
}

func (p *Page) WaitForFunction(expression string, opts ...WaitForFunctionOptions) (json.RawMessage, error) {
	timeout := defaultTimeout
	if len(opts) > 0 && opts[0].Timeout > 0 {
		timeout = opts[0].Timeout
	}
	polling := 100 * time.Millisecond
	if len(opts) > 0 && opts[0].Polling > 0 {
		polling = opts[0].Polling
	}
	ctx, cancel := context.WithTimeout(p.browser.ctx, timeout)
	defer cancel()

	var result json.RawMessage
	err := retryWithBackoff(ctx, func() error {
		js := fmt.Sprintf(`(function(){ var r = %s; return r ? true : false; })()`, expression)
		val, err := p.Evaluate(js)
		if err != nil {
			return err
		}
		var ok bool
		json.Unmarshal(val, &ok)
		if !ok {
			return fmt.Errorf("function returned falsy")
		}
		v, err := p.Evaluate(expression)
		if err != nil {
			return err
		}
		result = v
		return nil
	})
	_ = polling
	return result, err
}

type WaitForFunctionOptions struct {
	Timeout time.Duration
	Polling time.Duration
}

func (p *Page) WaitForPopup(opts ...WaitForPopupOptions) (*Page, error) {
	timeout := defaultTimeout
	if len(opts) > 0 && opts[0].Timeout > 0 {
		timeout = opts[0].Timeout
	}

	ch := make(chan *Page, 1)
	p.browser.conn.RootSession().OnEvent(func(method string, params json.RawMessage) {
		if method == "Target.attachedToTarget" {
			var payload struct {
				TargetInfo struct {
					TargetID         string `json:"targetId"`
					Type             string `json:"type"`
					OpenerID         string `json:"openerId"`
					BrowserContextID string `json:"browserContextId"`
				} `json:"targetInfo"`
			}
			json.Unmarshal(params, &payload)
			if payload.TargetInfo.Type == "page" && payload.TargetInfo.OpenerID == p.targetID {
				p.browser.mu.Lock()
				bc := p.browser.contexts[payload.TargetInfo.BrowserContextID]
				p.browser.mu.Unlock()
				if bc != nil {
					popupPage := bc.waitForPage(p.browser.ctx, payload.TargetInfo.TargetID)
					if popupPage != nil {
						select {
						case ch <- popupPage:
						default:
						}
					}
				}
			}
		}
	})

	ctx, cancel := context.WithTimeout(p.browser.ctx, timeout)
	defer cancel()
	select {
	case popup := <-ch:
		return popup, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForPopup timed out")
	}
}

type WaitForPopupOptions struct {
	Timeout time.Duration
}

func (p *Page) AddScriptTag(opts AddScriptTagOptions) error {
	var js string
	if opts.URL != "" {
		js = fmt.Sprintf(`new Promise(function(resolve, reject) {
			var s = document.createElement('script');
			s.src = %s;
			s.onload = resolve;
			s.onerror = reject;
			document.head.appendChild(s);
		})`, jsQuote(opts.URL))
	} else if opts.Content != "" {
		js = fmt.Sprintf(`(function() {
			var s = document.createElement('script');
			s.textContent = %s;
			document.head.appendChild(s);
		})()`, jsQuote(opts.Content))
	} else if opts.Path != "" {
		return fmt.Errorf("path-based script injection not supported; use URL or Content")
	}
	_, err := p.Evaluate(js)
	return err
}

type AddScriptTagOptions struct {
	URL     string
	Content string
	Path    string
}

func (p *Page) AddStyleTag(opts AddStyleTagOptions) error {
	var js string
	if opts.URL != "" {
		js = fmt.Sprintf(`new Promise(function(resolve, reject) {
			var l = document.createElement('link');
			l.rel = 'stylesheet';
			l.href = %s;
			l.onload = resolve;
			l.onerror = reject;
			document.head.appendChild(l);
		})`, jsQuote(opts.URL))
	} else if opts.Content != "" {
		js = fmt.Sprintf(`(function() {
			var s = document.createElement('style');
			s.textContent = %s;
			document.head.appendChild(s);
		})()`, jsQuote(opts.Content))
	}
	_, err := p.Evaluate(js)
	return err
}

type AddStyleTagOptions struct {
	URL     string
	Content string
}

func (p *Page) EmulateMedia(opts EmulateMediaOptions) error {
	ctx := p.browser.ctx
	params := map[string]any{}
	if opts.Media != "" {
		params["media"] = opts.Media
	}
	features := make([]map[string]string, 0)
	if opts.ColorScheme != "" {
		features = append(features, map[string]string{"name": "prefers-color-scheme", "value": opts.ColorScheme})
	}
	if opts.ReducedMotion != "" {
		features = append(features, map[string]string{"name": "prefers-reduced-motion", "value": opts.ReducedMotion})
	}
	if opts.ForcedColors != "" {
		features = append(features, map[string]string{"name": "forced-colors", "value": opts.ForcedColors})
	}
	if len(features) > 0 {
		params["features"] = features
	}
	_, err := p.session.Call(ctx, "Emulation.setEmulatedMedia", params)
	return err
}

type EmulateMediaOptions struct {
	Media         string
	ColorScheme   string
	ReducedMotion string
	ForcedColors  string
}

func (p *Page) BringToFront() error {
	_, err := p.session.Call(p.browser.ctx, "Page.bringToFront", nil)
	return err
}

func (p *Page) PDF(opts ...PDFOptions) ([]byte, error) {
	ctx := p.browser.ctx
	params := map[string]any{}
	if len(opts) > 0 {
		o := opts[0]
		if o.Format != "" {
			params["paperWidth"] = paperWidth(o.Format)
			params["paperHeight"] = paperHeight(o.Format)
		}
		if o.Width != "" {
			params["paperWidth"] = o.Width
		}
		if o.Height != "" {
			params["paperHeight"] = o.Height
		}
		if o.Landscape {
			params["landscape"] = true
		}
		if o.PrintBackground {
			params["printBackground"] = true
		}
		if o.Scale > 0 {
			params["scale"] = o.Scale
		}
		if o.HeaderTemplate != "" {
			params["headerTemplate"] = o.HeaderTemplate
			params["displayHeaderFooter"] = true
		}
		if o.FooterTemplate != "" {
			params["footerTemplate"] = o.FooterTemplate
			params["displayHeaderFooter"] = true
		}
	}
	result, err := p.session.Call(ctx, "Page.printToPDF", params)
	if err != nil {
		return nil, fmt.Errorf("pdf: %w", err)
	}
	var resp struct {
		Data string `json:"data"`
	}
	json.Unmarshal(result, &resp)
	return base64.StdEncoding.DecodeString(resp.Data)
}

type PDFOptions struct {
	Format         string
	Width          string
	Height         string
	Landscape      bool
	PrintBackground bool
	Scale          float64
	HeaderTemplate string
	FooterTemplate string
}

func paperWidth(format string) float64 {
	switch format {
	case "Letter":
		return 8.5
	case "Legal":
		return 8.5
	case "A4":
		return 8.27
	case "A3":
		return 11.69
	default:
		return 8.5
	}
}

func paperHeight(format string) float64 {
	switch format {
	case "Letter":
		return 11
	case "Legal":
		return 14
	case "A4":
		return 11.69
	case "A3":
		return 16.54
	default:
		return 11
	}
}

func (p *Page) SetDefaultTimeout(timeout time.Duration) {
	p.defaultTimeout = timeout
}

func (p *Page) SetDefaultNavigationTimeout(timeout time.Duration) {
	p.defaultNavigationTimeout = timeout
}

func (p *Page) IsClosed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.closed
}

func (p *Page) ViewportSize() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.viewportWidth, p.viewportHeight
}

func (p *Page) SetExtraHTTPHeaders(headers map[string]string) error {
	_, err := p.session.Call(p.browser.ctx, "Network.setExtraHTTPHeaders", map[string]any{
		"headers": headers,
	})
	return err
}

func (p *Page) ExposeFunction(name string, fn func(args ...json.RawMessage) (any, error)) error {
	ctx := p.browser.ctx
	_, err := p.session.Call(ctx, "Runtime.addBinding", map[string]any{
		"name": name,
	})
	if err != nil {
		return fmt.Errorf("expose function: %w", err)
	}

	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Runtime.bindingCalled" {
			var payload struct {
				Name    string `json:"name"`
				Payload string `json:"payload"`
				ID      int    `json:"executionContextId"`
			}
			json.Unmarshal(params, &payload)
			if payload.Name != name {
				return
			}
			var args []json.RawMessage
			json.Unmarshal([]byte(payload.Payload), &args)

			go func() {
				result, fnErr := fn(args...)
				resultJSON, _ := json.Marshal(result)
				if fnErr != nil {
					p.Evaluate(fmt.Sprintf(`window[%s].__reject(%s)`, jsQuote(name), jsQuote(fnErr.Error())))
				} else {
					p.Evaluate(fmt.Sprintf(`window[%s].__resolve(%s)`, jsQuote(name), string(resultJSON)))
				}
			}()
		}
	})

	_, err = p.Evaluate(fmt.Sprintf(`
		window[%s] = function(...args) {
			return new Promise(function(resolve, reject) {
				window[%s].__resolve = resolve;
				window[%s].__reject = reject;
				window[%s](JSON.stringify(args));
			});
		}
	`, jsQuote(name), jsQuote(name), jsQuote(name), jsQuote(name)))
	return err
}

func (p *Page) ExposeBinding(name string, fn func(source map[string]any, args ...json.RawMessage) (any, error)) error {
	return p.ExposeFunction(name, func(args ...json.RawMessage) (any, error) {
		source := map[string]any{"page": p}
		return fn(source, args...)
	})
}

func (p *Page) Opener() (*Page, error) {
	ctx := p.browser.ctx
	result, err := p.browser.conn.RootSession().Call(ctx, "Target.getTargetInfo", map[string]any{
		"targetId": p.targetID,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		TargetInfo struct {
			OpenerID         string `json:"openerId"`
			BrowserContextID string `json:"browserContextId"`
		} `json:"targetInfo"`
	}
	json.Unmarshal(result, &resp)
	if resp.TargetInfo.OpenerID == "" {
		return nil, nil
	}

	p.browser.mu.Lock()
	bc := p.browser.contexts[resp.TargetInfo.BrowserContextID]
	p.browser.mu.Unlock()
	if bc == nil {
		return nil, nil
	}

	bc.mu.Lock()
	opener := bc.pages[resp.TargetInfo.OpenerID]
	bc.mu.Unlock()
	return opener, nil
}

func (p *Page) Click(selector string, opts ...ClickOptions) error {
	return p.Locator(selector).Click()
}

func (p *Page) Fill(selector string, value string, opts ...FillOptions) error {
	return p.Locator(selector).Fill(value)
}

func (p *Page) Check(selector string) error {
	return p.Locator(selector).Check()
}

func (p *Page) Uncheck(selector string) error {
	return p.Locator(selector).Uncheck()
}

func (p *Page) Hover(selector string) error {
	return p.Locator(selector).Hover()
}

func (p *Page) SelectOption(selector string, values ...string) error {
	return p.Locator(selector).SelectOption(values...)
}

func (p *Page) Type(selector string, text string) error {
	return p.Locator(selector).Type(text)
}

func (p *Page) Press(selector string, key string) error {
	return p.Locator(selector).Press(key)
}

func (p *Page) Focus(selector string) error {
	return p.Locator(selector).Focus()
}

func (p *Page) TextContent(selector string) (string, error) {
	return p.Locator(selector).TextContent()
}

func (p *Page) InnerText(selector string) (string, error) {
	return p.Locator(selector).InnerText()
}

func (p *Page) InnerHTML(selector string) (string, error) {
	return p.Locator(selector).InnerHTML()
}

func (p *Page) InputValue(selector string) (string, error) {
	return p.Locator(selector).InputValue()
}

func (p *Page) GetAttribute(selector string, name string) (string, error) {
	return p.Locator(selector).GetAttribute(name)
}

func (p *Page) IsVisible(selector string) (bool, error) {
	return p.Locator(selector).IsVisible()
}

func (p *Page) IsEnabled(selector string) (bool, error) {
	return p.Locator(selector).IsEnabled()
}

func (p *Page) IsChecked(selector string) (bool, error) {
	return p.Locator(selector).IsChecked()
}

func (p *Page) IsHidden(selector string) (bool, error) {
	return p.Locator(selector).IsHidden()
}

func (p *Page) IsDisabled(selector string) (bool, error) {
	return p.Locator(selector).IsDisabled()
}

func (p *Page) IsEditable(selector string) (bool, error) {
	return p.Locator(selector).IsEditable()
}

func (p *Page) DispatchEvent(selector string, eventType string, eventInit ...map[string]any) error {
	return p.Locator(selector).DispatchEvent(eventType, eventInit...)
}

func (p *Page) Clock() *Clock {
	if p.clock == nil {
		p.clock = &Clock{page: p}
	}
	return p.clock
}

func (p *Page) Accessibility() *Accessibility {
	if p.accessibility == nil {
		p.accessibility = &Accessibility{page: p}
	}
	return p.accessibility
}

func (p *Page) Video() *Video {
	if p.video == nil {
		p.video = &Video{page: p}
	}
	return p.video
}

func (p *Page) Context() *BrowserContext {
	return p.context
}

type WaitForNavigationOptions struct {
	Timeout time.Duration
	URL     string
}

func (p *Page) WaitForNavigation(opts ...WaitForNavigationOptions) (*Response, error) {
	timeout := 30 * time.Second
	if len(opts) > 0 && opts[0].Timeout > 0 {
		timeout = opts[0].Timeout
	}
	ch := make(chan *Response, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.frameNavigated" {
			ch <- nil
		}
	})
	ctx, cancel := context.WithTimeout(p.browser.ctx, timeout)
	defer cancel()
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForNavigation timed out")
	}
}

func (p *Page) AddLocatorHandler(locator *Locator, handler func(*Locator)) {
	p.mu.Lock()
	if p.locatorHandlers == nil {
		p.locatorHandlers = make(map[string]func(*Locator))
	}
	p.locatorHandlers[locator.selector] = handler
	p.mu.Unlock()
}

func (p *Page) RemoveLocatorHandler(locator *Locator) {
	p.mu.Lock()
	delete(p.locatorHandlers, locator.selector)
	p.mu.Unlock()
}

type Credentials struct {
	Username string
	Password string
}

func (p *Page) Authenticate(credentials *Credentials) error {
	ctx := p.browser.ctx
	if credentials == nil {
		_, err := p.session.Call(ctx, "Fetch.disable", nil)
		return err
	}
	_, err := p.session.Call(ctx, "Fetch.enable", map[string]any{
		"handleAuthRequests": true,
	})
	if err != nil {
		return fmt.Errorf("enable fetch for auth: %w", err)
	}
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method != "Fetch.requestPaused" {
			return
		}
		var payload struct {
			RequestID          string `json:"requestId"`
			ResponseStatusCode int    `json:"responseStatusCode"`
		}
		json.Unmarshal(params, &payload)
		if payload.ResponseStatusCode == 401 {
			p.session.Call(ctx, "Fetch.provideAuthCredentials", map[string]any{
				"requestId": payload.RequestID,
				"authChallengeResponse": map[string]any{
					"response": "ProvideCredentials",
					"username": credentials.Username,
					"password": credentials.Password,
				},
			})
		} else {
			p.session.Call(ctx, "Fetch.continueRequest", map[string]any{
				"requestId": payload.RequestID,
			})
		}
	})
	return nil
}

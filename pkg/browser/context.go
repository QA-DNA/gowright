package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/QA-DNA/gowright/pkg/cdp"
)

type BrowserContext struct {
	browser   *Browser
	contextID string

	mu       sync.Mutex
	pages    map[string]*Page
	pageAdds chan string

	initScripts              []string
	extraHTTPHeaders         map[string]string
	defaultTimeout           time.Duration
	defaultNavigationTimeout time.Duration
	routeMgr                 *routeManager
	eventHandlers            map[string][]eventHandler
	har                      *HarRecorder
}

func (bc *BrowserContext) Browser() *Browser {
	return bc.browser
}

func (bc *BrowserContext) ID() string {
	return bc.contextID
}

// NewPage creates a new page (tab) in this context.
func (bc *BrowserContext) NewPage() (*Page, error) {
	ctx := bc.browser.ctx

	result, err := bc.browser.conn.RootSession().Call(ctx, "Target.createTarget", map[string]any{
		"url":              "about:blank",
		"browserContextId": bc.contextID,
	})
	if err != nil {
		return nil, fmt.Errorf("create target: %w", err)
	}

	var resp struct {
		TargetID string `json:"targetId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	// The page should appear via Target.attachedToTarget event.
	// Wait for it to be registered.
	page := bc.waitForPage(ctx, resp.TargetID)
	if page == nil {
		return nil, fmt.Errorf("page %s was not attached", resp.TargetID)
	}

	return page, nil
}

// Pages returns all open pages in this context.
func (bc *BrowserContext) Pages() []*Page {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	pages := make([]*Page, 0, len(bc.pages))
	for _, p := range bc.pages {
		pages = append(pages, p)
	}
	return pages
}

// Close closes this browser context and all its pages.
func (bc *BrowserContext) Close() error {
	_, err := bc.browser.conn.RootSession().Call(bc.browser.ctx, "Target.disposeBrowserContext", map[string]any{
		"browserContextId": bc.contextID,
	})
	if err != nil {
		return fmt.Errorf("dispose browser context: %w", err)
	}

	bc.browser.mu.Lock()
	delete(bc.browser.contexts, bc.contextID)
	bc.browser.mu.Unlock()

	return nil
}

// addPage registers a page and notifies waiters.
func (bc *BrowserContext) addPage(targetID string, page *Page) {
	bc.mu.Lock()
	bc.pages[targetID] = page
	h := bc.har
	bc.mu.Unlock()

	if h != nil && h.IsActive() {
		bc.enableHarOnPage(page, h)
	}

	// Non-blocking notify
	select {
	case bc.pageAdds <- targetID:
	default:
	}
}

func (bc *BrowserContext) enableHarOnPage(page *Page, h *HarRecorder) {
	ctx := bc.browser.ctx
	page.session.Call(ctx, "Network.enable", nil)

	requests := &sync.Map{}
	page.session.OnEvent(func(method string, params json.RawMessage) {
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
				Timestamp float64 `json:"timestamp"`
			}
			json.Unmarshal(params, &payload)
			req := &Request{
				URL:      payload.Request.URL,
				Method:   payload.Request.Method,
				Headers:  payload.Request.Headers,
				PostData: payload.Request.PostData,
				timing:   &RequestTiming{StartTime: payload.Timestamp},
			}
			requests.Store(payload.RequestID, req)

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
				} `json:"response"`
			}
			json.Unmarshal(params, &payload)
			if v, ok := requests.Load(payload.RequestID); ok {
				req := v.(*Request)
				if req.timing != nil {
					req.timing.DomainLookupStart = payload.Response.Timing.DNSStart
					req.timing.DomainLookupEnd = payload.Response.Timing.DNSEnd
					req.timing.ConnectStart = payload.Response.Timing.ConnStart
					req.timing.SecureConnectionStart = payload.Response.Timing.SSLStart
					req.timing.ConnectEnd = payload.Response.Timing.ConnEnd
					req.timing.RequestStart = payload.Response.Timing.SendStart
					req.timing.ResponseStart = payload.Response.Timing.RecvStart
				}
				resp := &Response{
					URL:        payload.Response.URL,
					Status:     payload.Response.Status,
					Headers:    payload.Response.Headers,
					statusText: payload.Response.StatusText,
				}
				h.AddEntry(req, resp)
			}

		case "Network.loadingFinished":
			var payload struct {
				RequestID         string  `json:"requestId"`
				EncodedDataLength float64 `json:"encodedDataLength"`
				Timestamp         float64 `json:"timestamp"`
			}
			json.Unmarshal(params, &payload)
			if v, ok := requests.Load(payload.RequestID); ok {
				req := v.(*Request)
				if req.timing != nil {
					req.timing.ResponseEnd = payload.Timestamp
				}
				if req.sizes == nil {
					req.sizes = &RequestSizes{}
				}
				req.sizes.ResponseBodySize = int(payload.EncodedDataLength)
				requests.Delete(payload.RequestID)
			}
		}
	})
}

// --- Cookie APIs ---

// Cookie represents a browser cookie.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// AddCookies adds cookies to this browser context.
func (bc *BrowserContext) AddCookies(cookies []Cookie) error {
	// Need a page session to set cookies via Network domain
	session := bc.anyPageSession()
	if session == nil {
		return fmt.Errorf("no pages open to set cookies on")
	}
	ctx := bc.browser.ctx

	cdpCookies := make([]map[string]any, len(cookies))
	for i, c := range cookies {
		cookie := map[string]any{
			"name":  c.Name,
			"value": c.Value,
		}
		if c.Domain != "" {
			cookie["domain"] = c.Domain
		}
		if c.Path != "" {
			cookie["path"] = c.Path
		}
		if c.Expires > 0 {
			cookie["expires"] = c.Expires
		}
		if c.HTTPOnly {
			cookie["httpOnly"] = true
		}
		if c.Secure {
			cookie["secure"] = true
		}
		if c.SameSite != "" {
			cookie["sameSite"] = c.SameSite
		}
		cdpCookies[i] = cookie
	}

	_, err := session.Call(ctx, "Network.setCookies", map[string]any{
		"cookies": cdpCookies,
	})
	return err
}

// Cookies returns cookies for this context.
func (bc *BrowserContext) Cookies(urls ...string) ([]Cookie, error) {
	session := bc.anyPageSession()
	if session == nil {
		return nil, fmt.Errorf("no pages open to get cookies from")
	}
	ctx := bc.browser.ctx

	params := map[string]any{}
	if len(urls) > 0 {
		params["urls"] = urls
	}

	result, err := session.Call(ctx, "Network.getCookies", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Cookies []Cookie `json:"cookies"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	return resp.Cookies, nil
}

// ClearCookies clears all cookies in this context.
func (bc *BrowserContext) ClearCookies() error {
	session := bc.anyPageSession()
	if session == nil {
		return fmt.Errorf("no pages open to clear cookies on")
	}
	_, err := session.Call(bc.browser.ctx, "Network.clearBrowserCookies", nil)
	return err
}

// anyPageSession returns a CDP session from any open page in this context.
func (bc *BrowserContext) anyPageSession() *cdp.Session {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	for _, p := range bc.pages {
		return p.session
	}
	return nil
}

func (bc *BrowserContext) AddInitScript(script string) {
	bc.mu.Lock()
	bc.initScripts = append(bc.initScripts, script)
	bc.mu.Unlock()

	for _, p := range bc.Pages() {
		p.session.Call(bc.browser.ctx, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
			"source": script,
		})
	}
}

func (bc *BrowserContext) ExposeFunction(name string, fn func(args ...json.RawMessage) (any, error)) error {
	for _, p := range bc.Pages() {
		if err := p.ExposeFunction(name, fn); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BrowserContext) ExposeBinding(name string, fn func(source map[string]any, args ...json.RawMessage) (any, error)) error {
	for _, p := range bc.Pages() {
		if err := p.ExposeBinding(name, fn); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BrowserContext) GrantPermissions(permissions []string, opts ...GrantPermissionsOptions) error {
	params := map[string]any{
		"permissions":      permissions,
		"browserContextId": bc.contextID,
	}
	if len(opts) > 0 && opts[0].Origin != "" {
		params["origin"] = opts[0].Origin
	}
	_, err := bc.browser.conn.RootSession().Call(bc.browser.ctx, "Browser.grantPermissions", params)
	return err
}

type GrantPermissionsOptions struct {
	Origin string
}

func (bc *BrowserContext) ClearPermissions() error {
	_, err := bc.browser.conn.RootSession().Call(bc.browser.ctx, "Browser.resetPermissions", map[string]any{
		"browserContextId": bc.contextID,
	})
	return err
}

func (bc *BrowserContext) SetGeolocation(geo *Geolocation) error {
	for _, p := range bc.Pages() {
		params := map[string]any{}
		if geo != nil {
			params["latitude"] = geo.Latitude
			params["longitude"] = geo.Longitude
			if geo.Accuracy > 0 {
				params["accuracy"] = geo.Accuracy
			}
		}
		_, err := p.session.Call(bc.browser.ctx, "Emulation.setGeolocationOverride", params)
		if err != nil {
			return err
		}
	}
	return nil
}

type Geolocation struct {
	Latitude  float64
	Longitude float64
	Accuracy  float64
}

func (bc *BrowserContext) SetOffline(offline bool) error {
	for _, p := range bc.Pages() {
		_, err := p.session.Call(bc.browser.ctx, "Network.emulateNetworkConditions", map[string]any{
			"offline":            offline,
			"latency":            0,
			"downloadThroughput": -1,
			"uploadThroughput":   -1,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (bc *BrowserContext) SetExtraHTTPHeaders(headers map[string]string) error {
	bc.mu.Lock()
	bc.extraHTTPHeaders = headers
	bc.mu.Unlock()
	for _, p := range bc.Pages() {
		if err := p.SetExtraHTTPHeaders(headers); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BrowserContext) StorageState() (*StorageState, error) {
	cookies, err := bc.Cookies()
	if err != nil {
		return nil, err
	}

	state := &StorageState{
		Cookies: cookies,
		Origins: []OriginState{},
	}

	for _, p := range bc.Pages() {
		val, err := p.Evaluate(`JSON.stringify({origin: window.location.origin, localStorage: Object.fromEntries(Object.entries(localStorage))})`)
		if err != nil {
			continue
		}
		var s string
		json.Unmarshal(val, &s)
		var origin OriginState
		json.Unmarshal([]byte(s), &origin)
		if origin.Origin != "" {
			state.Origins = append(state.Origins, origin)
		}
	}
	return state, nil
}

type StorageState struct {
	Cookies []Cookie      `json:"cookies"`
	Origins []OriginState `json:"origins"`
}

type OriginState struct {
	Origin       string            `json:"origin"`
	LocalStorage map[string]string `json:"localStorage"`
}

func (bc *BrowserContext) Route(urlPattern string, handler func(*Route, *Request)) error {
	for _, p := range bc.Pages() {
		if err := p.Route(urlPattern, handler); err != nil {
			return err
		}
	}
	return nil
}

func (bc *BrowserContext) Unroute(urlPattern string) {
	for _, p := range bc.Pages() {
		p.Unroute(urlPattern)
	}
}

func (bc *BrowserContext) UnrouteAll() {
	for _, p := range bc.Pages() {
		p.routeMgr = nil
	}
}

func (bc *BrowserContext) SetDefaultTimeout(timeout time.Duration) {
	bc.mu.Lock()
	bc.defaultTimeout = timeout
	bc.mu.Unlock()
}

func (bc *BrowserContext) SetDefaultNavigationTimeout(timeout time.Duration) {
	bc.mu.Lock()
	bc.defaultNavigationTimeout = timeout
	bc.mu.Unlock()
}

func (bc *BrowserContext) WaitForEvent(event string, timeout ...time.Duration) (any, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	ch := make(chan any, 1)
	bc.OnContext(event, func(payload any) {
		select {
		case ch <- payload:
		default:
		}
	})
	ctx, cancel := context.WithTimeout(bc.browser.ctx, t)
	defer cancel()
	select {
	case v := <-ch:
		return v, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("context.waitForEvent(%q) timed out", event)
	}
}

func (bc *BrowserContext) OnContext(event string, handler func(any)) {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.eventHandlers == nil {
		bc.eventHandlers = make(map[string][]eventHandler)
	}
	bc.eventHandlers[event] = append(bc.eventHandlers[event], eventHandler{fn: handler})
}

func (bc *BrowserContext) emitContextEvent(event string, payload any) {
	bc.mu.Lock()
	if bc.eventHandlers == nil {
		bc.mu.Unlock()
		return
	}
	handlers := make([]eventHandler, len(bc.eventHandlers[event]))
	copy(handlers, bc.eventHandlers[event])
	remaining := bc.eventHandlers[event][:0]
	for _, h := range bc.eventHandlers[event] {
		if !h.once {
			remaining = append(remaining, h)
		}
	}
	bc.eventHandlers[event] = remaining
	bc.mu.Unlock()

	for _, h := range handlers {
		h.fn(payload)
	}
}

func (bc *BrowserContext) applyInitScripts(p *Page) {
	bc.mu.Lock()
	scripts := make([]string, len(bc.initScripts))
	copy(scripts, bc.initScripts)
	headers := bc.extraHTTPHeaders
	bc.mu.Unlock()

	for _, script := range scripts {
		p.session.Call(bc.browser.ctx, "Page.addScriptToEvaluateOnNewDocument", map[string]any{
			"source": script,
		})
	}
	if headers != nil {
		p.SetExtraHTTPHeaders(headers)
	}
}

func (bc *BrowserContext) waitForPage(ctx context.Context, targetID string) *Page {
	// Quick check first
	bc.mu.Lock()
	if p, ok := bc.pages[targetID]; ok {
		bc.mu.Unlock()
		return p
	}
	bc.mu.Unlock()

	// Wait with timeout
	timeout := 10 * time.Second
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case _, ok := <-bc.pageAdds:
			if !ok {
				return nil
			}
			bc.mu.Lock()
			p, found := bc.pages[targetID]
			bc.mu.Unlock()
			if found {
				return p
			}
		}
	}
}

func (bc *BrowserContext) HarStart() {
	bc.mu.Lock()
	if bc.har == nil {
		bc.har = &HarRecorder{}
	}
	bc.har.Start()
	h := bc.har
	pages := make([]*Page, 0, len(bc.pages))
	for _, p := range bc.pages {
		pages = append(pages, p)
	}
	bc.mu.Unlock()

	for _, p := range pages {
		bc.enableHarOnPage(p, h)
	}
}

func (bc *BrowserContext) HarStop() {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	if bc.har != nil {
		bc.har.Stop()
	}
}

func (bc *BrowserContext) HarSaveAs(path string) error {
	bc.mu.Lock()
	h := bc.har
	bc.mu.Unlock()
	if h == nil {
		return fmt.Errorf("HAR recording not started")
	}
	return h.SaveAs(path)
}

func (bc *BrowserContext) HarRecorder() *HarRecorder {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return bc.har
}

func (bc *BrowserContext) RouteFromHAR(path string, opts ...RouteFromHAROptions) error {
	for _, p := range bc.Pages() {
		if err := p.RouteFromHAR(path, opts...); err != nil {
			return err
		}
	}
	return nil
}

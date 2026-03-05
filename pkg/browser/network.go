package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

// Request represents an intercepted HTTP request.
type Request struct {
	URL            string            `json:"url"`
	Method         string            `json:"method"`
	Headers        map[string]string `json:"headers"`
	PostData       string            `json:"postData"`
	resourceType   string
	redirectedFrom *Request
	redirectedTo   *Request
	frame          *Frame
	response       *Response
	failure        string
	timing         *RequestTiming
	sizes          *RequestSizes
}

// Response represents an HTTP response.
type Response struct {
	URL               string            `json:"url"`
	Status            int               `json:"status"`
	Headers           map[string]string `json:"headers"`
	session           *cdp.Session
	ctx               context.Context
	reqID             string
	request           *Request
	statusText        string
	fromServiceWorker bool
	fromCache         bool
}

// OK returns true if the response status is 200-299.
func (r *Response) OK() bool {
	return r.Status >= 200 && r.Status < 300
}

// Body returns the response body bytes.
func (r *Response) Body() ([]byte, error) {
	result, err := r.session.Call(r.ctx, "Network.getResponseBody", map[string]any{
		"requestId": r.reqID,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Body          string `json:"body"`
		Base64Encoded bool   `json:"base64Encoded"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	if resp.Base64Encoded {
		return base64.StdEncoding.DecodeString(resp.Body)
	}
	return []byte(resp.Body), nil
}

// Route represents an intercepted request that can be fulfilled, continued, or aborted.
type Route struct {
	session   *cdp.Session
	ctx       context.Context
	requestID string
	Request   *Request
}

// FulfillOptions configures Route.Fulfill.
type FulfillOptions struct {
	Status      int
	Headers     map[string]string
	Body        []byte
	ContentType string
}

// Fulfill responds to the request with a custom response.
func (r *Route) Fulfill(opts FulfillOptions) error {
	status := opts.Status
	if status == 0 {
		status = 200
	}
	headers := make([]map[string]string, 0)
	for k, v := range opts.Headers {
		headers = append(headers, map[string]string{"name": k, "value": v})
	}
	if opts.ContentType != "" {
		headers = append(headers, map[string]string{"name": "Content-Type", "value": opts.ContentType})
	}

	params := map[string]any{
		"requestId":       r.requestID,
		"responseCode":    status,
		"responseHeaders": headers,
	}
	if opts.Body != nil {
		params["body"] = base64.StdEncoding.EncodeToString(opts.Body)
	}

	_, err := r.session.Call(r.ctx, "Fetch.fulfillRequest", params)
	return err
}

// ContinueOptions configures Route.Continue.
type ContinueOptions struct {
	URL      string
	Method   string
	Headers  map[string]string
	PostData []byte
}

// Continue lets the request proceed, optionally with modifications.
func (r *Route) Continue(opts ...ContinueOptions) error {
	params := map[string]any{
		"requestId": r.requestID,
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.URL != "" {
			params["url"] = o.URL
		}
		if o.Method != "" {
			params["method"] = o.Method
		}
		if o.Headers != nil {
			headers := make([]map[string]string, 0)
			for k, v := range o.Headers {
				headers = append(headers, map[string]string{"name": k, "value": v})
			}
			params["headers"] = headers
		}
		if o.PostData != nil {
			params["postData"] = base64.StdEncoding.EncodeToString(o.PostData)
		}
	}

	_, err := r.session.Call(r.ctx, "Fetch.continueRequest", params)
	return err
}

// Abort aborts the request with an error code.
func (r *Route) Abort(errorCode ...string) error {
	code := "Failed"
	if len(errorCode) > 0 {
		code = errorCode[0]
	}
	_, err := r.session.Call(r.ctx, "Fetch.failRequest", map[string]any{
		"requestId":   r.requestID,
		"errorReason": code,
	})
	return err
}

// routeEntry holds a registered route handler.
type routeEntry struct {
	pattern string
	handler func(*Route, *Request)
}

// routeManager manages network interception for a page.
type routeManager struct {
	mu      sync.Mutex
	routes  []routeEntry
	session *cdp.Session
	ctx     context.Context
	enabled bool
	frame   *Frame
}

func newRouteManager(session *cdp.Session, ctx context.Context, frame *Frame) *routeManager {
	return &routeManager{session: session, ctx: ctx, frame: frame}
}

func (rm *routeManager) addRoute(pattern string, handler func(*Route, *Request)) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.routes = append(rm.routes, routeEntry{pattern: pattern, handler: handler})

	if !rm.enabled {
		// Enable Fetch domain for interception
		_, err := rm.session.Call(rm.ctx, "Fetch.enable", map[string]any{
			"patterns": []map[string]any{
				{"urlPattern": "*", "requestStage": "Request"},
			},
			"handleAuthRequests": false,
		})
		if err != nil {
			return fmt.Errorf("enable fetch: %w", err)
		}
		rm.enabled = true

		// Listen for paused requests
		rm.session.OnEvent(func(method string, params json.RawMessage) {
			if method == "Fetch.requestPaused" {
				rm.handlePausedRequest(params)
			}
		})
	}
	return nil
}

func (rm *routeManager) removeRoute(pattern string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	filtered := rm.routes[:0]
	for _, r := range rm.routes {
		if r.pattern != pattern {
			filtered = append(filtered, r)
		}
	}
	rm.routes = filtered
}

func (rm *routeManager) handlePausedRequest(params json.RawMessage) {
	var payload struct {
		RequestID string `json:"requestId"`
		Request   struct {
			URL      string            `json:"url"`
			Method   string            `json:"method"`
			Headers  map[string]string `json:"headers"`
			PostData string            `json:"postData"`
		} `json:"request"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return
	}

	req := &Request{
		URL:      payload.Request.URL,
		Method:   payload.Request.Method,
		Headers:  payload.Request.Headers,
		PostData: payload.Request.PostData,
		frame:    rm.frame,
	}

	route := &Route{
		session:   rm.session,
		ctx:       rm.ctx,
		requestID: payload.RequestID,
		Request:   req,
	}

	rm.mu.Lock()
	routes := make([]routeEntry, len(rm.routes))
	copy(routes, rm.routes)
	rm.mu.Unlock()

	handled := false
	for _, entry := range routes {
		if matchPattern(entry.pattern, req.URL) {
			entry.handler(route, req)
			handled = true
			break
		}
	}

	if !handled {
		route.Continue()
	}
}

// matchPattern checks if a URL matches a glob-like pattern.
func matchPattern(pattern, url string) bool {
	if pattern == "**" || pattern == "*" {
		return true
	}
	// Split on all ** segments and check that each literal part appears in order
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		remaining := url
		for i, part := range parts {
			if part == "" {
				continue
			}
			idx := strings.Index(remaining, part)
			if idx < 0 {
				return false
			}
			if i == 0 && idx != 0 {
				return false // first part must be a prefix
			}
			remaining = remaining[idx+len(part):]
		}
		// If pattern doesn't end with **, last part must be at end of URL
		if parts[len(parts)-1] != "" {
			return strings.HasSuffix(url, parts[len(parts)-1])
		}
		return true
	}
	return strings.Contains(url, pattern)
}

// --- Page integration ---

// Route intercepts requests matching a URL pattern.
func (p *Page) Route(urlPattern string, handler func(*Route, *Request)) error {
	if p.routeMgr == nil {
		p.routeMgr = newRouteManager(p.session, p.browser.ctx, p.mainFrame)
	}
	return p.routeMgr.addRoute(urlPattern, handler)
}

// Unroute removes a route handler.
func (p *Page) Unroute(urlPattern string) {
	if p.routeMgr != nil {
		p.routeMgr.removeRoute(urlPattern)
	}
}

// WaitForRequest waits for a request matching a URL pattern.
func (p *Page) WaitForRequest(urlPattern string, timeout ...time.Duration) (*Request, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	ch := make(chan *Request, 1)
	p.On("request", func(v any) {
		req, ok := v.(*Request)
		if !ok {
			return
		}
		if matchPattern(urlPattern, req.URL) {
			select {
			case ch <- req:
			default:
			}
		}
	})

	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case req := <-ch:
		return req, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForRequest(%q) timed out", urlPattern)
	}
}

// WaitForResponse waits for a response matching a URL pattern.
func (p *Page) WaitForResponse(urlPattern string, timeout ...time.Duration) (*Response, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	ch := make(chan *Response, 1)
	p.On("response", func(v any) {
		resp, ok := v.(*Response)
		if !ok {
			return
		}
		if matchPattern(urlPattern, resp.URL) {
			select {
			case ch <- resp:
			default:
			}
		}
	})

	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case resp := <-ch:
		return resp, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForResponse(%q) timed out", urlPattern)
	}
}

func (r *Request) AllHeaders() map[string]string {
	return r.Headers
}

func (r *Request) HeaderValue(name string) string {
	return r.Headers[name]
}

func (r *Request) IsNavigationRequest() bool {
	return r.resourceType == "Document"
}

func (r *Request) ResourceType() string {
	return r.resourceType
}

func (r *Request) RedirectedFrom() *Request {
	return r.redirectedFrom
}

func (r *Request) RedirectedTo() *Request {
	return r.redirectedTo
}

func (r *Request) Frame() *Frame {
	return r.frame
}

func (r *Request) Response() *Response {
	return r.response
}

func (r *Request) Failure() string {
	return r.failure
}

func (r *Request) PostDataBuffer() []byte {
	return []byte(r.PostData)
}

type RequestTiming struct {
	StartTime             float64 `json:"startTime"`
	DomainLookupStart     float64 `json:"domainLookupStart"`
	DomainLookupEnd       float64 `json:"domainLookupEnd"`
	ConnectStart          float64 `json:"connectStart"`
	SecureConnectionStart float64 `json:"secureConnectionStart"`
	ConnectEnd            float64 `json:"connectEnd"`
	RequestStart          float64 `json:"requestStart"`
	ResponseStart         float64 `json:"responseStart"`
	ResponseEnd           float64 `json:"responseEnd"`
}

func (r *Request) Timing() *RequestTiming {
	return r.timing
}

type RequestSizes struct {
	RequestBodySize  int `json:"requestBodySize"`
	RequestHeadersSize int `json:"requestHeadersSize"`
	ResponseBodySize int `json:"responseBodySize"`
	ResponseHeadersSize int `json:"responseHeadersSize"`
}

func (r *Request) Sizes() *RequestSizes {
	return r.sizes
}

func (r *Response) AllHeaders() map[string]string {
	return r.Headers
}

func (r *Response) HeaderValue(name string) string {
	return r.Headers[name]
}

func (r *Response) Request() *Request {
	return r.request
}

func (r *Response) StatusText() string {
	return r.statusText
}

func (r *Response) FromServiceWorker() bool {
	return r.fromServiceWorker
}

func (r *Response) FromCache() bool {
	return r.fromCache
}

func (r *Response) HeaderValues(name string) []string {
	val, ok := r.Headers[name]
	if !ok {
		return nil
	}
	parts := strings.Split(val, ",")
	for i, p := range parts {
		parts[i] = strings.TrimSpace(p)
	}
	return parts
}

func (r *Response) Json() (json.RawMessage, error) {
	body, err := r.Body()
	if err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func (r *Response) Text() (string, error) {
	body, err := r.Body()
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (r *Response) SecurityDetails() (*SecurityDetails, error) {
	result, err := r.session.Call(r.ctx, "Network.getResponseBody", map[string]any{
		"requestId": r.reqID,
	})
	if err != nil {
		return &SecurityDetails{}, nil
	}
	_ = result
	return &SecurityDetails{}, nil
}

type SecurityDetails struct {
	Protocol                string `json:"protocol"`
	SubjectName             string `json:"subjectName"`
	Issuer                  string `json:"issuer"`
	ValidFrom               float64 `json:"validFrom"`
	ValidTo                 float64 `json:"validTo"`
}

func (r *Response) ServerAddr() (*ServerAddr, error) {
	return &ServerAddr{}, nil
}

type ServerAddr struct {
	IPAddress string `json:"ipAddress"`
	Port      int    `json:"port"`
}

func (r *Route) Fetch(opts ...RouteFetchOptions) (*Response, error) {
	params := map[string]any{
		"requestId": r.requestID,
	}
	if len(opts) > 0 {
		o := opts[0]
		if o.URL != "" {
			params["url"] = o.URL
		}
		if o.Method != "" {
			params["method"] = o.Method
		}
		if o.Headers != nil {
			headers := make([]map[string]string, 0)
			for k, v := range o.Headers {
				headers = append(headers, map[string]string{"name": k, "value": v})
			}
			params["headers"] = headers
		}
	}

	result, err := r.session.Call(r.ctx, "Fetch.getResponseBody", params)
	if err != nil {
		return nil, fmt.Errorf("route.Fetch: %w", err)
	}
	var resp struct {
		Body          string `json:"body"`
		Base64Encoded bool   `json:"base64Encoded"`
	}
	json.Unmarshal(result, &resp)

	body := []byte(resp.Body)
	if resp.Base64Encoded {
		body, _ = base64.StdEncoding.DecodeString(resp.Body)
	}

	_ = body
	return &Response{
		URL:     r.Request.URL,
		Status:  200,
		Headers: r.Request.Headers,
		session: r.session,
		ctx:     r.ctx,
		reqID:   r.requestID,
		request: r.Request,
	}, nil
}

type RouteFetchOptions struct {
	URL     string
	Method  string
	Headers map[string]string
}

func (r *Route) Fallback(opts ...ContinueOptions) error {
	return r.Continue(opts...)
}

func (p *Page) UnrouteAll() {
	p.routeMgr = nil
}

func (p *Page) RouteFromHAR(path string, opts ...RouteFromHAROptions) error {
	entries, err := LoadHar(path)
	if err != nil {
		return fmt.Errorf("load HAR: %w", err)
	}

	for i := range entries {
		entry := &entries[i]
		pattern := "**" + entry.Request.URL + "**"
		if len(opts) > 0 && opts[0].URL != "" {
			pattern = opts[0].URL
		}
		err := p.Route(pattern, func(route *Route, req *Request) {
			if req.URL != entry.Request.URL {
				route.Continue()
				return
			}
			headers := make(map[string]string, len(entry.Response.Headers))
			for _, h := range entry.Response.Headers {
				headers[h.Name] = h.Value
			}
			body := []byte{}
			if entry.Response.Content != nil {
				body = []byte(entry.Response.Content.Text)
			}
			route.Fulfill(FulfillOptions{
				Status:  entry.Response.Status,
				Headers: headers,
				Body:    body,
			})
		})
		if err != nil {
			return err
		}
	}
	return nil
}

type RouteFromHAROptions struct {
	URL      string
	NotFound string
}

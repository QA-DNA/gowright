// Package browser provides the high-level Browser and BrowserContext types.
package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/QA-DNA/gowright/pkg/cdp"
)

// Browser represents a running browser instance.
type Browser struct {
	conn *cdp.Conn
	ctx  context.Context

	mu       sync.Mutex
	contexts map[string]*BrowserContext // browserContextId -> context

	cleanup          func() // kills process, removes temp dirs
	slowMo           time.Duration
	defaultViewportW int
	defaultViewportH int
}

// SetSlowMo sets a delay before each action (for debugging).
func (b *Browser) SetSlowMo(d time.Duration) {
	b.slowMo = d
}

// SetDefaultViewport sets the default viewport applied to new pages.
func (b *Browser) SetDefaultViewport(width, height int) {
	b.defaultViewportW = width
	b.defaultViewportH = height
}

// ConnectOptions configure how to connect to a browser.
type ConnectOptions struct {
	// WSEndpoint is the WebSocket URL (ws://...).
	WSEndpoint string
}

// Connect connects to an already-running browser at the given WebSocket endpoint.
func Connect(ctx context.Context, wsEndpoint string) (*Browser, error) {
	ws, err := cdp.DialWebSocket(ctx, wsEndpoint)
	if err != nil {
		return nil, err
	}

	conn := cdp.NewConn(ws)

	b := &Browser{
		conn:     conn,
		ctx:      ctx,
		contexts: make(map[string]*BrowserContext),
	}

	// Set up auto-attach so we get notified of new pages
	if err := b.setupAutoAttach(); err != nil {
		conn.Close()
		return nil, err
	}

	return b, nil
}

// SetCleanup sets the cleanup function called on Close().
func (b *Browser) SetCleanup(fn func()) {
	b.cleanup = fn
}

// Conn returns the underlying CDP connection.
func (b *Browser) Conn() *cdp.Conn {
	return b.conn
}

// NewContext creates a new browser context (like an incognito window).
func (b *Browser) NewContext() (*BrowserContext, error) {
	result, err := b.conn.RootSession().Call(b.ctx, "Target.createBrowserContext", map[string]any{
		"disposeOnDetach": true,
	})
	if err != nil {
		return nil, fmt.Errorf("create browser context: %w", err)
	}

	var resp struct {
		BrowserContextID string `json:"browserContextId"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}

	bc := &BrowserContext{
		browser:   b,
		contextID: resp.BrowserContextID,
		pages:     make(map[string]*Page),
		pageAdds:  make(chan string, 16),
	}

	b.mu.Lock()
	b.contexts[resp.BrowserContextID] = bc
	b.mu.Unlock()

	return bc, nil
}

func (b *Browser) NewPage() (*Page, error) {
	bc, err := b.NewContext()
	if err != nil {
		return nil, err
	}
	return bc.NewPage()
}

// Close the browser and clean up resources.
func (b *Browser) Close() error {
	// Try graceful close
	_, _ = b.conn.RootSession().Call(b.ctx, "Browser.close", nil)

	err := b.conn.Close()

	if b.cleanup != nil {
		b.cleanup()
	}

	return err
}

// Version returns the browser version string.
func (b *Browser) Version() (string, error) {
	result, err := b.conn.RootSession().Call(b.ctx, "Browser.getVersion", nil)
	if err != nil {
		return "", err
	}

	var resp struct {
		Product   string `json:"product"`
		UserAgent string `json:"userAgent"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}

	return resp.Product, nil
}

// setupAutoAttach tells the browser to auto-attach to new targets (pages, workers).
func (b *Browser) setupAutoAttach() error {
	_, err := b.conn.RootSession().Call(b.ctx, "Target.setAutoAttach", map[string]any{
		"autoAttach":             true,
		"waitForDebuggerOnStart": true,
		"flatten":                true,
	})
	if err != nil {
		return fmt.Errorf("set auto attach: %w", err)
	}

	// Listen for target attachment events
	b.conn.RootSession().OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Target.attachedToTarget":
			b.onAttachedToTarget(params)
		case "Target.detachedFromTarget":
			b.onDetachedFromTarget(params)
		}
	})

	return nil
}

func (b *Browser) onAttachedToTarget(params json.RawMessage) {
	var payload struct {
		SessionID  string `json:"sessionId"`
		TargetInfo struct {
			TargetID         string `json:"targetId"`
			Type             string `json:"type"`
			URL              string `json:"url"`
			BrowserContextID string `json:"browserContextId"`
		} `json:"targetInfo"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return
	}

	// Create a CDP session for this target
	session := b.conn.CreateSession(payload.SessionID)

	if payload.TargetInfo.Type == "page" {
		b.mu.Lock()
		pageContext := b.contexts[payload.TargetInfo.BrowserContextID]
		b.mu.Unlock()

		page := &Page{
			session:       session,
			targetID:      payload.TargetInfo.TargetID,
			url:           payload.TargetInfo.URL,
			browser:       b,
			context:       pageContext,
			eventHandlers: make(map[string][]eventHandler),
			frames:        make(map[string]*Frame),
		}
		page.setupPageEvents()
		page.setupFrameEvents()
		page.initFrameTree()

		if pageContext != nil {
			pageContext.addPage(payload.TargetInfo.TargetID, page)
			pageContext.applyInitScripts(page)
			pageContext.emitContextEvent("page", page)
		}

		// Apply default viewport
		if b.defaultViewportW > 0 && b.defaultViewportH > 0 {
			go page.SetViewportSize(b.defaultViewportW, b.defaultViewportH)
		}
	}

	// Resume the target (it was paused by waitForDebuggerOnStart).
	// Do this in a goroutine to avoid blocking the event handler
	// (Call() waits for response which needs the message loop).
	go func() {
		session.Call(b.ctx, "Runtime.runIfWaitingForDebugger", nil)
	}()
}

func (b *Browser) Contexts() []*BrowserContext {
	b.mu.Lock()
	defer b.mu.Unlock()
	contexts := make([]*BrowserContext, 0, len(b.contexts))
	for _, ctx := range b.contexts {
		contexts = append(contexts, ctx)
	}
	return contexts
}

func (b *Browser) IsConnected() bool {
	return b.conn != nil
}

func (b *Browser) onDetachedFromTarget(params json.RawMessage) {
	var payload struct {
		SessionID string `json:"sessionId"`
		TargetID  string `json:"targetId"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return
	}

	b.conn.RemoveSession(payload.SessionID)
}

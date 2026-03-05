// Package cdp implements a Chrome DevTools Protocol client.
//
// It provides WebSocket-based communication with Chromium browsers,
// including request-response matching, session multiplexing, and event streaming.
package cdp

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Request is a CDP command sent to the browser.
type Request struct {
	ID        int         `json:"id"`
	SessionID string      `json:"sessionId,omitempty"`
	Method    string      `json:"method"`
	Params    interface{} `json:"params,omitempty"`
}

// Response is a CDP response from the browser (has an ID matching a request).
type Response struct {
	ID     int             `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *Error          `json:"error,omitempty"`
}

// Event is a CDP event from the browser (no ID, has method + params).
type Event struct {
	SessionID string          `json:"sessionId,omitempty"`
	Method    string          `json:"method"`
	Params    json.RawMessage `json:"params,omitempty"`
}

// Error represents a CDP protocol error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("cdp error %d: %s", e.Code, e.Message)
}

// Transport is the interface for sending/receiving raw bytes.
// Implemented by WebSocket connections.
type Transport interface {
	Send(data []byte) error
	Read() ([]byte, error)
	Close() error
}

// Conn is a CDP connection that multiplexes sessions over a single transport.
type Conn struct {
	nextID uint64

	transport Transport
	sessions  sync.Map // map[string]*Session
	closed    atomic.Bool
	done      chan struct{}

	// OnEvent is called for events that don't match any session.
	// This shouldn't normally happen but provides a safety valve.
	OnEvent func(*Event)
}

// NewConn creates a CDP connection over the given transport.
// It starts a background goroutine to consume messages.
func NewConn(transport Transport) *Conn {
	c := &Conn{
		transport: transport,
		done:      make(chan struct{}),
	}

	// Root session (empty sessionId) is always present
	root := newSession(c, "")
	c.sessions.Store("", root)

	go c.consumeMessages()
	return c
}

// RootSession returns the root CDP session (browser-level commands).
func (c *Conn) RootSession() *Session {
	s, _ := c.sessions.Load("")
	return s.(*Session)
}

// Session returns a session by ID, or nil if not found.
func (c *Conn) Session(id string) *Session {
	s, ok := c.sessions.Load(id)
	if !ok {
		return nil
	}
	return s.(*Session)
}

// CreateSession registers a new child session for the given sessionId.
func (c *Conn) CreateSession(sessionID string) *Session {
	s := newSession(c, sessionID)
	c.sessions.Store(sessionID, s)
	return s
}

// RemoveSession unregisters a session and cancels any pending callbacks.
func (c *Conn) RemoveSession(sessionID string) {
	if v, ok := c.sessions.LoadAndDelete(sessionID); ok {
		v.(*Session).dispose()
	}
}

// Close the connection and all sessions.
func (c *Conn) Close() error {
	if c.closed.Swap(true) {
		return nil // already closed
	}
	err := c.transport.Close()
	<-c.done // wait for consumeMessages to exit
	return err
}

// Done returns a channel that's closed when the connection is done.
func (c *Conn) Done() <-chan struct{} {
	return c.done
}

// rawSend sends a raw CDP request over the transport.
func (c *Conn) rawSend(sessionID, method string, params interface{}) (int, error) {
	id := int(atomic.AddUint64(&c.nextID, 1))

	req := Request{
		ID:        id,
		SessionID: sessionID,
		Method:    method,
		Params:    params,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return 0, fmt.Errorf("marshal request: %w", err)
	}

	if err := c.transport.Send(data); err != nil {
		return 0, fmt.Errorf("send: %w", err)
	}

	return id, nil
}

// consumeMessages reads messages from the transport and routes them.
// Runs in a single goroutine - no concurrent reads.
func (c *Conn) consumeMessages() {
	defer close(c.done)

	for {
		data, err := c.transport.Read()
		if err != nil {
			if c.closed.Load() {
				return // expected close
			}
			// Transport error - notify all pending callbacks
			c.sessions.Range(func(_, val interface{}) bool {
				val.(*Session).abortAll(err)
				return true
			})
			return
		}

		// First pass: determine if it's a response (has ID) or event (no ID)
		var peek struct {
			ID        int    `json:"id"`
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(data, &peek); err != nil {
			continue // malformed message, skip
		}

		if peek.ID == 0 {
			// Event (no request ID)
			var evt Event
			if err := json.Unmarshal(data, &evt); err != nil {
				continue
			}
			c.routeEvent(&evt)
		} else {
			// Response to a request
			var res Response
			if err := json.Unmarshal(data, &res); err != nil {
				continue
			}
			c.routeResponse(peek.SessionID, &res)
		}
	}
}

// routeResponse delivers a response to the matching session's pending callback.
func (c *Conn) routeResponse(sessionID string, res *Response) {
	s, ok := c.sessions.Load(sessionID)
	if !ok {
		return
	}
	s.(*Session).handleResponse(res)
}

// routeEvent delivers an event to the matching session.
func (c *Conn) routeEvent(evt *Event) {
	s, ok := c.sessions.Load(evt.SessionID)
	if !ok {
		if c.OnEvent != nil {
			c.OnEvent(evt)
		}
		return
	}
	s.(*Session).handleEvent(evt)
}

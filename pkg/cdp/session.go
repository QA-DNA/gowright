package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// EventHandler is a callback for CDP events.
type EventHandler func(method string, params json.RawMessage)

// Session represents a CDP session multiplexed over a connection.
// The root session (empty sessionId) handles browser-level commands.
// Child sessions handle page-level commands.
type Session struct {
	conn      *Conn
	sessionID string

	mu       sync.Mutex
	pending  map[int]chan callResult
	closed   bool
	handlers []EventHandler

	// Events is a channel of CDP events for this session.
	// Must be consumed or it will block the message loop.
	Events chan *Event
}

type callResult struct {
	result json.RawMessage
	err    error
}

func newSession(conn *Conn, sessionID string) *Session {
	return &Session{
		conn:      conn,
		sessionID: sessionID,
		pending:   make(map[int]chan callResult),
		Events:    make(chan *Event, 64), // buffered to avoid blocking
	}
}

// ID returns the session's CDP session ID.
func (s *Session) ID() string {
	return s.sessionID
}

// Call sends a CDP command and waits for the response.
// The params argument is JSON-serialized and sent with the command.
// Returns the result as raw JSON.
func (s *Session) Call(ctx context.Context, method string, params interface{}) (json.RawMessage, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, fmt.Errorf("session %q is closed", s.sessionID)
	}

	id, err := s.conn.rawSend(s.sessionID, method, params)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}

	ch := make(chan callResult, 1)
	s.pending[id] = ch
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-ch:
		return res.result, res.err
	}
}

// OnEvent registers a handler that's called for every event on this session.
// Multiple handlers can be registered and are called in order.
func (s *Session) OnEvent(handler EventHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, handler)
}

// handleResponse matches a response to a pending request.
func (s *Session) handleResponse(res *Response) {
	s.mu.Lock()
	ch, ok := s.pending[res.ID]
	if ok {
		delete(s.pending, res.ID)
	}
	s.mu.Unlock()

	if !ok {
		return
	}

	if res.Error != nil {
		ch <- callResult{err: res.Error}
	} else {
		ch <- callResult{result: res.Result}
	}
}

// handleEvent dispatches an event to handlers and the Events channel.
// Runs asynchronously to avoid blocking the message consumption loop.
func (s *Session) handleEvent(evt *Event) {
	s.mu.Lock()
	handlers := make([]EventHandler, len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.Unlock()

	// Dispatch asynchronously to prevent blocking consumeMessages.
	// This is critical: handlers may call Session.Call() which waits
	// for a response that needs to be read by consumeMessages.
	go func() {
		for _, h := range handlers {
			h(evt.Method, evt.Params)
		}
	}()

	// Non-blocking send to Events channel
	select {
	case s.Events <- evt:
	default:
	}
}

// abortAll cancels all pending requests with the given error.
func (s *Session) abortAll(err error) {
	s.mu.Lock()
	pending := s.pending
	s.pending = make(map[int]chan callResult)
	s.mu.Unlock()

	for _, ch := range pending {
		ch <- callResult{err: err}
	}
}

// dispose marks the session as closed and aborts pending requests.
func (s *Session) dispose() {
	s.mu.Lock()
	s.closed = true
	pending := s.pending
	s.pending = make(map[int]chan callResult)
	s.mu.Unlock()

	for _, ch := range pending {
		ch <- callResult{err: fmt.Errorf("session %q disposed", s.sessionID)}
	}

	close(s.Events)
}

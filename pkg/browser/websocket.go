package browser

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type WebSocket struct {
	page   *Page
	url    string
	mu     sync.Mutex
	closed bool
	handlers map[string][]func(any)
}

func (ws *WebSocket) URL() string { return ws.url }

func (ws *WebSocket) IsClosed() bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	return ws.closed
}

func (ws *WebSocket) On(event string, handler func(any)) {
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.handlers == nil {
		ws.handlers = make(map[string][]func(any))
	}
	ws.handlers[event] = append(ws.handlers[event], handler)
}

func (ws *WebSocket) emit(event string, payload any) {
	ws.mu.Lock()
	handlers := make([]func(any), len(ws.handlers[event]))
	copy(handlers, ws.handlers[event])
	ws.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
}

func (ws *WebSocket) WaitForEvent(event string, timeout ...time.Duration) (any, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}
	ch := make(chan any, 1)
	ws.On(event, func(payload any) {
		select {
		case ch <- payload:
		default:
		}
	})
	timer := time.NewTimer(t)
	defer timer.Stop()
	select {
	case v := <-ch:
		return v, nil
	case <-timer.C:
		return nil, fmt.Errorf("websocket.waitForEvent(%q) timed out", event)
	}
}

type WebSocketFrame struct {
	Opcode      int    `json:"opcode"`
	PayloadData string `json:"payloadData"`
}

func (p *Page) setupWebSocketTracking() {
	webSockets := make(map[string]*WebSocket)
	var mu sync.Mutex

	p.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Network.webSocketCreated":
			var payload struct {
				RequestID string `json:"requestId"`
				URL       string `json:"url"`
			}
			json.Unmarshal(params, &payload)
			ws := &WebSocket{page: p, url: payload.URL, handlers: make(map[string][]func(any))}
			mu.Lock()
			webSockets[payload.RequestID] = ws
			mu.Unlock()
			p.emitEvent("websocket", ws)
		case "Network.webSocketClosed":
			var payload struct {
				RequestID string `json:"requestId"`
			}
			json.Unmarshal(params, &payload)
			mu.Lock()
			ws := webSockets[payload.RequestID]
			mu.Unlock()
			if ws != nil {
				ws.mu.Lock()
				ws.closed = true
				ws.mu.Unlock()
				ws.emit("close", nil)
			}
		case "Network.webSocketFrameReceived":
			var payload struct {
				RequestID string       `json:"requestId"`
				Response  WebSocketFrame `json:"response"`
			}
			json.Unmarshal(params, &payload)
			mu.Lock()
			ws := webSockets[payload.RequestID]
			mu.Unlock()
			if ws != nil {
				ws.emit("framereceived", payload.Response)
			}
		case "Network.webSocketFrameSent":
			var payload struct {
				RequestID string       `json:"requestId"`
				Response  WebSocketFrame `json:"response"`
			}
			json.Unmarshal(params, &payload)
			mu.Lock()
			ws := webSockets[payload.RequestID]
			mu.Unlock()
			if ws != nil {
				ws.emit("framesent", payload.Response)
			}
		case "Network.webSocketFrameError":
			var payload struct {
				RequestID    string `json:"requestId"`
				ErrorMessage string `json:"errorMessage"`
			}
			json.Unmarshal(params, &payload)
			mu.Lock()
			ws := webSockets[payload.RequestID]
			mu.Unlock()
			if ws != nil {
				ws.emit("socketerror", payload.ErrorMessage)
			}
		}
	})
}

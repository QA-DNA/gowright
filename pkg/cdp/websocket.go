package cdp

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// WebSocket implements Transport using github.com/coder/websocket.
type WebSocket struct {
	conn *websocket.Conn
	mu   sync.Mutex
	ctx  context.Context
}

// DialWebSocket connects to a CDP WebSocket endpoint.
func DialWebSocket(ctx context.Context, url string) (*WebSocket, error) {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"User-Agent": {"Gowright/0.1"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", url, err)
	}

	// CDP can send large messages (screenshots, DOM snapshots)
	conn.SetReadLimit(256 * 1024 * 1024) // 256 MB

	return &WebSocket{conn: conn, ctx: ctx}, nil
}

// Send writes a message to the WebSocket. Thread-safe.
func (ws *WebSocket) Send(data []byte) error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ctx, cancel := context.WithTimeout(ws.ctx, 30*time.Second)
	defer cancel()

	return ws.conn.Write(ctx, websocket.MessageText, data)
}

// Read reads the next message from the WebSocket. Blocks until a message arrives.
func (ws *WebSocket) Read() ([]byte, error) {
	_, data, err := ws.conn.Read(ws.ctx)
	return data, err
}

// Close the WebSocket connection.
func (ws *WebSocket) Close() error {
	return ws.conn.Close(websocket.StatusNormalClosure, "")
}

package browser

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

type Worker struct {
	page    *Page
	session *cdp.Session
	url     string
	mu      sync.Mutex
	closed  bool
	handlers map[string][]func(any)
}

func (w *Worker) URL() string { return w.url }

func (w *Worker) Evaluate(expression string) (json.RawMessage, error) {
	ctx := w.page.browser.ctx
	result, err := w.session.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
	}
	json.Unmarshal(result, &resp)
	return resp.Result.Value, nil
}

func (w *Worker) EvaluateHandle(expression string) (*JSHandle, error) {
	ctx := w.page.browser.ctx
	result, err := w.session.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": false,
		"awaitPromise":  true,
	})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			ObjectID string `json:"objectId"`
		} `json:"result"`
	}
	json.Unmarshal(result, &resp)
	return &JSHandle{session: w.session, page: w.page, objectID: resp.Result.ObjectID}, nil
}

func (w *Worker) On(event string, handler func(any)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.handlers == nil {
		w.handlers = make(map[string][]func(any))
	}
	w.handlers[event] = append(w.handlers[event], handler)
}

func (w *Worker) emit(event string, payload any) {
	w.mu.Lock()
	handlers := make([]func(any), len(w.handlers[event]))
	copy(handlers, w.handlers[event])
	w.mu.Unlock()
	for _, h := range handlers {
		h(payload)
	}
}

func (p *Page) Workers() []*Worker {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*Worker(nil), p.workers...)
}

func (p *Page) setupWorkerTracking() {
	p.browser.conn.RootSession().OnEvent(func(method string, params json.RawMessage) {
		if method != "Target.attachedToTarget" {
			return
		}
		var payload struct {
			SessionID  string `json:"sessionId"`
			TargetInfo struct {
				TargetID string `json:"targetId"`
				Type     string `json:"type"`
				URL      string `json:"url"`
			} `json:"targetInfo"`
		}
		json.Unmarshal(params, &payload)
		if payload.TargetInfo.Type != "worker" && payload.TargetInfo.Type != "service_worker" {
			return
		}

		session := p.browser.conn.CreateSession(payload.SessionID)
		w := &Worker{
			page:     p,
			session:  session,
			url:      payload.TargetInfo.URL,
			handlers: make(map[string][]func(any)),
		}

		p.mu.Lock()
		p.workers = append(p.workers, w)
		p.mu.Unlock()

		p.emitEvent("worker", w)

		go session.Call(context.Background(), "Runtime.runIfWaitingForDebugger", nil)
	})
}

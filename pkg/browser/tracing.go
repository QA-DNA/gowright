package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

type Tracing struct {
	page    *Page
	started bool
}

func (p *Page) Tracing() *Tracing {
	if p.tracing == nil {
		p.tracing = &Tracing{page: p}
	}
	return p.tracing
}

func (t *Tracing) Start(opts ...TracingStartOptions) error {
	ctx := t.page.browser.ctx
	params := map[string]any{}
	if len(opts) > 0 {
		if opts[0].Screenshots {
			params["screenshots"] = true
		}
		if len(opts[0].Categories) > 0 {
			params["categories"] = opts[0].Categories
		}
	}
	_, err := t.page.session.Call(ctx, "Tracing.start", params)
	if err != nil {
		return fmt.Errorf("tracing start: %w", err)
	}
	t.started = true
	return nil
}

func (t *Tracing) Stop() ([]byte, error) {
	ctx := t.page.browser.ctx
	ch := make(chan []byte, 1)
	var chunks []string

	t.page.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Tracing.dataCollected":
			var payload struct {
				Value []json.RawMessage `json:"value"`
			}
			json.Unmarshal(params, &payload)
			for _, v := range payload.Value {
				chunks = append(chunks, string(v))
			}
		case "Tracing.tracingComplete":
			var payload struct {
				Stream string `json:"stream"`
			}
			json.Unmarshal(params, &payload)
			if payload.Stream != "" {
				data := t.readStream(payload.Stream)
				ch <- data
			} else {
				result := "[" + joinStrings(chunks, ",") + "]"
				ch <- []byte(result)
			}
		}
	})

	_, err := t.page.session.Call(ctx, "Tracing.end", nil)
	if err != nil {
		return nil, fmt.Errorf("tracing end: %w", err)
	}
	t.started = false

	select {
	case data := <-ch:
		return data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (t *Tracing) StartChunk() error {
	return t.Start()
}

func (t *Tracing) StopChunk() ([]byte, error) {
	return t.Stop()
}

func (t *Tracing) readStream(handle string) []byte {
	ctx := t.page.browser.ctx
	var data []byte
	for {
		result, err := t.page.session.Call(ctx, "IO.read", map[string]any{
			"handle": handle,
		})
		if err != nil {
			break
		}
		var resp struct {
			Data          string `json:"data"`
			EOF           bool   `json:"eof"`
			Base64Encoded bool   `json:"base64Encoded"`
		}
		json.Unmarshal(result, &resp)
		if resp.Base64Encoded {
			decoded, _ := base64.StdEncoding.DecodeString(resp.Data)
			data = append(data, decoded...)
		} else {
			data = append(data, []byte(resp.Data)...)
		}
		if resp.EOF {
			break
		}
	}
	t.page.session.Call(ctx, "IO.close", map[string]any{"handle": handle})
	return data
}

type TracingStartOptions struct {
	Screenshots bool
	Categories  []string
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}

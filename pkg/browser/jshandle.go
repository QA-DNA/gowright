package browser

import (
	"encoding/json"
	"fmt"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

type JSHandle struct {
	session  *cdp.Session
	page     *Page
	objectID string
	value    json.RawMessage
}

func (h *JSHandle) Evaluate(expression string, arg ...any) (json.RawMessage, error) {
	ctx := h.page.browser.ctx
	var argsJSON string
	if len(arg) > 0 {
		b, _ := json.Marshal(arg[0])
		argsJSON = string(b)
	}
	return callFunctionOn(ctx, h.session, h.objectID, expression, argsJSON)
}

func (h *JSHandle) GetProperties() (map[string]*JSHandle, error) {
	ctx := h.page.browser.ctx
	result, err := h.session.Call(ctx, "Runtime.getProperties", map[string]any{
		"objectId":               h.objectID,
		"ownProperties":          true,
		"generatePreview":        false,
	})
	if err != nil {
		return nil, fmt.Errorf("get properties: %w", err)
	}
	var resp struct {
		Result []struct {
			Name  string `json:"name"`
			Value struct {
				ObjectID string          `json:"objectId"`
				Type     string          `json:"type"`
				Value    json.RawMessage `json:"value"`
			} `json:"value"`
		} `json:"result"`
	}
	json.Unmarshal(result, &resp)

	props := make(map[string]*JSHandle)
	for _, prop := range resp.Result {
		handle := &JSHandle{
			session:  h.session,
			page:     h.page,
			objectID: prop.Value.ObjectID,
		}
		if prop.Value.ObjectID == "" && prop.Value.Value != nil {
			handle.value = prop.Value.Value
		}
		props[prop.Name] = handle
	}
	return props, nil
}

func (h *JSHandle) JsonValue() (json.RawMessage, error) {
	if h.objectID == "" && h.value != nil {
		return h.value, nil
	}
	ctx := h.page.browser.ctx
	result, err := h.session.Call(ctx, "Runtime.callFunctionOn", map[string]any{
		"functionDeclaration": "function() { return this; }",
		"objectId":            h.objectID,
		"returnByValue":       true,
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

func (h *JSHandle) Dispose() error {
	ctx := h.page.browser.ctx
	releaseObject(ctx, h.session, h.objectID)
	return nil
}

func (h *JSHandle) GetProperty(name string) (*JSHandle, error) {
	props, err := h.GetProperties()
	if err != nil {
		return nil, err
	}
	prop, ok := props[name]
	if !ok {
		return &JSHandle{session: h.session, page: h.page}, nil
	}
	return prop, nil
}

func (h *JSHandle) EvaluateHandle(expression string, arg ...any) (*JSHandle, error) {
	ctx := h.page.browser.ctx
	params := map[string]any{
		"functionDeclaration": expression,
		"objectId":            h.objectID,
		"returnByValue":       false,
		"awaitPromise":        true,
	}
	if len(arg) > 0 {
		b, _ := json.Marshal(arg[0])
		params["arguments"] = []map[string]any{{"value": json.RawMessage(b)}}
	}
	result, err := h.session.Call(ctx, "Runtime.callFunctionOn", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result struct {
			ObjectID string          `json:"objectId"`
			Value    json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	json.Unmarshal(result, &resp)
	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}
	return &JSHandle{session: h.session, page: h.page, objectID: resp.Result.ObjectID, value: resp.Result.Value}, nil
}

func (p *Page) EvaluateHandle(expression string) (*JSHandle, error) {
	ctx := p.browser.ctx
	objectID, err := evaluateHandle(ctx, p.session, expression)
	if err != nil {
		return nil, err
	}
	if objectID == "" {
		return nil, fmt.Errorf("expression returned null")
	}
	return &JSHandle{session: p.session, page: p, objectID: objectID}, nil
}

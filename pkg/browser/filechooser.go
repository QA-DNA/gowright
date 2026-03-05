package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type FileChooser struct {
	page       *Page
	element    string
	isMultiple bool
}

func (fc *FileChooser) SetFiles(files ...string) error {
	ctx := fc.page.browser.ctx
	objectID, err := evaluateHandle(ctx, fc.page.session, fc.element)
	if err != nil || objectID == "" {
		return fmt.Errorf("file chooser element not found")
	}
	defer releaseObject(ctx, fc.page.session, objectID)

	result, err := fc.page.session.Call(ctx, "DOM.describeNode", map[string]any{
		"objectId": objectID,
	})
	if err != nil {
		return err
	}
	var resp struct {
		Node struct {
			BackendNodeID int `json:"backendNodeId"`
		} `json:"node"`
	}
	json.Unmarshal(result, &resp)

	_, err = fc.page.session.Call(ctx, "DOM.setFileInputFiles", map[string]any{
		"files":         files,
		"backendNodeId": resp.Node.BackendNodeID,
	})
	return err
}

func (fc *FileChooser) IsMultiple() bool { return fc.isMultiple }
func (fc *FileChooser) Page() *Page      { return fc.page }

func (p *Page) WaitForFileChooser(timeout ...time.Duration) (*FileChooser, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	_, err := p.session.Call(p.browser.ctx, "Page.setInterceptFileChooserDialog", map[string]any{
		"enabled": true,
	})
	if err != nil {
		return nil, fmt.Errorf("enable file chooser interception: %w", err)
	}

	ch := make(chan *FileChooser, 1)
	p.session.OnEvent(func(method string, params json.RawMessage) {
		if method == "Page.fileChooserOpened" {
			var payload struct {
				BackendNodeID int  `json:"backendNodeId"`
				Mode          string `json:"mode"`
			}
			json.Unmarshal(params, &payload)
			fc := &FileChooser{
				page:       p,
				isMultiple: payload.Mode == "selectMultiple",
			}
			select {
			case ch <- fc:
			default:
			}
		}
	})

	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case fc := <-ch:
		return fc, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForFileChooser timed out")
	}
}

package browser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Video struct {
	page   *Page
	mu     sync.Mutex
	path   string
	frames [][]byte
	active bool
}

type VideoOptions struct {
	Dir  string
	Size *VideoSize
}

type VideoSize struct {
	Width  int
	Height int
}

func (v *Video) Start() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.active {
		return nil
	}

	ctx := v.page.browser.ctx
	params := map[string]any{
		"format":        "png",
		"everyNthFrame": 1,
	}

	_, err := v.page.session.Call(ctx, "Page.startScreencast", params)
	if err != nil {
		return fmt.Errorf("start screencast: %w", err)
	}
	v.active = true
	v.frames = nil

	v.page.session.OnEvent(func(method string, params json.RawMessage) {
		if method != "Page.screencastFrame" {
			return
		}
		v.handleFrame(params)
	})

	return nil
}

func (v *Video) handleFrame(params json.RawMessage) {
	var payload struct {
		Data      string `json:"data"`
		SessionID int    `json:"sessionId"`
	}
	json.Unmarshal(params, &payload)

	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return
	}

	v.mu.Lock()
	v.frames = append(v.frames, data)
	v.mu.Unlock()

	v.page.session.Call(v.page.browser.ctx, "Page.screencastFrameAck", map[string]any{
		"sessionId": payload.SessionID,
	})
}

func (v *Video) Stop() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if !v.active {
		return nil
	}
	v.active = false
	_, err := v.page.session.Call(v.page.browser.ctx, "Page.stopScreencast", nil)
	return err
}

func (v *Video) Path() string {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.path
}

func (v *Video) SaveAs(path string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.frames) == 0 {
		return fmt.Errorf("no video frames captured")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, v.frames[len(v.frames)-1], 0o644)
}

func (v *Video) SaveAllFrames(dir string) (int, error) {
	v.mu.Lock()
	frames := make([][]byte, len(v.frames))
	copy(frames, v.frames)
	v.mu.Unlock()

	if len(frames) == 0 {
		return 0, fmt.Errorf("no video frames captured")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	for i, data := range frames {
		path := filepath.Join(dir, fmt.Sprintf("frame_%04d.png", i))
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return i, err
		}
	}
	return len(frames), nil
}

func (v *Video) FrameCount() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.frames)
}

func (v *Video) Delete() error {
	v.mu.Lock()
	p := v.path
	v.mu.Unlock()
	if p == "" {
		return nil
	}
	return os.Remove(p)
}


package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Download struct {
	page              *Page
	guid              string
	url               string
	suggestedFilename string
	mu                sync.Mutex
	path              string
	failure           string
	done              chan struct{}
	canceled          bool
}

func (d *Download) URL() string               { return d.url }
func (d *Download) SuggestedFilename() string { return d.suggestedFilename }

func (d *Download) Path() (string, error) {
	<-d.done
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.failure != "" {
		return "", fmt.Errorf("download failed: %s", d.failure)
	}
	return d.path, nil
}

func (d *Download) SaveAs(path string) error {
	src, err := d.Path()
	if err != nil {
		return err
	}
	if src == "" {
		return fmt.Errorf("download path not available")
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0o755)
	return os.WriteFile(path, data, 0o644)
}

func (d *Download) Failure() string {
	<-d.done
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.failure
}

func (d *Download) Delete() error {
	p, err := d.Path()
	if err != nil {
		return err
	}
	if p != "" {
		return os.Remove(p)
	}
	return nil
}

func (d *Download) Cancel() {
	d.mu.Lock()
	d.canceled = true
	d.mu.Unlock()
}

func (p *Page) WaitForDownload(timeout ...time.Duration) (*Download, error) {
	t := 30 * time.Second
	if len(timeout) > 0 {
		t = timeout[0]
	}

	p.session.Call(p.browser.ctx, "Browser.setDownloadBehavior", map[string]any{
		"behavior":     "allowAndName",
		"downloadPath": os.TempDir(),
	})

	ch := make(chan *Download, 1)
	downloads := make(map[string]*Download)
	var mu sync.Mutex

	p.session.OnEvent(func(method string, params json.RawMessage) {
		switch method {
		case "Page.downloadWillBegin":
			var payload struct {
				GUID              string `json:"guid"`
				URL               string `json:"url"`
				SuggestedFilename string `json:"suggestedFilename"`
			}
			json.Unmarshal(params, &payload)
			dl := &Download{
				page:              p,
				guid:              payload.GUID,
				url:               payload.URL,
				suggestedFilename: payload.SuggestedFilename,
				done:              make(chan struct{}),
			}
			mu.Lock()
			downloads[payload.GUID] = dl
			mu.Unlock()
			select {
			case ch <- dl:
			default:
			}
		case "Page.downloadProgress":
			var payload struct {
				GUID  string `json:"guid"`
				State string `json:"state"`
			}
			json.Unmarshal(params, &payload)
			mu.Lock()
			dl := downloads[payload.GUID]
			mu.Unlock()
			if dl != nil && (payload.State == "completed" || payload.State == "canceled") {
				dl.mu.Lock()
				if payload.State == "canceled" {
					dl.failure = "canceled"
				} else {
					dl.path = filepath.Join(os.TempDir(), dl.guid)
				}
				dl.mu.Unlock()
				close(dl.done)
			}
		}
	})

	ctx, cancel := context.WithTimeout(p.browser.ctx, t)
	defer cancel()
	select {
	case dl := <-ch:
		return dl, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("waitForDownload timed out")
	}
}

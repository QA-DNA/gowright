package gowright_test

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestHarRecording(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	bc.HarStart()

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	if err := page.Goto("https://example.com"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)

	bc.HarStop()

	tmpDir := t.TempDir()
	path := tmpDir + "/recording.har"
	if err := bc.HarSaveAs(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "example.com") {
		t.Error("HAR file doesn't contain expected URL")
	}
	var har map[string]any
	json.Unmarshal(data, &har)
	log, _ := har["log"].(map[string]any)
	if log == nil {
		t.Fatal("HAR missing log field")
	}
	if log["version"] != "1.2" {
		t.Errorf("expected HAR version 1.2, got %v", log["version"])
	}
	entries, _ := log["entries"].([]any)
	if len(entries) == 0 {
		t.Fatal("HAR has no entries — automatic recording did not capture any requests")
	}
	entry, _ := entries[0].(map[string]any)
	req, _ := entry["request"].(map[string]any)
	if req == nil || !strings.Contains(req["url"].(string), "example.com") {
		t.Error("first HAR entry doesn't contain example.com URL")
	}
	resp, _ := entry["response"].(map[string]any)
	if resp == nil {
		t.Error("first HAR entry missing response")
	} else if status, _ := resp["status"].(float64); status != 200 {
		t.Errorf("expected status 200, got %v", status)
	}
}

func TestRequestTiming(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	page, err := b.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	ch := make(chan *gowright.Response, 1)
	page.On("response", func(v any) {
		if resp, ok := v.(*gowright.Response); ok {
			select {
			case ch <- resp:
			default:
			}
		}
	})

	if err := page.Goto("https://example.com"); err != nil {
		t.Fatal(err)
	}

	select {
	case resp := <-ch:
		req := resp.Request()
		if req == nil {
			t.Fatal("response has no linked request")
		}
		timing := req.Timing()
		if timing == nil {
			t.Fatal("expected non-nil timing for real request")
		}
		if timing.StartTime == 0 {
			t.Error("expected non-zero StartTime")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for response event")
	}

	req := &gowright.Request{URL: "https://example.com", Method: "GET", Headers: map[string]string{}}
	if req.Timing() != nil {
		t.Error("expected nil timing for manually created request")
	}
}

func TestRequestSizes(t *testing.T) {
	t.Parallel()
	req := &gowright.Request{URL: "https://example.com", Method: "GET", Headers: map[string]string{}}
	sizes := req.Sizes()
	if sizes != nil {
		t.Error("expected nil sizes for request without size data")
	}
}

func TestWaitForEventWithPredicate(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)
	go func() {
		time.Sleep(200 * time.Millisecond)
		page.Evaluate(`console.log("hello")`)
		time.Sleep(100 * time.Millisecond)
		page.Evaluate(`console.log("target")`)
	}()
	result, err := page.WaitForEventWith("console", gowright.WaitForEventOptions{
		Predicate: func(v any) bool {
			msg, ok := v.(*gowright.ConsoleMessage)
			if !ok {
				return false
			}
			return strings.Contains(msg.Text(), "target")
		},
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	msg, ok := result.(*gowright.ConsoleMessage)
	if !ok {
		t.Fatal("expected ConsoleMessage")
	}
	if !strings.Contains(msg.Text(), "target") {
		t.Errorf("expected 'target', got %q", msg.Text())
	}
}

func TestWaitForURLGlob(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>start</div>`)
	go func() {
		time.Sleep(200 * time.Millisecond)
		page.Goto("data:text/html,<div>navigated</div>")
	}()
	err := page.WaitForURL("**navigated**")
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForURLRegex(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>start</div>`)
	go func() {
		time.Sleep(200 * time.Millisecond)
		page.Goto("data:text/html,<div>regex-test-123</div>")
	}()
	err := page.WaitForURL("", gowright.WaitForURLOptions{
		Regex:   regexp.MustCompile(`regex-test-\d+`),
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForURLExact(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>start</div>`)
	url := page.URL()
	err := page.WaitForURL(url, gowright.WaitForURLOptions{Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForURLTimeout(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>start</div>`)
	err := page.WaitForURL("https://never-gonna-match.example.com", gowright.WaitForURLOptions{Timeout: 500 * time.Millisecond})
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestVideoAllFramesToDisk(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div style="width:200px;height:200px;background:blue">frame test</div>`)
	video := page.Video()
	if err := video.Start(); err != nil {
		t.Fatal(err)
	}
	page.Evaluate(`document.querySelector('div').style.background = 'red'`)
	time.Sleep(1 * time.Second)
	page.Evaluate(`document.querySelector('div').style.background = 'green'`)
	time.Sleep(500 * time.Millisecond)
	if err := video.Stop(); err != nil {
		t.Fatal(err)
	}

	count := video.FrameCount()
	if count == 0 {
		t.Fatal("no frames captured")
	}
	t.Logf("captured %d frames", count)

	tmpDir := t.TempDir()
	framesDir := tmpDir + "/frames"
	written, err := video.SaveAllFrames(framesDir)
	if err != nil {
		t.Fatal(err)
	}
	if written != count {
		t.Errorf("expected %d frames written, got %d", count, written)
	}

	entries, _ := os.ReadDir(framesDir)
	if len(entries) != count {
		t.Errorf("expected %d files, got %d", count, len(entries))
	}
	for _, e := range entries {
		info, _ := e.Info()
		if info.Size() == 0 {
			t.Errorf("frame %s is empty", e.Name())
		}
	}
}

func TestRouteFromHAR(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	harPath := tmpDir + "/test.har"
	harData := `{
		"log": {
			"version": "1.2",
			"entries": [{
				"startedDateTime": "2024-01-01T00:00:00Z",
				"request": {
					"method": "GET",
					"url": "https://hartest.example.com/api/data",
					"httpVersion": "HTTP/1.1",
					"headers": [],
					"queryString": [],
					"bodySize": 0
				},
				"response": {
					"status": 200,
					"statusText": "OK",
					"httpVersion": "HTTP/1.1",
					"headers": [{"name": "content-type", "value": "application/json"}],
					"content": {
						"size": 27,
						"mimeType": "application/json",
						"text": "{\"message\":\"from har\"}"
					},
					"bodySize": 27
				}
			}]
		}
	}`
	os.WriteFile(harPath, []byte(harData), 0o644)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.RouteFromHAR(harPath); err != nil {
		t.Fatal(err)
	}
	page.Route("**hartest.example.com**", func(route *gowright.Route, req *gowright.Request) {
		route.Fulfill(gowright.FulfillOptions{
			Status:      200,
			ContentType: "text/html",
			Body:        []byte(`<html><body>ok</body></html>`),
		})
	})

	page.Goto("https://hartest.example.com/")

	result, err := page.Evaluate(`fetch("https://hartest.example.com/api/data").then(r => r.json()).then(d => d.message)`)
	if err != nil {
		t.Fatal(err)
	}
	var msg string
	json.Unmarshal(result, &msg)
	if msg != "from har" {
		t.Errorf("expected 'from har', got %q", msg)
	}
}

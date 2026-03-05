package gowright_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

func TestWaitForFunction(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<script>var ready=false; setTimeout(function(){ ready=true; }, 300);</script>
	`)

	_, err := page.WaitForFunction("ready")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddScriptTag(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	err := page.AddScriptTag(gowright.AddScriptTagOptions{Content: "document.title='injected'"})
	if err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "injected" {
		t.Errorf("expected title 'injected', got %q", title)
	}
}

func TestAddStyleTag(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="box" style="width:100px;height:100px">test</div>`)

	err := page.AddStyleTag(gowright.AddStyleTagOptions{Content: "#box { display: none; }"})
	if err != nil {
		t.Fatal(err)
	}

	hidden, err := page.Locator("#box").IsHidden()
	if err != nil {
		t.Fatal(err)
	}
	if !hidden {
		t.Errorf("expected #box to be hidden after injecting style")
	}
}

func TestEmulateMedia(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	err := page.EmulateMedia(gowright.EmulateMediaOptions{ColorScheme: "dark"})
	if err != nil {
		t.Fatal(err)
	}

	val, err := page.Evaluate(`window.matchMedia('(prefers-color-scheme: dark)').matches`)
	if err != nil {
		t.Fatal(err)
	}
	var matches bool
	json.Unmarshal(val, &matches)
	if !matches {
		t.Errorf("expected prefers-color-scheme: dark to match")
	}
}

func TestPagePDF(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>PDF test</div>`)

	data, err := page.PDF()
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("PDF returned empty bytes")
	}
	if !strings.HasPrefix(string(data), "%PDF") {
		t.Errorf("PDF does not start with %%PDF magic bytes")
	}
}

func TestSetDefaultTimeout(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	page.SetDefaultTimeout(500 * time.Millisecond)

	start := time.Now()
	err := page.Locator("#nonexistent").Click()
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error for missing element")
	}
	if elapsed > 5*time.Second {
		t.Errorf("expected quick timeout, took %v", elapsed)
	}
}

func TestIsClosed(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	if page.IsClosed() {
		t.Fatal("expected page to not be closed initially")
	}

	err := page.Close()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(200 * time.Millisecond)

	if !page.IsClosed() {
		t.Errorf("expected page to be closed after Close()")
	}
}

func TestViewportSize(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	err := page.SetViewportSize(1024, 768)
	if err != nil {
		t.Fatal(err)
	}

	w, h := page.ViewportSize()
	if w != 1024 || h != 768 {
		t.Errorf("expected viewport 1024x768, got %dx%d", w, h)
	}
}

func TestSetExtraHTTPHeaders(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	err := page.SetExtraHTTPHeaders(map[string]string{
		"X-Custom-Header": "test-value",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConsoleMessage(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	msgCh := make(chan *gowright.ConsoleMessage, 1)
	page.On("console", func(payload any) {
		if cm, ok := payload.(*gowright.ConsoleMessage); ok {
			select {
			case msgCh <- cm:
			default:
			}
		}
	})

	_, err := page.Evaluate(`console.log('hello')`)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case msg := <-msgCh:
		if msg.Text() != "hello" {
			t.Errorf("expected console text 'hello', got %q", msg.Text())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("console message not received")
	}
}

func TestPageError(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	errCh := make(chan any, 1)
	page.On("pageerror", func(payload any) {
		select {
		case errCh <- payload:
		default:
		}
	})

	_, _ = page.Evaluate(`setTimeout(function(){ throw new Error('boom'); }, 100)`)

	select {
	case <-errCh:
	case <-time.After(5 * time.Second):
		t.Fatal("pageerror event not received")
	}
}

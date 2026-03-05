package gowright_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

func TestPageReload(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<title>Initial</title>`)

	// Change title via JS, then reload to get original back
	page.Evaluate(`document.title = "Changed"`)
	title, _ := page.Title()
	if title != "Changed" {
		t.Fatalf("expected 'Changed', got %q", title)
	}

	if err := page.Reload(); err != nil {
		t.Fatal(err)
	}

	title, _ = page.Title()
	if title != "Initial" {
		t.Errorf("expected 'Initial' after reload, got %q", title)
	}
}

func TestPageContent(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="test">Hello</div>`)

	content, err := page.Content()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, `id="test"`) {
		t.Errorf("content missing test div: %s", content[:min(200, len(content))])
	}
	if !strings.Contains(content, "Hello") {
		t.Errorf("content missing 'Hello': %s", content[:min(200, len(content))])
	}
}

func TestPageSetContent(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>Old</div>`)

	err := page.SetContent(`<html><body><h1 id="new">New Content</h1></body></html>`)
	if err != nil {
		t.Fatal(err)
	}

	text, err := page.Locator("#new").TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "New Content" {
		t.Errorf("expected 'New Content', got %q", text)
	}
}

func TestPageSetViewportSize(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>viewport test</div>`)

	if err := page.SetViewportSize(800, 600); err != nil {
		t.Fatal(err)
	}

	val, err := page.Evaluate(`JSON.stringify({w: window.innerWidth, h: window.innerHeight})`)
	if err != nil {
		t.Fatal(err)
	}
	var sizeStr string
	json.Unmarshal(val, &sizeStr)
	var size struct {
		W int `json:"w"`
		H int `json:"h"`
	}
	json.Unmarshal([]byte(sizeStr), &size)

	if size.W != 800 || size.H != 600 {
		t.Errorf("expected 800x600, got %dx%d", size.W, size.H)
	}
}

func TestPageWaitForSelector(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="container"></div>
		<script>
			setTimeout(function() {
				var el = document.createElement('span');
				el.id = 'delayed';
				el.textContent = 'Appeared';
				document.getElementById('container').appendChild(el);
			}, 300);
		</script>
	`)

	err := page.WaitForSelector("#delayed")
	if err != nil {
		t.Fatal(err)
	}

	text, _ := page.Locator("#delayed").TextContent()
	if text != "Appeared" {
		t.Errorf("expected 'Appeared', got %q", text)
	}
}

func TestPageWaitForSelectorDetached(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="removeme">Going away</div>
		<script>
			setTimeout(function() {
				document.getElementById('removeme').remove();
			}, 300);
		</script>
	`)

	err := page.WaitForSelector("#removeme", gowright.WaitForSelectorOptions{
		State:   "detached",
		Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestPageDialog(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<button id="btn">Alert</button>`)

	dialogMsg := make(chan string, 1)
	page.OnDialog(func(d *gowright.Dialog) {
		dialogMsg <- d.Message()
		d.Accept()
	})

	page.Evaluate(`setTimeout(function() { alert("Hello from dialog"); }, 100)`)

	select {
	case msg := <-dialogMsg:
		if msg != "Hello from dialog" {
			t.Errorf("expected 'Hello from dialog', got %q", msg)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("dialog not received")
	}
}

func TestPageEvaluateWithArg(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>test</div>`)

	val, err := page.EvaluateWithArg(`function(x) { return x * 2; }`, 21)
	if err != nil {
		t.Fatal(err)
	}

	var result int
	json.Unmarshal(val, &result)
	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}
}

func TestLocatorInnerHTML(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="parent"><span>child</span></div>`)

	html, err := page.Locator("#parent").InnerHTML()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<span>child</span>") {
		t.Errorf("expected innerHTML to contain '<span>child</span>', got %q", html)
	}
}

func TestLocatorBoundingBox(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="box" style="position:absolute;left:10px;top:20px;width:100px;height:50px;"></div>
	`)

	box, err := page.Locator("#box").BoundingBox()
	if err != nil {
		t.Fatal(err)
	}
	if box.Width != 100 || box.Height != 50 {
		t.Errorf("expected 100x50, got %.0fx%.0f", box.Width, box.Height)
	}
}

func TestLocatorIsHiddenAndEditable(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="hidden" style="display:none">Hidden</div>
		<input id="editable" type="text" />
		<input id="readonly" type="text" readonly />
	`)

	hidden, _ := page.Locator("#hidden").IsHidden()
	if !hidden {
		t.Error("expected #hidden to be hidden")
	}

	editable, _ := page.Locator("#editable").IsEditable()
	if !editable {
		t.Error("expected #editable to be editable")
	}

	editable2, _ := page.Locator("#readonly").IsEditable()
	if editable2 {
		t.Error("expected #readonly to not be editable")
	}
}

func TestLocatorClear(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp" type="text" value="hello" />`)

	page.Locator("#inp").Clear()

	val, _ := page.Locator("#inp").InputValue()
	if val != "" {
		t.Errorf("expected empty after Clear, got %q", val)
	}
}

func TestLocatorWaitFor(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="container"></div>
		<script>
			setTimeout(function() {
				var el = document.createElement('div');
				el.id = 'target';
				el.textContent = 'Ready';
				document.getElementById('container').appendChild(el);
			}, 300);
		</script>
	`)

	err := page.Locator("#target").WaitFor()
	if err != nil {
		t.Fatal(err)
	}

	text, _ := page.Locator("#target").TextContent()
	if text != "Ready" {
		t.Errorf("expected 'Ready', got %q", text)
	}
}

func TestLocatorEvaluate(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="el" data-count="5">Hello</div>`)

	val, err := page.Locator("#el").Evaluate(`function() { return parseInt(this.getAttribute('data-count')); }`)
	if err != nil {
		t.Fatal(err)
	}
	var count int
	json.Unmarshal(val, &count)
	if count != 5 {
		t.Errorf("expected 5, got %d", count)
	}
}

func TestNetworkRoute(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>Route test</div>`)

	// Single route handler for all requests to example.com
	handlerCalled := make(chan error, 1)
	page.Route("**example.com**", func(route *gowright.Route, req *gowright.Request) {
		if strings.Contains(req.URL, "/api/data") {
			err := route.Fulfill(gowright.FulfillOptions{
				Status:      200,
				ContentType: "application/json",
				Body:        []byte(`{"mocked": true}`),
			})
			handlerCalled <- err
			return
		}
		route.Fulfill(gowright.FulfillOptions{
			Status:      200,
			ContentType: "text/html",
			Body:        []byte(`<html><body>ok</body></html>`),
		})
	})

	page.Goto("http://example.com")

	val, err := page.Evaluate(`
		fetch('/api/data').then(r => r.json()).then(d => JSON.stringify(d)).catch(e => 'error:' + e.message)
	`)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case handlerErr := <-handlerCalled:
		if handlerErr != nil {
			t.Fatalf("route handler error: %v", handlerErr)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("route handler was not called")
	}

	var result string
	json.Unmarshal(val, &result)
	if !strings.Contains(result, `"mocked":true`) && !strings.Contains(result, `"mocked": true`) {
		t.Errorf("expected mocked response, got %q", result)
	}
}

func TestNetworkAbort(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>Abort test</div>`)

	// Give us a proper origin without real network
	page.Route("**example.com**", func(route *gowright.Route, req *gowright.Request) {
		if strings.Contains(req.URL, "/blocked") {
			route.Abort()
			return
		}
		route.Fulfill(gowright.FulfillOptions{
			Status:      200,
			ContentType: "text/html",
			Body:        []byte(`<html><body>ok</body></html>`),
		})
	})
	page.Goto("http://example.com")

	val, err := page.Evaluate(`
		fetch('http://example.com/blocked').then(() => 'ok').catch(() => 'blocked')
	`)
	if err != nil {
		t.Fatal(err)
	}

	var result string
	json.Unmarshal(val, &result)
	if result != "blocked" {
		t.Errorf("expected 'blocked', got %q", result)
	}
}

func TestPageScreenshotWithOptions(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div style="width:200px;height:200px;background:red;"></div>
	`)

	data, err := page.ScreenshotWithOptions(gowright.ScreenshotOptions{
		Type: "jpeg",
	})
	if err != nil {
		t.Fatal(err)
	}
	// JPEG magic bytes
	if len(data) < 2 || data[0] != 0xFF || data[1] != 0xD8 {
		t.Error("screenshot is not a valid JPEG")
	}
}

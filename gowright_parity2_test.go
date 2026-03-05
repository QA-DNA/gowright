package gowright_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestBrowserContexts(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	bc1, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc1.Close() })
	bc2, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc2.Close() })
	contexts := b.Contexts()
	if len(contexts) < 2 {
		t.Errorf("expected at least 2 contexts, got %d", len(contexts))
	}
}

func TestBrowserIsConnected(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	if !b.IsConnected() {
		t.Error("expected IsConnected to be true")
	}
}

func TestContextBrowser(t *testing.T) {
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
	if bc.Browser() == nil {
		t.Error("Browser() returned nil")
	}
}

func TestFrameMainFrame(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>frame test</div>")
	mf := page.MainFrame()
	if mf == nil {
		t.Fatal("MainFrame() is nil")
	}
	if mf.URL() == "" {
		t.Error("MainFrame URL is empty")
	}
}

func TestFrameTitle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<title>My Title</title><div>content</div>")
	mf := page.MainFrame()
	title, err := mf.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "My Title" {
		t.Errorf("expected 'My Title', got %q", title)
	}
}

func TestFrameContent(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div id='test'>frame content</div>")
	mf := page.MainFrame()
	content, err := mf.Content()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "frame content") {
		t.Errorf("content missing expected text: %s", content[:min(200, len(content))])
	}
}

func TestFrameSetContent(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>original</div>")
	mf := page.MainFrame()
	err := mf.SetContent("<div id='new'>replaced</div>")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := page.Content()
	if !strings.Contains(content, "replaced") {
		t.Error("SetContent didn't work")
	}
}

func TestFramePage(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	mf := page.MainFrame()
	if mf.Page() == nil {
		t.Error("Page() returned nil")
	}
}

func TestFrames(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	frames := page.Frames()
	if len(frames) < 1 {
		t.Errorf("expected at least 1 frame, got %d", len(frames))
	}
}

func TestLocatorGetByTextChained(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="parent"><span>hello</span><span>world</span></div>`)
	loc := page.Locator("#parent").GetByText("world")
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "world") {
		t.Errorf("expected 'world', got %q", text)
	}
}

func TestLocatorGetByRoleChained(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="container"><button>Submit</button></div>`)
	loc := page.Locator("#container").GetByRole("button")
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Submit") {
		t.Errorf("expected 'Submit', got %q", text)
	}
}

func TestLocatorGetByTestIdChained(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="wrap"><span data-testid="item">test item</span></div>`)
	loc := page.Locator("#wrap").GetByTestId("item")
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "test item") {
		t.Errorf("expected 'test item', got %q", text)
	}
}

func TestLocatorEvaluateHandle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="elem">eval test</div>`)
	handle, err := page.Locator("#elem").EvaluateHandle("function() { return {tag: this.tagName}; }")
	if err != nil {
		t.Fatal(err)
	}
	val, _ := handle.JsonValue()
	var result map[string]string
	json.Unmarshal(val, &result)
	if result["tag"] != "DIV" {
		t.Errorf("expected DIV, got %s", result["tag"])
	}
	handle.Dispose()
}

func TestResponseText(t *testing.T) {
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
	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		page.Goto("https://example.com")
	}()
	resp, err := page.WaitForResponse("**example.com**", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	text, err := resp.Text()
	if err != nil {
		t.Fatal(err)
	}
	if text == "" {
		t.Error("expected non-empty response text")
	}
}

func TestResponseJson(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	val, err := page.Evaluate(`
		fetch("data:application/json," + encodeURIComponent('{"key":"value"}'))
			.then(function(r) { return r.json(); })
			.then(function(j) { return JSON.stringify(j); })
	`)
	if err != nil {
		t.Fatal(err)
	}
	var text string
	json.Unmarshal(val, &text)
	var m map[string]string
	json.Unmarshal([]byte(text), &m)
	if m["key"] != "value" {
		t.Errorf("expected value, got %s", m["key"])
	}
}

func TestResponseHeaderValues(t *testing.T) {
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
	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(200 * time.Millisecond)
		page.Goto("https://example.com")
	}()
	resp, err := page.WaitForResponse("**example.com**", 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	headers := resp.AllHeaders()
	if len(headers) == 0 {
		t.Error("expected at least one header")
	}
}

func TestFrameLocatorFirstNthLast(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<iframe name="f1"></iframe><iframe name="f2"></iframe>`)
	fl := page.FrameLocator("iframe")
	first := fl.First()
	if first == nil {
		t.Fatal("First() returned nil")
	}
	nth := fl.Nth(1)
	if nth == nil {
		t.Fatal("Nth(1) returned nil")
	}
	last := fl.Last()
	if last == nil {
		t.Fatal("Last() returned nil")
	}
}

func TestJSHandleGetProperty(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	handle, _ := page.EvaluateHandle("({name: 'test', age: 42})")
	prop, err := handle.GetProperty("name")
	if err != nil {
		t.Fatal(err)
	}
	val, _ := prop.JsonValue()
	var s string
	json.Unmarshal(val, &s)
	if s != "test" {
		t.Errorf("expected 'test', got %q", s)
	}
	handle.Dispose()
}

func TestJSHandleEvaluateHandle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	handle, _ := page.EvaluateHandle("({items: [1,2,3]})")
	sub, err := handle.EvaluateHandle("function() { return this.items; }")
	if err != nil {
		t.Fatal(err)
	}
	val, _ := sub.JsonValue()
	var items []int
	json.Unmarshal(val, &items)
	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
	handle.Dispose()
	sub.Dispose()
}

func TestCustomSelectorEngine(t *testing.T) {
	t.Parallel()
	selectors := gowright.GetSelectors()
	selectors.Register("mytag", gowright.SelectorEngine{
		QueryAll: `return Array.from(root.querySelectorAll(selector));`,
	})
	_, page := setupPage(t, `<article>custom engine</article>`)
	loc := page.Locator("mytag=article")
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "custom engine" {
		t.Errorf("expected 'custom engine', got %q", text)
	}
}

func TestCustomSelectorEngineDuplicate(t *testing.T) {
	t.Parallel()
	selectors := gowright.GetSelectors()
	selectors.Register("duptest", gowright.SelectorEngine{QueryAll: `return [];`})
	err := selectors.Register("duptest", gowright.SelectorEngine{QueryAll: `return [];`})
	if err == nil {
		t.Error("expected error when registering duplicate engine")
	}
}

func TestFrameExecutionContext(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<iframe srcdoc="<div id='child'>iframe content</div>"></iframe>`)
	time.Sleep(500 * time.Millisecond)
	frames := page.Frames()
	if len(frames) < 2 {
		t.Fatalf("expected at least 2 frames, got %d", len(frames))
	}
	var childFrame *gowright.Frame
	for _, f := range frames {
		if f != page.MainFrame() {
			childFrame = f
			break
		}
	}
	if childFrame == nil {
		t.Fatal("child frame not found")
	}
	val, err := childFrame.Evaluate("document.getElementById('child').textContent")
	if err != nil {
		t.Fatal(err)
	}
	var text string
	json.Unmarshal(val, &text)
	if text != "iframe content" {
		t.Errorf("expected 'iframe content', got %q", text)
	}
}

func TestFrameExecutionContextIsolation(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="parent">parent</div><iframe srcdoc="<div id='parent'>child</div>"></iframe>`)
	time.Sleep(500 * time.Millisecond)
	frames := page.Frames()
	var childFrame *gowright.Frame
	for _, f := range frames {
		if f != page.MainFrame() {
			childFrame = f
			break
		}
	}
	if childFrame == nil {
		t.Fatal("child frame not found")
	}
	mainVal, err := page.MainFrame().Evaluate("document.getElementById('parent').textContent")
	if err != nil {
		t.Fatal(err)
	}
	var mainText string
	json.Unmarshal(mainVal, &mainText)
	childVal, err := childFrame.Evaluate("document.getElementById('parent').textContent")
	if err != nil {
		t.Fatal(err)
	}
	var childText string
	json.Unmarshal(childVal, &childText)
	if mainText != "parent" {
		t.Errorf("main frame: expected 'parent', got %q", mainText)
	}
	if childText != "child" {
		t.Errorf("child frame: expected 'child', got %q", childText)
	}
}

func TestBrowserNewPage(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	if err := page.Goto("data:text/html,<div>newpage</div>"); err != nil {
		t.Fatal(err)
	}
	title, _ := page.Title()
	_ = title
}

func TestLocatorStrictMode(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div class="item">one</div><div class="item">two</div>`)
	loc := page.Locator(".item")
	err := loc.Click()
	if err == nil {
		t.Error("expected strict mode error when selector matches multiple elements")
	}
	if err != nil && !strings.Contains(err.Error(), "strict mode violation") {
		t.Errorf("expected strict mode violation error, got: %v", err)
	}
}

func TestLocatorFirstBypassesStrict(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div class="item" onclick="document.title='clicked'">one</div><div class="item">two</div>`)
	err := page.Locator(".item").First().Click()
	if err != nil {
		t.Fatal(err)
	}
	title, _ := page.Title()
	if title != "clicked" {
		t.Errorf("expected title 'clicked', got %q", title)
	}
}

func TestStrictModeGetByText(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<span>hello</span><span>hello</span>`)
	err := page.GetByText("hello").Click()
	if err == nil {
		t.Error("expected strict mode error for GetByText matching multiple elements")
	}
	if err != nil && !strings.Contains(err.Error(), "strict mode violation") {
		t.Errorf("expected strict mode violation, got: %v", err)
	}
}

func TestStrictModeGetByRole(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<button>A</button><button>B</button>`)
	err := page.GetByRole("button").Click()
	if err == nil {
		t.Error("expected strict mode error for GetByRole matching multiple elements")
	}
	if err != nil && !strings.Contains(err.Error(), "strict mode violation") {
		t.Errorf("expected strict mode violation, got: %v", err)
	}
}

func TestStrictModeGetByTextFirst(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<span onclick="document.title='ok'">hello</span><span>hello</span>`)
	err := page.GetByText("hello").First().Click()
	if err != nil {
		t.Fatal(err)
	}
	title, _ := page.Title()
	if title != "ok" {
		t.Errorf("expected title 'ok', got %q", title)
	}
}

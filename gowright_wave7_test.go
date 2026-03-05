package gowright_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestBrowserType(t *testing.T) {
	t.Parallel()
	bt := gowright.Chromium()
	if bt.Name() != "chromium" {
		t.Errorf("expected 'chromium', got %q", bt.Name())
	}
}

func TestBrowserTypeExecutablePath(t *testing.T) {
	t.Parallel()
	bt := gowright.Chromium()
	path, err := bt.ExecutablePath()
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("empty executable path")
	}
}

func TestBrowserTypeLaunch(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	bt := gowright.Chromium()
	b, err := bt.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	v, err := b.Version()
	if err != nil {
		t.Fatal(err)
	}
	if v == "" {
		t.Fatal("empty version")
	}
}

func TestSelectors(t *testing.T) {
	t.Parallel()
	sel := gowright.GetSelectors()
	sel.SetTestIdAttribute("data-id")
	if sel.TestIdAttribute() != "data-id" {
		t.Error("wrong test id attribute")
	}
	sel.SetTestIdAttribute("data-testid")
}

func TestPageDragAndDrop(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="src" draggable="true" style="width:50px;height:50px;background:red">drag</div>
		<div id="dst" style="width:50px;height:50px;background:blue">drop</div>
	`)

	err := page.DragAndDrop("#src", "#dst")
	if err != nil {
		t.Fatal(err)
	}
}

func TestPageTapShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button id="btn" onclick="document.title='page-tapped'">tap</button>
	`)

	if err := page.Tap("#btn"); err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "page-tapped" {
		t.Errorf("expected title 'page-tapped', got %q", title)
	}
}

func TestPageSetInputFiles(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<input id="file" type="file" />
	`)

	err := page.SetInputFiles("#file")
	_ = err
}

func TestRequestResponseAPIs(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	page.Route("**example.com**", func(route *gowright.Route, req *gowright.Request) {
		_ = req.AllHeaders()
		_ = req.HeaderValue("Content-Type")
		_ = req.ResourceType()
		_ = req.IsNavigationRequest()
		_ = req.RedirectedFrom()
		_ = req.RedirectedTo()
		route.Fulfill(gowright.FulfillOptions{
			Status:      200,
			Body:        []byte("ok"),
			ContentType: "text/html",
		})
	})

	if err := page.Goto("http://example.com"); err != nil {
		t.Fatal(err)
	}

	content, err := page.Content()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "ok") {
		t.Errorf("expected page to contain 'ok', got %s", content[:min(200, len(content))])
	}
}

func TestPageUnrouteAll(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	page.Route("**", func(route *gowright.Route, req *gowright.Request) {
		route.Continue()
	})

	page.UnrouteAll()
}

package gowright_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestClockInstall(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	clock := page.Clock()
	if err := clock.Install(); err != nil {
		t.Fatal(err)
	}
	if err := clock.SetSystemTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	now, err := clock.Now()
	if err != nil {
		t.Fatal(err)
	}
	expected := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	diff := now.Sub(expected)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("expected time near %v, got %v (diff %v)", expected, now, diff)
	}
}

func TestClockFastForward(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	clock := page.Clock()
	if err := clock.Install(); err != nil {
		t.Fatal(err)
	}
	before, err := clock.Now()
	if err != nil {
		t.Fatal(err)
	}
	if err := clock.FastForward(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	after, err := clock.Now()
	if err != nil {
		t.Fatal(err)
	}
	diff := after.Sub(before)
	if diff < 4900*time.Millisecond || diff > 5100*time.Millisecond {
		t.Errorf("expected ~5s advance, got %v", diff)
	}
}

func TestClockPauseAndResume(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	clock := page.Clock()
	if err := clock.Install(); err != nil {
		t.Fatal(err)
	}
	if err := clock.PauseAt(time.Date(2020, 6, 15, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := clock.Resume(); err != nil {
		t.Fatal(err)
	}
}

func TestClockNotInstalled(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	clock := page.Clock()
	_, err := clock.Now()
	if err == nil {
		t.Error("expected error when calling Now() without Install()")
	}
}

func TestAccessibilitySnapshot(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<button>Click me</button><input placeholder="Name">`)
	a := page.Accessibility()
	snapshot, err := a.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot == nil {
		t.Fatal("expected non-nil snapshot")
	}
	if snapshot.Role == "" {
		t.Error("expected snapshot to have a role")
	}
}

func TestVideoAccessor(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")
	video := page.Video()
	if video == nil {
		t.Fatal("Video() returned nil")
	}
	if video.Path() != "" {
		t.Error("expected empty path before recording")
	}
}

func TestVideoFrameCapture(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div style="width:200px;height:200px;background:red">video test</div>`)
	video := page.Video()
	if err := video.Start(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1 * time.Second)
	if err := video.Stop(); err != nil {
		t.Fatal(err)
	}
	tmpDir := t.TempDir()
	path := tmpDir + "/frame.png"
	if err := video.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Error("saved frame is empty")
	}
}

func TestPageClickShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<button id="btn" onclick="document.title='shortcut'">Click</button>`)
	if err := page.Click("#btn"); err != nil {
		t.Fatal(err)
	}
	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "shortcut" {
		t.Errorf("expected title 'shortcut', got %q", title)
	}
}

func TestPageFillShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp">`)
	if err := page.Fill("#inp", "hello"); err != nil {
		t.Fatal(err)
	}
	val, err := page.InputValue("#inp")
	if err != nil {
		t.Fatal(err)
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
}

func TestPageCheckShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input type="checkbox" id="cb">`)
	if err := page.Check("#cb"); err != nil {
		t.Fatal(err)
	}
	checked, err := page.IsChecked("#cb")
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Error("expected checkbox to be checked")
	}
}

func TestPageHoverShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="target" onmouseenter="document.title='hovered'">Hover</div>`)
	if err := page.Hover("#target"); err != nil {
		t.Fatal(err)
	}
	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "hovered" {
		t.Errorf("expected title 'hovered', got %q", title)
	}
}

func TestPageTextContentShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="d">hello world</div>`)
	text, err := page.TextContent("#d")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Errorf("expected 'hello world', got %q", text)
	}
}

func TestPageInnerHTMLShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="d"><span>inner</span></div>`)
	html, err := page.InnerHTML("#d")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(html, "<span>inner</span>") {
		t.Errorf("expected innerHTML to contain '<span>inner</span>', got %q", html)
	}
}

func TestPageIsVisibleShortcut(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="vis">visible</div><div id="hid" style="display:none">hidden</div>`)
	vis, err := page.IsVisible("#vis")
	if err != nil {
		t.Fatal(err)
	}
	if !vis {
		t.Error("expected #vis to be visible")
	}
	hid, err := page.IsVisible("#hid")
	if err != nil {
		t.Fatal(err)
	}
	if hid {
		t.Error("expected #hid to be hidden")
	}
}

func TestPageContext(t *testing.T) {
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
	if err := page.Goto("data:text/html,<div>test</div>"); err != nil {
		t.Fatal(err)
	}
	if page.Context() == nil {
		t.Error("Context() returned nil")
	}
}

func TestPageAddLocatorHandler(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="overlay">overlay</div>`)
	loc := page.Locator("#overlay")
	called := false
	page.AddLocatorHandler(loc, func(l *gowright.Locator) {
		called = true
	})
	page.RemoveLocatorHandler(loc)
	_ = called
}

func TestPageWaitForNavigation(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>start</div>`)
	go func() {
		time.Sleep(100 * time.Millisecond)
		page.Goto("data:text/html,<div>navigated</div>")
	}()
	_, err := page.WaitForNavigation()
	if err != nil {
		t.Fatal(err)
	}
}

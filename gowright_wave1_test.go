package gowright_test

import (
	"context"
	"testing"
	"time"
)

func TestRetryBackoff(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="container"></div>
		<script>setTimeout(function(){ var d=document.createElement('div'); d.id='delayed'; d.textContent='found'; document.getElementById('container').appendChild(d); }, 200);</script>
	`)

	err := page.Locator("#delayed").Click()
	if err != nil {
		t.Fatal(err)
	}

	text, err := page.Locator("#delayed").TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "found" {
		t.Errorf("expected 'found', got %q", text)
	}
}

func TestActionabilityStable(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="mover" style="position:absolute;left:0" onclick="document.title='stable-clicked'">Click</div>
		<script>
		var el=document.getElementById('mover');
		var pos=0;
		var iv=setInterval(function(){ pos+=10; el.style.left=pos+'px'; if(pos>=50){clearInterval(iv);} }, 50);
		</script>
	`)

	time.Sleep(400 * time.Millisecond)

	err := page.Locator("#mover").Click()
	if err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "stable-clicked" {
		t.Errorf("expected title 'stable-clicked', got %q", title)
	}
}

func TestActionabilityHitTarget(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button id="btn" onclick="document.title='hit'">Click</button>
		<div id="overlay" style="position:absolute;top:0;left:0;width:100%;height:100%;background:red;z-index:999"></div>
		<script>setTimeout(function(){ document.getElementById('overlay').remove(); }, 300);</script>
	`)

	err := page.Locator("#btn").Click()
	if err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "hit" {
		t.Errorf("expected title 'hit', got %q", title)
	}
}

func TestFrameManagement(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<iframe name="child" srcdoc="<div id='inner'>inside</div>"></iframe>
	`)

	time.Sleep(500 * time.Millisecond)

	frames := page.Frames()
	if len(frames) < 2 {
		t.Errorf("expected at least 2 frames, got %d", len(frames))
	}

	main := page.MainFrame()
	if main == nil {
		t.Fatal("MainFrame() returned nil")
	}
}

func TestKeyboardStandalone(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp" type="text" />`)

	err := page.Locator("#inp").Focus()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	err = page.Keyboard().Type(ctx, "hello")
	if err != nil {
		t.Fatal(err)
	}

	val, err := page.Locator("#inp").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "hello" {
		t.Errorf("expected 'hello', got %q", val)
	}
}

func TestMouseStandalone(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button id="btn" style="position:absolute;left:50px;top:50px;width:100px;height:40px" onclick="document.title='mouse-click'">Click</button>
	`)

	ctx := context.Background()
	err := page.Mouse().Click(ctx, 100, 70)
	if err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "mouse-click" {
		t.Errorf("expected title 'mouse-click', got %q", title)
	}
}

func TestTouchscreenTap(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button id="btn" style="position:absolute;left:50px;top:50px;width:100px;height:40px" ontouchstart="document.title='tapped'">Tap</button>
	`)

	ctx := context.Background()
	err := page.Touchscreen().Tap(ctx, 100, 70)
	if err != nil {
		t.Fatal(err)
	}
}

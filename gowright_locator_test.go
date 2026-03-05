package gowright_test

import (
	"context"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

// Helper to get a page with a loaded HTML document.
func setupPage(t *testing.T, html string) (*gowright.Browser, *gowright.Page) {
	t.Helper()
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

	// Load HTML via data URL
	dataURL := "data:text/html," + html
	if err := page.Goto(dataURL); err != nil {
		t.Fatal(err)
	}

	return b, page
}

func TestLocatorClick(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button id="btn" onclick="document.title='clicked'">Click Me</button>
	`)

	page.Locator("#btn").Click()

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}
	if title != "clicked" {
		t.Errorf("expected title 'clicked', got %q", title)
	}
}

func TestLocatorFill(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<input id="name" type="text" value="old value" />
	`)

	page.Locator("#name").Fill("hello world")

	val, err := page.Locator("#name").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "hello world" {
		t.Errorf("expected 'hello world', got %q", val)
	}
}

func TestLocatorGetByText(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div>
			<span>First Item</span>
			<span id="target">Second Item</span>
			<span>Third Item</span>
		</div>
	`)

	text, err := page.GetByText("Second").TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "Second Item" {
		t.Errorf("expected 'Second Item', got %q", text)
	}
}

func TestLocatorGetByRole(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<button>Submit Form</button>
		<button>Cancel</button>
	`)

	err := page.GetByRole("button", gowright.ByRoleOption{Name: "Submit"}).Click()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocatorGetByTestId(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div data-testid="greeting">Hello World</div>
	`)

	text, err := page.GetByTestId("greeting").TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "Hello World" {
		t.Errorf("expected 'Hello World', got %q", text)
	}
}

func TestLocatorGetByLabel(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<label for="email">Email Address</label>
		<input id="email" type="email" />
	`)

	page.GetByLabel("Email").Fill("test@example.com")

	val, err := page.Locator("#email").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "test@example.com" {
		t.Errorf("expected 'test@example.com', got %q", val)
	}
}

func TestLocatorGetByPlaceholder(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<input placeholder="Search..." type="text" />
	`)

	page.GetByPlaceholder("Search...").Fill("gowright")

	val, err := page.GetByPlaceholder("Search...").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "gowright" {
		t.Errorf("expected 'gowright', got %q", val)
	}
}

func TestLocatorChaining(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="form1">
			<input type="text" value="form1-value" />
		</div>
		<div id="form2">
			<input type="text" value="form2-value" />
		</div>
	`)

	val, err := page.Locator("#form2").Locator("input").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "form2-value" {
		t.Errorf("expected 'form2-value', got %q", val)
	}
}

func TestLocatorIsVisible(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="visible">I am visible</div>
		<div id="hidden" style="display:none">I am hidden</div>
	`)

	vis1, err := page.Locator("#visible").IsVisible()
	if err != nil {
		t.Fatal(err)
	}
	if !vis1 {
		t.Error("expected #visible to be visible")
	}

	vis2, err := page.Locator("#hidden").IsVisible()
	if err != nil {
		t.Fatal(err)
	}
	if vis2 {
		t.Error("expected #hidden to not be visible")
	}
}

func TestLocatorCheckbox(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<input id="agree" type="checkbox" />
	`)

	checked, _ := page.Locator("#agree").IsChecked()
	if checked {
		t.Error("expected checkbox to start unchecked")
	}

	page.Locator("#agree").Check()

	checked, _ = page.Locator("#agree").IsChecked()
	if !checked {
		t.Error("expected checkbox to be checked after Check()")
	}

	page.Locator("#agree").Uncheck()

	checked, _ = page.Locator("#agree").IsChecked()
	if checked {
		t.Error("expected checkbox to be unchecked after Uncheck()")
	}
}

func TestLocatorAutoWaitsForElement(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="container"></div>
		<script>
			setTimeout(function() {
				var btn = document.createElement('button');
				btn.id = 'delayed';
				btn.textContent = 'Delayed Button';
				btn.onclick = function() { document.title = 'auto-waited'; };
				document.getElementById('container').appendChild(btn);
			}, 500);
		</script>
	`)

	// This should auto-wait for the button to appear
	err := page.Locator("#delayed").Click()
	if err != nil {
		t.Fatal(err)
	}

	title, _ := page.Title()
	if title != "auto-waited" {
		t.Errorf("expected title 'auto-waited', got %q", title)
	}
}

func TestLocatorPress(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<input id="field" type="text" onkeydown="if(event.key==='Enter') document.title='enter-pressed'" />
	`)

	page.Locator("#field").Press("Enter")

	title, _ := page.Title()
	if title != "enter-pressed" {
		t.Errorf("expected title 'enter-pressed', got %q", title)
	}
}

func TestLocatorGetAttribute(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<a id="link" href="https://example.com" class="primary">Link</a>
	`)

	href, err := page.Locator("#link").GetAttribute("href")
	if err != nil {
		t.Fatal(err)
	}
	if href != "https://example.com" {
		t.Errorf("expected href 'https://example.com', got %q", href)
	}
}

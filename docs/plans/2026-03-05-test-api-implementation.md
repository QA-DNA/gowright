# Playwright-Style Test API Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add `test.Run`, `test.Describe`, `BeforeEach`/`AfterEach`, auto-fatal wrappers, and unified `Expect` so Go tests read like Playwright.

**Architecture:** Three new files in `package gowright` at the repo root. `test.go` defines `Test`, `Suite`, `TestConfig`, `NewTest()`. `test_context.go` defines `TestContext`, `TestPage`, `TestLocator` — thin wrappers that call `t.Fatal(err)` on error. `test_expect.go` defines `Expect()` which accepts both `TestPage` and `TestLocator` and auto-fatals. All existing `pkg/` code stays untouched.

**Tech Stack:** Go standard library `testing` package, existing `pkg/browser` and `pkg/expect` types.

---

### Task 1: TestConfig and Test entry point (`test.go`)

**Files:**
- Create: `test.go`
- Test: `test_test.go`

**Step 1: Write the failing test**

Create `test_test.go`:

```go
package gowright_test

import (
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestNewTestDefaults(t *testing.T) {
	test := gowright.NewTest()
	if test == nil {
		t.Fatal("NewTest() returned nil")
	}
}

func TestNewTestWithConfig(t *testing.T) {
	test := gowright.NewTest(gowright.TestConfig{
		BaseURL:  "http://localhost:3000",
		Headless: true,
		Timeout:  10 * time.Second,
		Viewport: &gowright.Viewport{Width: 800, Height: 600},
	})
	if test == nil {
		t.Fatal("NewTest() returned nil")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run TestNewTest -v -count=1 ./...`
Expected: FAIL — `NewTest` not defined

**Step 3: Write minimal implementation**

Create `test.go`:

```go
package gowright

import (
	"context"
	"testing"
	"time"

	"github.com/PeterStoica/gowright/pkg/browser/chromium"
)

// TestConfig mirrors playwright.config.ts options.
type TestConfig struct {
	BaseURL  string
	Headless bool
	Timeout  time.Duration
	Viewport *Viewport
	SlowMo   time.Duration
}

// Test is the top-level test runner, analogous to Playwright's test object.
type Test struct {
	config TestConfig
}

// NewTest creates a new Test instance with optional config.
func NewTest(configs ...TestConfig) *Test {
	cfg := TestConfig{
		Headless: true,
		Timeout:  30 * time.Second,
	}
	if len(configs) > 0 {
		c := configs[0]
		cfg.BaseURL = c.BaseURL
		cfg.Headless = c.Headless
		if c.Timeout > 0 {
			cfg.Timeout = c.Timeout
		}
		if c.Viewport != nil {
			cfg.Viewport = c.Viewport
		}
		cfg.SlowMo = c.SlowMo
	}
	return &Test{config: cfg}
}

// Run executes a single test with auto-managed browser fixtures.
func (tt *Test) Run(t *testing.T, name string, fn func(pw *TestContext)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		t.Parallel()
		pw := tt.createContext(t)
		fn(pw)
	})
}

// createContext launches a browser and builds the TestContext.
func (tt *Test) createContext(t *testing.T) *TestContext {
	t.Helper()
	timeout := tt.config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	t.Cleanup(cancel)

	launchOpts := chromium.DefaultLaunchOptions()
	launchOpts.Headless = tt.config.Headless
	if tt.config.SlowMo > 0 {
		launchOpts.SlowMo = tt.config.SlowMo
	}

	b, err := Launch(ctx, launchOpts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	if tt.config.Viewport != nil {
		b.SetDefaultViewport(tt.config.Viewport.Width, tt.config.Viewport.Height)
	}

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if tt.config.BaseURL != "" {
		page.SetDefaultTimeout(timeout)
	}

	return &TestContext{
		t:       t,
		Page:    newTestPage(t, page, tt.config.BaseURL),
		Context: bc,
		Browser: b,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test -run TestNewTest -v -count=1 ./...`
Expected: PASS (note: `TestContext` and `newTestPage` don't exist yet, so this will fail — that's expected, we'll fix in Task 2)

**Step 5: Commit**

```bash
git add test.go test_test.go
git commit -m "feat: add Test entry point and TestConfig"
```

---

### Task 2: TestContext, TestPage, TestLocator auto-fatal wrappers (`test_context.go`)

**Files:**
- Create: `test_context.go`

**Step 1: Write the failing test**

Add to `test_test.go`:

```go
func TestRunSimple(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "navigates to data URL", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>Hello</title><h1>World</h1>")
		title := pw.Page.Title()
		if title != "Hello" {
			t.Errorf("expected title 'Hello', got %q", title)
		}
	})
}

func TestRunLocator(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "locator methods auto-fatal", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<div id='msg'>hello</div>")
		text := pw.Page.Locator("#msg").TextContent()
		if text != "hello" {
			t.Errorf("expected 'hello', got %q", text)
		}
	})
}

func TestRunLocatorClick(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "click changes title", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<button onclick=\"document.title='clicked'\">Click</button>")
		pw.Page.Locator("button").Click()
		title := pw.Page.Title()
		if title != "clicked" {
			t.Errorf("expected 'clicked', got %q", title)
		}
	})
}

func TestRunLocatorFill(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "fill input", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<input id='inp' type='text'/>")
		pw.Page.Locator("#inp").Fill("hello world")
		val := pw.Page.Locator("#inp").InputValue()
		if val != "hello world" {
			t.Errorf("expected 'hello world', got %q", val)
		}
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestRun(Simple|Locator)" -v -count=1 ./...`
Expected: FAIL — `TestContext`, `TestPage`, `TestLocator` not defined

**Step 3: Write minimal implementation**

Create `test_context.go`:

```go
package gowright

import (
	"encoding/json"
	"testing"

	"github.com/PeterStoica/gowright/pkg/browser"
)

// TestContext is the fixture passed to every test function.
// Analogous to Playwright's { page, context, browser } destructured parameter.
type TestContext struct {
	t       *testing.T
	Page    *TestPage
	Context *BrowserContext
	Browser *Browser
}

// TestPage wraps *browser.Page with auto-fatal behavior.
// All methods that would return error instead call t.Fatal on failure.
type TestPage struct {
	t       *testing.T
	raw     *browser.Page
	baseURL string
}

func newTestPage(t *testing.T, page *browser.Page, baseURL string) *TestPage {
	return &TestPage{t: t, raw: page, baseURL: baseURL}
}

// Raw returns the underlying *browser.Page for advanced usage.
func (p *TestPage) Raw() *Page {
	return p.raw
}

func (p *TestPage) Goto(url string) {
	p.t.Helper()
	if p.baseURL != "" && len(url) > 0 && url[0] == '/' {
		url = p.baseURL + url
	}
	if err := p.raw.Goto(url); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) Title() string {
	p.t.Helper()
	title, err := p.raw.Title()
	if err != nil {
		p.t.Fatal(err)
	}
	return title
}

func (p *TestPage) URL() string {
	p.t.Helper()
	return p.raw.URL()
}

func (p *TestPage) Content() string {
	p.t.Helper()
	content, err := p.raw.Content()
	if err != nil {
		p.t.Fatal(err)
	}
	return content
}

func (p *TestPage) SetContent(html string) {
	p.t.Helper()
	if err := p.raw.SetContent(html); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) Reload() {
	p.t.Helper()
	if err := p.raw.Reload(); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) GoBack() {
	p.t.Helper()
	if err := p.raw.GoBack(); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) GoForward() {
	p.t.Helper()
	if err := p.raw.GoForward(); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) Screenshot() []byte {
	p.t.Helper()
	data, err := p.raw.Screenshot()
	if err != nil {
		p.t.Fatal(err)
	}
	return data
}

func (p *TestPage) ScreenshotWithOptions(opts ScreenshotOptions) []byte {
	p.t.Helper()
	data, err := p.raw.ScreenshotWithOptions(opts)
	if err != nil {
		p.t.Fatal(err)
	}
	return data
}

func (p *TestPage) Evaluate(expression string) json.RawMessage {
	p.t.Helper()
	val, err := p.raw.Evaluate(expression)
	if err != nil {
		p.t.Fatal(err)
	}
	return val
}

func (p *TestPage) EvaluateWithArg(expression string, arg any) json.RawMessage {
	p.t.Helper()
	val, err := p.raw.EvaluateWithArg(expression, arg)
	if err != nil {
		p.t.Fatal(err)
	}
	return val
}

func (p *TestPage) SetViewportSize(width, height int) {
	p.t.Helper()
	if err := p.raw.SetViewportSize(width, height); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) WaitForSelector(selector string, opts ...WaitForSelectorOptions) {
	p.t.Helper()
	if err := p.raw.WaitForSelector(selector, opts...); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) WaitForLoadState(state string) {
	p.t.Helper()
	if err := p.raw.WaitForLoadState(state); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) WaitForURL(pattern string, opts ...WaitForURLOptions) {
	p.t.Helper()
	if err := p.raw.WaitForURL(pattern, opts...); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) Close() {
	p.t.Helper()
	if err := p.raw.Close(); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) BringToFront() {
	p.t.Helper()
	if err := p.raw.BringToFront(); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) OnDialog(handler func(*Dialog)) {
	p.raw.OnDialog(handler)
}

func (p *TestPage) On(event string, handler func(any)) {
	p.raw.On(event, handler)
}

func (p *TestPage) Route(pattern string, handler func(*Route, *Request)) {
	p.t.Helper()
	if err := p.raw.Route(pattern, handler); err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestPage) Keyboard() *Keyboard {
	return p.raw.Keyboard()
}

func (p *TestPage) Mouse() *Mouse {
	return p.raw.Mouse()
}

func (p *TestPage) Touchscreen() *Touchscreen {
	return p.raw.Touchscreen()
}

func (p *TestPage) MainFrame() *Frame {
	return p.raw.MainFrame()
}

func (p *TestPage) Frames() []*Frame {
	return p.raw.Frames()
}

// Locator returns a TestLocator with auto-fatal behavior.
func (p *TestPage) Locator(selector string) *TestLocator {
	return newTestLocator(p.t, p.raw.Locator(selector))
}

func (p *TestPage) GetByText(text string, exact ...bool) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByText(text, exact...))
}

func (p *TestPage) GetByRole(role string, opts ...ByRoleOption) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByRole(role, opts...))
}

func (p *TestPage) GetByTestId(testID string) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByTestId(testID))
}

func (p *TestPage) GetByLabel(text string) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByLabel(text))
}

func (p *TestPage) GetByPlaceholder(text string) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByPlaceholder(text))
}

func (p *TestPage) GetByAltText(text string, exact ...bool) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByAltText(text, exact...))
}

func (p *TestPage) GetByTitle(text string, exact ...bool) *TestLocator {
	return newTestLocator(p.t, p.raw.GetByTitle(text, exact...))
}

// --- TestLocator ---

// TestLocator wraps *browser.Locator with auto-fatal behavior.
type TestLocator struct {
	t   *testing.T
	raw *browser.Locator
}

func newTestLocator(t *testing.T, l *browser.Locator) *TestLocator {
	return &TestLocator{t: t, raw: l}
}

// Raw returns the underlying *browser.Locator for advanced usage.
func (l *TestLocator) Raw() *Locator {
	return l.raw
}

func (l *TestLocator) Click() {
	l.t.Helper()
	if err := l.raw.Click(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) DblClick() {
	l.t.Helper()
	if err := l.raw.DblClick(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Fill(value string) {
	l.t.Helper()
	if err := l.raw.Fill(value); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Type(text string) {
	l.t.Helper()
	if err := l.raw.Type(text); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Press(key string) {
	l.t.Helper()
	if err := l.raw.Press(key); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) SelectOption(values ...string) {
	l.t.Helper()
	if err := l.raw.SelectOption(values...); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Check() {
	l.t.Helper()
	if err := l.raw.Check(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Uncheck() {
	l.t.Helper()
	if err := l.raw.Uncheck(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Hover() {
	l.t.Helper()
	if err := l.raw.Hover(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Focus() {
	l.t.Helper()
	if err := l.raw.Focus(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Clear() {
	l.t.Helper()
	if err := l.raw.Clear(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) WaitFor(opts ...WaitForOptions) {
	l.t.Helper()
	if err := l.raw.WaitFor(opts...); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) ScrollIntoViewIfNeeded() {
	l.t.Helper()
	if err := l.raw.ScrollIntoViewIfNeeded(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) Tap() {
	l.t.Helper()
	if err := l.raw.Tap(); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) SetInputFiles(files ...string) {
	l.t.Helper()
	if err := l.raw.SetInputFiles(files...); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) TextContent() string {
	l.t.Helper()
	text, err := l.raw.TextContent()
	if err != nil {
		l.t.Fatal(err)
	}
	return text
}

func (l *TestLocator) InnerText() string {
	l.t.Helper()
	text, err := l.raw.InnerText()
	if err != nil {
		l.t.Fatal(err)
	}
	return text
}

func (l *TestLocator) InnerHTML() string {
	l.t.Helper()
	html, err := l.raw.InnerHTML()
	if err != nil {
		l.t.Fatal(err)
	}
	return html
}

func (l *TestLocator) InputValue() string {
	l.t.Helper()
	val, err := l.raw.InputValue()
	if err != nil {
		l.t.Fatal(err)
	}
	return val
}

func (l *TestLocator) GetAttribute(name string) string {
	l.t.Helper()
	val, err := l.raw.GetAttribute(name)
	if err != nil {
		l.t.Fatal(err)
	}
	return val
}

func (l *TestLocator) IsVisible() bool {
	l.t.Helper()
	v, err := l.raw.IsVisible()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) IsHidden() bool {
	l.t.Helper()
	v, err := l.raw.IsHidden()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) IsEnabled() bool {
	l.t.Helper()
	v, err := l.raw.IsEnabled()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) IsDisabled() bool {
	l.t.Helper()
	v, err := l.raw.IsDisabled()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) IsChecked() bool {
	l.t.Helper()
	v, err := l.raw.IsChecked()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) IsEditable() bool {
	l.t.Helper()
	v, err := l.raw.IsEditable()
	if err != nil {
		l.t.Fatal(err)
	}
	return v
}

func (l *TestLocator) Count() int {
	l.t.Helper()
	n, err := l.raw.Count()
	if err != nil {
		l.t.Fatal(err)
	}
	return n
}

func (l *TestLocator) BoundingBox() *BoundingBox {
	l.t.Helper()
	box, err := l.raw.BoundingBox()
	if err != nil {
		l.t.Fatal(err)
	}
	return box
}

func (l *TestLocator) Screenshot() []byte {
	l.t.Helper()
	data, err := l.raw.Screenshot()
	if err != nil {
		l.t.Fatal(err)
	}
	return data
}

func (l *TestLocator) Evaluate(expression string, arg ...any) json.RawMessage {
	l.t.Helper()
	val, err := l.raw.Evaluate(expression, arg...)
	if err != nil {
		l.t.Fatal(err)
	}
	return val
}

// Chaining methods — return TestLocator
func (l *TestLocator) Locator(selector string) *TestLocator {
	return newTestLocator(l.t, l.raw.Locator(selector))
}

func (l *TestLocator) First() *TestLocator {
	return newTestLocator(l.t, l.raw.First())
}

func (l *TestLocator) Last() *TestLocator {
	return newTestLocator(l.t, l.raw.Last())
}

func (l *TestLocator) Nth(index int) *TestLocator {
	return newTestLocator(l.t, l.raw.Nth(index))
}

func (l *TestLocator) Filter(opts FilterOptions) *TestLocator {
	return newTestLocator(l.t, l.raw.Filter(opts))
}

func (l *TestLocator) And(other *TestLocator) *TestLocator {
	return newTestLocator(l.t, l.raw.And(other.raw))
}

func (l *TestLocator) Or(other *TestLocator) *TestLocator {
	return newTestLocator(l.t, l.raw.Or(other.raw))
}

func (l *TestLocator) GetByText(text string, exact ...bool) *TestLocator {
	return newTestLocator(l.t, l.raw.GetByText(text, exact...))
}

func (l *TestLocator) GetByRole(role string, opts ...ByRoleOption) *TestLocator {
	return newTestLocator(l.t, l.raw.GetByRole(role, opts...))
}

func (l *TestLocator) GetByTestId(testID string) *TestLocator {
	return newTestLocator(l.t, l.raw.GetByTestId(testID))
}

func (l *TestLocator) GetByLabel(text string) *TestLocator {
	return newTestLocator(l.t, l.raw.GetByLabel(text))
}

func (l *TestLocator) GetByPlaceholder(text string) *TestLocator {
	return newTestLocator(l.t, l.raw.GetByPlaceholder(text))
}

func (l *TestLocator) PressSequentially(text string, opts ...PressSequentiallyOptions) {
	l.t.Helper()
	if err := l.raw.PressSequentially(text, opts...); err != nil {
		l.t.Fatal(err)
	}
}

func (l *TestLocator) DragTo(target *TestLocator, opts ...DragToOptions) {
	l.t.Helper()
	if err := l.raw.DragTo(target.raw, opts...); err != nil {
		l.t.Fatal(err)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestRun(Simple|Locator)" -v -count=1 ./...`
Expected: PASS — all 4 tests run, browser launches, navigates, locator works

**Step 5: Commit**

```bash
git add test_context.go test_test.go test.go
git commit -m "feat: add TestContext, TestPage, TestLocator auto-fatal wrappers"
```

---

### Task 3: Expect API (`test_expect.go`)

**Files:**
- Create: `test_expect.go`

**Step 1: Write the failing test**

Add to `test_test.go`:

```go
func TestExpectToHaveTextWrapper(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "expect text", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<h1 id='title'>Hello World</h1>")
		pw.Expect(pw.Page.Locator("#title")).ToHaveText("Hello World")
	})
}

func TestExpectToBeVisibleWrapper(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "expect visible", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<div id='vis'>visible</div>")
		pw.Expect(pw.Page.Locator("#vis")).ToBeVisible()
	})
}

func TestExpectNotToBeVisible(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "expect not visible", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<div id='hid' style='display:none'>hidden</div>")
		pw.Expect(pw.Page.Locator("#hid")).Not().ToBeVisible()
	})
}

func TestExpectPageTitle(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "expect page title", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>My Page</title>")
		pw.Expect(pw.Page).ToHaveTitle("My Page")
	})
}

func TestExpectPageURL(t *testing.T) {
	test := gowright.NewTest()
	test.Run(t, "expect page url", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<div>url test</div>")
		pw.Expect(pw.Page).ToHaveURL("data:text/html")
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestExpect.*(Wrapper|Not|Page)" -v -count=1 ./...`
Expected: FAIL — `Expect` method not defined on `TestContext`

**Step 3: Write minimal implementation**

Create `test_expect.go`:

```go
package gowright

import (
	"testing"
	"time"

	"github.com/PeterStoica/gowright/pkg/expect"
)

// Expectable is an interface for types that can be passed to Expect().
// Satisfied by *TestPage and *TestLocator.
type Expectable interface {
	expectTarget()
}

func (p *TestPage) expectTarget()    {}
func (l *TestLocator) expectTarget() {}

// Expect starts an assertion chain. Accepts *TestPage or *TestLocator.
func (tc *TestContext) Expect(target Expectable) *Expectation {
	return &Expectation{t: tc.t, target: target}
}

// Expectation holds the assertion state.
type Expectation struct {
	t       *testing.T
	target  Expectable
	negated bool
	timeout time.Duration
}

// Not negates the assertion.
func (e *Expectation) Not() *Expectation {
	return &Expectation{t: e.t, target: e.target, negated: true, timeout: e.timeout}
}

// WithTimeout sets a custom timeout for the assertion.
func (e *Expectation) WithTimeout(d time.Duration) *Expectation {
	return &Expectation{t: e.t, target: e.target, negated: e.negated, timeout: d}
}

func (e *Expectation) locatorAssertions() *expect.LocatorAssertions {
	tl, ok := e.target.(*TestLocator)
	if !ok {
		e.t.Fatal("Expect(): target is not a locator")
		return nil
	}
	a := expect.Locator(tl.raw)
	if e.negated {
		a = a.Not()
	}
	if e.timeout > 0 {
		a = a.WithTimeout(e.timeout)
	}
	return a
}

func (e *Expectation) pageAssertions() *expect.PageAssertions {
	tp, ok := e.target.(*TestPage)
	if !ok {
		e.t.Fatal("Expect(): target is not a page")
		return nil
	}
	a := expect.Page(tp.raw)
	if e.negated {
		a = a.Not()
	}
	if e.timeout > 0 {
		a = a.WithTimeout(e.timeout)
	}
	return a
}

// --- Locator assertions ---

func (e *Expectation) ToBeVisible() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeVisible(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeHidden() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeHidden(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeEnabled() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeEnabled(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeDisabled() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeDisabled(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeChecked() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeChecked(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveText(expected string) {
	e.t.Helper()
	switch e.target.(type) {
	case *TestLocator:
		if err := e.locatorAssertions().ToHaveText(expected); err != nil {
			e.t.Fatal(err)
		}
	default:
		e.t.Fatal("ToHaveText requires a locator target")
	}
}

func (e *Expectation) ToContainText(expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToContainText(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveValue(expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveValue(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveAttribute(name, expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveAttribute(name, expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveCount(expected int) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveCount(expected); err != nil {
		e.t.Fatal(err)
	}
}

// --- Page assertions ---

func (e *Expectation) ToHaveTitle(expected string) {
	e.t.Helper()
	if err := e.pageAssertions().ToHaveTitle(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveURL(expected string) {
	e.t.Helper()
	if err := e.pageAssertions().ToHaveURL(expected); err != nil {
		e.t.Fatal(err)
	}
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestExpect" -v -count=1 ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add test_expect.go test_test.go
git commit -m "feat: add Expect API with auto-fatal assertions"
```

---

### Task 4: Suite with Describe, BeforeEach, AfterEach (`test.go` update)

**Files:**
- Modify: `test.go`

**Step 1: Write the failing test**

Add to `test_test.go`:

```go
func TestDescribeWithHooks(t *testing.T) {
	test := gowright.NewTest()
	var hookOrder []string

	test.Describe(t, "suite with hooks", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			hookOrder = append(hookOrder, "before")
			pw.Page.Goto("data:text/html,<title>Hook Page</title><div id='content'>ready</div>")
		})

		s.AfterEach(func(pw *gowright.TestContext) {
			hookOrder = append(hookOrder, "after")
		})

		s.Test("first test", func(pw *gowright.TestContext) {
			hookOrder = append(hookOrder, "test1")
			title := pw.Page.Title()
			if title != "Hook Page" {
				t.Errorf("expected 'Hook Page', got %q", title)
			}
		})

		s.Test("second test", func(pw *gowright.TestContext) {
			hookOrder = append(hookOrder, "test2")
			text := pw.Page.Locator("#content").TextContent()
			if text != "ready" {
				t.Errorf("expected 'ready', got %q", text)
			}
		})
	})
}

func TestDescribeNested(t *testing.T) {
	test := gowright.NewTest()
	test.Describe(t, "outer", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			pw.Page.Goto("data:text/html,<title>Nested</title>")
		})

		s.Describe("inner", func(s *gowright.Suite) {
			s.Test("has title", func(pw *gowright.TestContext) {
				title := pw.Page.Title()
				if title != "Nested" {
					t.Errorf("expected 'Nested', got %q", title)
				}
			})
		})
	})
}
```

**Step 2: Run test to verify it fails**

Run: `go test -run "TestDescribe" -v -count=1 ./...`
Expected: FAIL — `Describe`, `Suite` not defined

**Step 3: Write the implementation**

Add to `test.go`:

```go
// Suite represents a test group created by Describe.
// Provides BeforeEach, AfterEach, Test, and nested Describe.
type Suite struct {
	tt         *Test
	t          *testing.T
	beforeEach []func(*TestContext)
	afterEach  []func(*TestContext)
	parent     *Suite
}

// Describe creates a grouped test suite with optional hooks.
func (tt *Test) Describe(t *testing.T, name string, fn func(s *Suite)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		t.Helper()
		s := &Suite{tt: tt, t: t}
		fn(s)
	})
}

// Describe creates a nested suite, inheriting parent hooks.
func (s *Suite) Describe(name string, fn func(s *Suite)) {
	s.t.Helper()
	s.t.Run(name, func(t *testing.T) {
		t.Helper()
		child := &Suite{tt: s.tt, t: t, parent: s}
		fn(child)
	})
}

// BeforeEach registers a hook that runs before each test in this suite.
func (s *Suite) BeforeEach(fn func(pw *TestContext)) {
	s.beforeEach = append(s.beforeEach, fn)
}

// AfterEach registers a hook that runs after each test in this suite.
func (s *Suite) AfterEach(fn func(pw *TestContext)) {
	s.afterEach = append(s.afterEach, fn)
}

// Test runs a single test within the suite, executing hooks.
func (s *Suite) Test(name string, fn func(pw *TestContext)) {
	s.t.Helper()
	s.t.Run(name, func(t *testing.T) {
		t.Helper()
		t.Parallel()
		pw := s.tt.createContext(t)

		// Collect hooks from parent chain (outermost first)
		befores, afters := s.collectHooks()

		for _, hook := range befores {
			hook(pw)
		}

		// Register afters as cleanup (runs in reverse order automatically)
		for i := len(afters) - 1; i >= 0; i-- {
			after := afters[i]
			t.Cleanup(func() { after(pw) })
		}

		fn(pw)
	})
}

// collectHooks walks the parent chain and returns hooks outermost-first.
func (s *Suite) collectHooks() (befores []func(*TestContext), afters []func(*TestContext)) {
	var chain []*Suite
	for cur := s; cur != nil; cur = cur.parent {
		chain = append(chain, cur)
	}
	// Reverse so outermost is first
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	for _, suite := range chain {
		befores = append(befores, suite.beforeEach...)
		afters = append(afters, suite.afterEach...)
	}
	return
}
```

**Step 4: Run tests to verify they pass**

Run: `go test -run "TestDescribe" -v -count=1 ./...`
Expected: PASS

**Step 5: Commit**

```bash
git add test.go test_test.go
git commit -m "feat: add Suite with Describe, BeforeEach, AfterEach hooks"
```

---

### Task 5: Integration test — full Playwright-style test

**Files:**
- Modify: `test_test.go`

**Step 1: Write a realistic Playwright-style test**

Add to `test_test.go`:

```go
func TestPlaywrightStyle(t *testing.T) {
	test := gowright.NewTest()

	test.Run(t, "simple navigation", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>Example</title><h1>Hello World</h1><p>Content here</p>")
		pw.Expect(pw.Page).ToHaveTitle("Example")
		pw.Expect(pw.Page.Locator("h1")).ToHaveText("Hello World")
		pw.Expect(pw.Page.Locator("p")).ToContainText("Content")
	})

	test.Describe(t, "form interactions", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			pw.Page.Goto(`data:text/html,
				<form>
					<input id="name" type="text" placeholder="Name"/>
					<input id="email" type="email" placeholder="Email"/>
					<input id="agree" type="checkbox"/>
					<button type="submit" onclick="event.preventDefault();document.title='submitted'">Submit</button>
				</form>`)
		})

		s.Test("fill and submit", func(pw *gowright.TestContext) {
			pw.Page.Locator("#name").Fill("John Doe")
			pw.Page.Locator("#email").Fill("john@example.com")
			pw.Page.Locator("#agree").Check()
			pw.Page.Locator("button").Click()

			pw.Expect(pw.Page.Locator("#name")).ToHaveValue("John Doe")
			pw.Expect(pw.Page.Locator("#agree")).ToBeChecked()
			pw.Expect(pw.Page).ToHaveTitle("submitted")
		})

		s.Test("clear input", func(pw *gowright.TestContext) {
			pw.Page.Locator("#name").Fill("temp")
			pw.Page.Locator("#name").Clear()
			pw.Expect(pw.Page.Locator("#name")).ToHaveValue("")
		})
	})
}
```

**Step 2: Run all new tests**

Run: `go test -run "TestPlaywrightStyle" -v -count=1 ./...`
Expected: PASS

**Step 3: Run the full test suite to verify nothing is broken**

Run: `go test -v -count=1 -timeout=120s ./...`
Expected: All existing tests still pass, new tests pass

**Step 4: Commit**

```bash
git add test_test.go
git commit -m "test: add Playwright-style integration tests"
```

---

### Task 6: Update go.mod module path and push

**Step 1: Check if module path needs updating**

The `go.mod` currently says `github.com/PeterStoica/gowright`. The repo is now at `github.com/QA-DNA/gowright`. This needs to stay as-is for now since all internal imports reference it, OR update everything. Ask the user which they prefer before proceeding.

**Step 2: Push to remote**

```bash
git push origin main
```

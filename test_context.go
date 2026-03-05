package gowright

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/QA-DNA/gowright/pkg/browser"
)

// TestContext is the fixture passed to test functions.
type TestContext struct {
	t       *testing.T
	Page    *TestPage
	Context *BrowserContext
	Browser *Browser
	cancel  context.CancelFunc
	ctx     context.Context
}

func (tc *TestContext) cleanup() {
	tc.Page.raw.Close()
	tc.Context.Close()
	tc.Browser.Close()
	tc.cancel()
}

// ---------------------------------------------------------------------------
// TestPage — wraps *browser.Page, auto-fatals on errors
// ---------------------------------------------------------------------------

// TestPage wraps a Page so that error-returning methods become void methods
// that call t.Fatal on error.
type TestPage struct {
	t       *testing.T
	raw     *browser.Page
	baseURL string
}

func newTestPage(t *testing.T, p *browser.Page, baseURL string) *TestPage {
	return &TestPage{t: t, raw: p, baseURL: baseURL}
}

// expectTarget marks TestPage as satisfying the Expectable interface.
func (tp *TestPage) expectTarget() {}

// Raw returns the underlying *Page for advanced usage.
func (tp *TestPage) Raw() *Page { return tp.raw }

// resolveURL prepends the baseURL if the url starts with "/".
func (tp *TestPage) resolveURL(url string) string {
	if tp.baseURL != "" && strings.HasPrefix(url, "/") {
		return tp.baseURL + url
	}
	return url
}

func (tp *TestPage) Goto(url string) {
	tp.t.Helper()
	if err := tp.raw.Goto(tp.resolveURL(url)); err != nil {
		tp.t.Fatalf("Page.Goto: %v", err)
	}
}

func (tp *TestPage) Title() string {
	tp.t.Helper()
	v, err := tp.raw.Title()
	if err != nil {
		tp.t.Fatalf("Page.Title: %v", err)
	}
	return v
}

func (tp *TestPage) URL() string {
	return tp.raw.URL()
}

func (tp *TestPage) Content() string {
	tp.t.Helper()
	v, err := tp.raw.Content()
	if err != nil {
		tp.t.Fatalf("Page.Content: %v", err)
	}
	return v
}

func (tp *TestPage) SetContent(html string) {
	tp.t.Helper()
	if err := tp.raw.SetContent(html); err != nil {
		tp.t.Fatalf("Page.SetContent: %v", err)
	}
}

func (tp *TestPage) Reload() {
	tp.t.Helper()
	if err := tp.raw.Reload(); err != nil {
		tp.t.Fatalf("Page.Reload: %v", err)
	}
}

func (tp *TestPage) GoBack() {
	tp.t.Helper()
	if err := tp.raw.GoBack(); err != nil {
		tp.t.Fatalf("Page.GoBack: %v", err)
	}
}

func (tp *TestPage) GoForward() {
	tp.t.Helper()
	if err := tp.raw.GoForward(); err != nil {
		tp.t.Fatalf("Page.GoForward: %v", err)
	}
}

func (tp *TestPage) Screenshot() []byte {
	tp.t.Helper()
	v, err := tp.raw.Screenshot()
	if err != nil {
		tp.t.Fatalf("Page.Screenshot: %v", err)
	}
	return v
}

func (tp *TestPage) ScreenshotWithOptions(opts ScreenshotOptions) []byte {
	tp.t.Helper()
	v, err := tp.raw.ScreenshotWithOptions(opts)
	if err != nil {
		tp.t.Fatalf("Page.ScreenshotWithOptions: %v", err)
	}
	return v
}

func (tp *TestPage) Evaluate(expr string) json.RawMessage {
	tp.t.Helper()
	v, err := tp.raw.Evaluate(expr)
	if err != nil {
		tp.t.Fatalf("Page.Evaluate: %v", err)
	}
	return v
}

func (tp *TestPage) EvaluateWithArg(expr string, arg interface{}) json.RawMessage {
	tp.t.Helper()
	v, err := tp.raw.EvaluateWithArg(expr, arg)
	if err != nil {
		tp.t.Fatalf("Page.EvaluateWithArg: %v", err)
	}
	return v
}

func (tp *TestPage) SetViewportSize(w, h int) {
	tp.t.Helper()
	if err := tp.raw.SetViewportSize(w, h); err != nil {
		tp.t.Fatalf("Page.SetViewportSize: %v", err)
	}
}

func (tp *TestPage) WaitForSelector(sel string, opts ...WaitForSelectorOptions) {
	tp.t.Helper()
	if err := tp.raw.WaitForSelector(sel, opts...); err != nil {
		tp.t.Fatalf("Page.WaitForSelector: %v", err)
	}
}

func (tp *TestPage) WaitForLoadState(state string) {
	tp.t.Helper()
	if err := tp.raw.WaitForLoadState(state); err != nil {
		tp.t.Fatalf("Page.WaitForLoadState: %v", err)
	}
}

func (tp *TestPage) WaitForURL(pattern string, opts ...WaitForURLOptions) {
	tp.t.Helper()
	if err := tp.raw.WaitForURL(pattern, opts...); err != nil {
		tp.t.Fatalf("Page.WaitForURL: %v", err)
	}
}

func (tp *TestPage) Close() {
	tp.t.Helper()
	if err := tp.raw.Close(); err != nil {
		tp.t.Fatalf("Page.Close: %v", err)
	}
}

func (tp *TestPage) BringToFront() {
	tp.t.Helper()
	if err := tp.raw.BringToFront(); err != nil {
		tp.t.Fatalf("Page.BringToFront: %v", err)
	}
}

func (tp *TestPage) OnDialog(handler func(*Dialog)) {
	tp.raw.OnDialog(handler)
}

func (tp *TestPage) On(event string, handler func(any)) {
	tp.raw.On(event, handler)
}

func (tp *TestPage) Route(pattern string, handler func(*Route, *Request)) {
	tp.t.Helper()
	if err := tp.raw.Route(pattern, handler); err != nil {
		tp.t.Fatalf("Page.Route: %v", err)
	}
}

func (tp *TestPage) Click(sel string, opts ...ClickOptions) {
	tp.t.Helper()
	if err := tp.raw.Click(sel, opts...); err != nil {
		tp.t.Fatalf("Page.Click: %v", err)
	}
}

func (tp *TestPage) Fill(sel string, val string, opts ...FillOptions) {
	tp.t.Helper()
	if err := tp.raw.Fill(sel, val, opts...); err != nil {
		tp.t.Fatalf("Page.Fill: %v", err)
	}
}

func (tp *TestPage) Keyboard() *Keyboard       { return tp.raw.Keyboard() }
func (tp *TestPage) Mouse() *Mouse             { return tp.raw.Mouse() }
func (tp *TestPage) Touchscreen() *Touchscreen { return tp.raw.Touchscreen() }
func (tp *TestPage) MainFrame() *Frame         { return tp.raw.MainFrame() }
func (tp *TestPage) Frames() []*Frame          { return tp.raw.Frames() }

// Locator returns a TestLocator for the given CSS selector.
func (tp *TestPage) Locator(sel string) *TestLocator {
	return newTestLocator(tp.t, tp.raw.Locator(sel))
}

func (tp *TestPage) GetByText(text string, exact ...bool) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByText(text, exact...))
}

func (tp *TestPage) GetByRole(role string, opts ...ByRoleOption) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByRole(role, opts...))
}

func (tp *TestPage) GetByTestId(id string) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByTestId(id))
}

func (tp *TestPage) GetByLabel(text string) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByLabel(text))
}

func (tp *TestPage) GetByPlaceholder(text string) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByPlaceholder(text))
}

func (tp *TestPage) GetByAltText(text string, exact ...bool) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByAltText(text, exact...))
}

func (tp *TestPage) GetByTitle(text string, exact ...bool) *TestLocator {
	return newTestLocator(tp.t, tp.raw.GetByTitle(text, exact...))
}

// ---------------------------------------------------------------------------
// TestLocator — wraps *browser.Locator, auto-fatals on errors
// ---------------------------------------------------------------------------

// TestLocator wraps a Locator so that error-returning methods become void
// methods that call t.Fatal on error.
type TestLocator struct {
	t   *testing.T
	raw *browser.Locator
}

func newTestLocator(t *testing.T, l *browser.Locator) *TestLocator {
	return &TestLocator{t: t, raw: l}
}

// expectTarget marks TestLocator as satisfying the Expectable interface.
func (tl *TestLocator) expectTarget() {}

// Raw returns the underlying *Locator for advanced usage.
func (tl *TestLocator) Raw() *Locator { return tl.raw }

// --- Action methods ---

func (tl *TestLocator) Click() {
	tl.t.Helper()
	if err := tl.raw.Click(); err != nil {
		tl.t.Fatalf("Locator.Click: %v", err)
	}
}

func (tl *TestLocator) DblClick() {
	tl.t.Helper()
	if err := tl.raw.DblClick(); err != nil {
		tl.t.Fatalf("Locator.DblClick: %v", err)
	}
}

func (tl *TestLocator) Fill(val string) {
	tl.t.Helper()
	if err := tl.raw.Fill(val); err != nil {
		tl.t.Fatalf("Locator.Fill: %v", err)
	}
}

func (tl *TestLocator) Type(text string) {
	tl.t.Helper()
	if err := tl.raw.Type(text); err != nil {
		tl.t.Fatalf("Locator.Type: %v", err)
	}
}

func (tl *TestLocator) Press(key string) {
	tl.t.Helper()
	if err := tl.raw.Press(key); err != nil {
		tl.t.Fatalf("Locator.Press: %v", err)
	}
}

func (tl *TestLocator) SelectOption(vals ...string) {
	tl.t.Helper()
	if err := tl.raw.SelectOption(vals...); err != nil {
		tl.t.Fatalf("Locator.SelectOption: %v", err)
	}
}

func (tl *TestLocator) Check() {
	tl.t.Helper()
	if err := tl.raw.Check(); err != nil {
		tl.t.Fatalf("Locator.Check: %v", err)
	}
}

func (tl *TestLocator) Uncheck() {
	tl.t.Helper()
	if err := tl.raw.Uncheck(); err != nil {
		tl.t.Fatalf("Locator.Uncheck: %v", err)
	}
}

func (tl *TestLocator) Hover() {
	tl.t.Helper()
	if err := tl.raw.Hover(); err != nil {
		tl.t.Fatalf("Locator.Hover: %v", err)
	}
}

func (tl *TestLocator) Focus() {
	tl.t.Helper()
	if err := tl.raw.Focus(); err != nil {
		tl.t.Fatalf("Locator.Focus: %v", err)
	}
}

func (tl *TestLocator) Clear() {
	tl.t.Helper()
	if err := tl.raw.Clear(); err != nil {
		tl.t.Fatalf("Locator.Clear: %v", err)
	}
}

func (tl *TestLocator) WaitFor(opts ...WaitForOptions) {
	tl.t.Helper()
	if err := tl.raw.WaitFor(opts...); err != nil {
		tl.t.Fatalf("Locator.WaitFor: %v", err)
	}
}

func (tl *TestLocator) ScrollIntoViewIfNeeded() {
	tl.t.Helper()
	if err := tl.raw.ScrollIntoViewIfNeeded(); err != nil {
		tl.t.Fatalf("Locator.ScrollIntoViewIfNeeded: %v", err)
	}
}

func (tl *TestLocator) Tap() {
	tl.t.Helper()
	if err := tl.raw.Tap(); err != nil {
		tl.t.Fatalf("Locator.Tap: %v", err)
	}
}

func (tl *TestLocator) SetInputFiles(files ...string) {
	tl.t.Helper()
	if err := tl.raw.SetInputFiles(files...); err != nil {
		tl.t.Fatalf("Locator.SetInputFiles: %v", err)
	}
}

func (tl *TestLocator) PressSequentially(text string, opts ...PressSequentiallyOptions) {
	tl.t.Helper()
	if err := tl.raw.PressSequentially(text, opts...); err != nil {
		tl.t.Fatalf("Locator.PressSequentially: %v", err)
	}
}

func (tl *TestLocator) DragTo(target *TestLocator, opts ...DragToOptions) {
	tl.t.Helper()
	if err := tl.raw.DragTo(target.raw, opts...); err != nil {
		tl.t.Fatalf("Locator.DragTo: %v", err)
	}
}

// --- Query methods ---

func (tl *TestLocator) TextContent() string {
	tl.t.Helper()
	v, err := tl.raw.TextContent()
	if err != nil {
		tl.t.Fatalf("Locator.TextContent: %v", err)
	}
	return v
}

func (tl *TestLocator) InnerText() string {
	tl.t.Helper()
	v, err := tl.raw.InnerText()
	if err != nil {
		tl.t.Fatalf("Locator.InnerText: %v", err)
	}
	return v
}

func (tl *TestLocator) InnerHTML() string {
	tl.t.Helper()
	v, err := tl.raw.InnerHTML()
	if err != nil {
		tl.t.Fatalf("Locator.InnerHTML: %v", err)
	}
	return v
}

func (tl *TestLocator) InputValue() string {
	tl.t.Helper()
	v, err := tl.raw.InputValue()
	if err != nil {
		tl.t.Fatalf("Locator.InputValue: %v", err)
	}
	return v
}

func (tl *TestLocator) GetAttribute(name string) string {
	tl.t.Helper()
	v, err := tl.raw.GetAttribute(name)
	if err != nil {
		tl.t.Fatalf("Locator.GetAttribute: %v", err)
	}
	return v
}

func (tl *TestLocator) IsVisible() bool {
	tl.t.Helper()
	v, err := tl.raw.IsVisible()
	if err != nil {
		tl.t.Fatalf("Locator.IsVisible: %v", err)
	}
	return v
}

func (tl *TestLocator) IsHidden() bool {
	tl.t.Helper()
	v, err := tl.raw.IsHidden()
	if err != nil {
		tl.t.Fatalf("Locator.IsHidden: %v", err)
	}
	return v
}

func (tl *TestLocator) IsEnabled() bool {
	tl.t.Helper()
	v, err := tl.raw.IsEnabled()
	if err != nil {
		tl.t.Fatalf("Locator.IsEnabled: %v", err)
	}
	return v
}

func (tl *TestLocator) IsDisabled() bool {
	tl.t.Helper()
	v, err := tl.raw.IsDisabled()
	if err != nil {
		tl.t.Fatalf("Locator.IsDisabled: %v", err)
	}
	return v
}

func (tl *TestLocator) IsChecked() bool {
	tl.t.Helper()
	v, err := tl.raw.IsChecked()
	if err != nil {
		tl.t.Fatalf("Locator.IsChecked: %v", err)
	}
	return v
}

func (tl *TestLocator) IsEditable() bool {
	tl.t.Helper()
	v, err := tl.raw.IsEditable()
	if err != nil {
		tl.t.Fatalf("Locator.IsEditable: %v", err)
	}
	return v
}

func (tl *TestLocator) Count() int {
	tl.t.Helper()
	v, err := tl.raw.Count()
	if err != nil {
		tl.t.Fatalf("Locator.Count: %v", err)
	}
	return v
}

func (tl *TestLocator) BoundingBox() *BoundingBox {
	tl.t.Helper()
	v, err := tl.raw.BoundingBox()
	if err != nil {
		tl.t.Fatalf("Locator.BoundingBox: %v", err)
	}
	return v
}

func (tl *TestLocator) Screenshot() []byte {
	tl.t.Helper()
	v, err := tl.raw.Screenshot()
	if err != nil {
		tl.t.Fatalf("Locator.Screenshot: %v", err)
	}
	return v
}

func (tl *TestLocator) Evaluate(expr string, arg ...interface{}) json.RawMessage {
	tl.t.Helper()
	v, err := tl.raw.Evaluate(expr, arg...)
	if err != nil {
		tl.t.Fatalf("Locator.Evaluate: %v", err)
	}
	return v
}

// --- Chaining methods ---

func (tl *TestLocator) Locator(sel string) *TestLocator {
	return newTestLocator(tl.t, tl.raw.Locator(sel))
}

func (tl *TestLocator) First() *TestLocator {
	return newTestLocator(tl.t, tl.raw.First())
}

func (tl *TestLocator) Last() *TestLocator {
	return newTestLocator(tl.t, tl.raw.Last())
}

func (tl *TestLocator) Nth(i int) *TestLocator {
	return newTestLocator(tl.t, tl.raw.Nth(i))
}

func (tl *TestLocator) Filter(opts FilterOptions) *TestLocator {
	return newTestLocator(tl.t, tl.raw.Filter(opts))
}

func (tl *TestLocator) And(other *TestLocator) *TestLocator {
	return newTestLocator(tl.t, tl.raw.And(other.raw))
}

func (tl *TestLocator) Or(other *TestLocator) *TestLocator {
	return newTestLocator(tl.t, tl.raw.Or(other.raw))
}

func (tl *TestLocator) GetByText(text string, exact ...bool) *TestLocator {
	return newTestLocator(tl.t, tl.raw.GetByText(text, exact...))
}

func (tl *TestLocator) GetByRole(role string, opts ...ByRoleOption) *TestLocator {
	return newTestLocator(tl.t, tl.raw.GetByRole(role, opts...))
}

func (tl *TestLocator) GetByTestId(id string) *TestLocator {
	return newTestLocator(tl.t, tl.raw.GetByTestId(id))
}

func (tl *TestLocator) GetByLabel(text string) *TestLocator {
	return newTestLocator(tl.t, tl.raw.GetByLabel(text))
}

func (tl *TestLocator) GetByPlaceholder(text string) *TestLocator {
	return newTestLocator(tl.t, tl.raw.GetByPlaceholder(text))
}

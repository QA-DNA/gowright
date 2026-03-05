# Gowright Test API Design

**Date:** 2026-03-05
**Goal:** Make Go tests read like Playwright tests — no error handling, fixture auto-management, `expect` built-in, `describe`/`beforeEach`/`afterEach` hooks.

## API Surface

```go
import "github.com/QA-DNA/gowright"

var test = gowright.NewTest()
```

### `test.Run` — single test

```go
func TestHomepage(t *testing.T) {
    test.Run(t, "has title", func(pw *gowright.TestContext) {
        pw.Page.Goto("https://example.com")
        pw.Expect(pw.Page.Locator("h1")).ToHaveText("Example Domain")
    })
}
```

### `test.Describe` — grouped tests with hooks

```go
func TestLogin(t *testing.T) {
    test.Describe(t, "login flow", func(s *gowright.Suite) {
        s.BeforeEach(func(pw *gowright.TestContext) {
            pw.Page.Goto("/login")
        })

        s.AfterEach(func(pw *gowright.TestContext) {
            pw.Page.Screenshot()
        })

        s.Test("valid credentials", func(pw *gowright.TestContext) {
            pw.Page.Locator("#user").Fill("admin")
            pw.Page.Locator("#pass").Fill("secret")
            pw.Page.Locator("button").Click()
            pw.Expect(pw.Page).ToHaveURL("/dashboard")
        })

        s.Test("invalid credentials", func(pw *gowright.TestContext) {
            pw.Page.Locator("#user").Fill("wrong")
            pw.Page.Locator("button").Click()
            pw.Expect(pw.Page.Locator(".error")).ToBeVisible()
        })
    })
}
```

### `test.Configure` — global options

```go
var test = gowright.NewTest(gowright.TestConfig{
    BaseURL:  "http://localhost:3000",
    Headless: true,
    Timeout:  30 * time.Second,
    Viewport: &gowright.Viewport{Width: 1280, Height: 720},
})
```

## Core Types

| Type | Role |
|------|------|
| `Test` | Entry point. Holds config. Has `Run`, `Describe`. |
| `Suite` | Scoped group from `Describe`. Has `Test`, `BeforeEach`, `AfterEach`, nested `Describe`. |
| `TestContext` | Passed to every test/hook fn. Has `Page`, `Context`, `Browser`, `Expect`. |
| `Expect` | Returned by `pw.Expect()`. Assertions: `ToBeVisible`, `ToHaveText`, `ToHaveURL`, etc. |

## Auto-Fatal Behavior

All actions on `TestContext` (and objects accessed through it) call `t.Fatal()` on error instead of returning errors.

Implemented via wrapper types:

- `TestContext.Page` is a `TestPage` (wraps `*browser.Page`, holds `*testing.T`)
- `TestPage.Locator()` returns a `TestLocator` (wraps `*browser.Locator`, holds `*testing.T`)
- Every method on these wrappers calls the underlying method, checks the error, and calls `t.Fatal(err)` if non-nil

The raw error-returning API (`*browser.Page`, `*browser.Locator`) remains unchanged for power users.

## Lifecycle per test

```
test.Run(t, "name", fn)
  └─ t.Run("name", func(t) {
       1. Launch browser (or reuse per config)
       2. Create BrowserContext
       3. Create Page
       4. Build TestContext{Page, Context, Browser, t}
       5. Run BeforeEach hooks (if in Suite)
       6. Run fn(pw)
       7. Run AfterEach hooks (if in Suite)
       8. t.Cleanup: close Page, Context, Browser
     })
```

## Expect API

```go
pw.Expect(pw.Page.Locator("h1")).ToHaveText("Example Domain")
pw.Expect(pw.Page.Locator("#btn")).ToBeVisible()
pw.Expect(pw.Page.Locator("#btn")).Not().ToBeVisible()
pw.Expect(pw.Page.Locator("#btn")).ToBeEnabled()
pw.Expect(pw.Page.Locator("#btn")).ToBeChecked()
pw.Expect(pw.Page.Locator("#inp")).ToHaveValue("hello")
pw.Expect(pw.Page.Locator("li")).ToHaveCount(3)
pw.Expect(pw.Page).ToHaveTitle("My Page")
pw.Expect(pw.Page).ToHaveURL("/dashboard")
```

All assertions auto-retry with timeout (default 5s), then `t.Fatal` on failure.

## File Structure

```
gowright/
├── test.go          # Test, Suite, TestConfig, NewTest()
├── test_context.go  # TestContext, TestPage, TestLocator (auto-fatal wrappers)
├── test_expect.go   # Expect() on TestContext, unified expect for page + locator
```

## What Doesn't Change

- `pkg/browser`, `pkg/cdp`, `pkg/expect`, `pkg/runner` — untouched
- The raw `*browser.Page`, `*browser.Locator` APIs keep returning errors
- Existing tests keep working

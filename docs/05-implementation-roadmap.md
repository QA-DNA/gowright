# Gowright Implementation Roadmap

## Phase 1: CDP Foundation (Week 1-2)

### 1.1 WebSocket CDP Client
- Connect to Chrome's debugging WebSocket
- Send JSON messages, receive responses
- Match responses to requests by ID (atomic counter + callback map)
- Handle events (messages without ID)
- Context-aware cancellation

**Key decisions:**
- Use `nhooyr.io/websocket` (modern, context-aware) or `gorilla/websocket` (battle-tested)
- Single goroutine for message consumption (Rod pattern)
- sync.Map for pending callbacks

### 1.2 Session Multiplexing
- Root session for browser-level commands
- Child sessions per page (via Target.attachedToTarget)
- Route messages by sessionId field

### 1.3 Chrome Launcher
- Find Chrome binary (platform-specific paths)
- Launch with correct flags (headless, no-sandbox, etc.)
- Parse WebSocket URL from stderr
- Process lifecycle management (cleanup on exit)
- Optional: auto-download Chromium

### 1.4 Basic Object Hierarchy
```go
type Browser struct { ... }
func (b *Browser) NewContext(opts ...ContextOption) (*BrowserContext, error)
func (b *Browser) Close() error

type BrowserContext struct { ... }
func (c *BrowserContext) NewPage() (*Page, error)
func (c *BrowserContext) Close() error

type Page struct { ... }
func (p *Page) Goto(url string) error
func (p *Page) Close() error
```

### Milestone: Can launch Chrome, navigate to a URL, close it.

---

## Phase 2: Page Operations (Week 3-4)

### 2.1 Navigation
- `page.Goto(url)` with wait options (load, domcontentloaded, networkidle)
- `page.Reload()`, `page.GoBack()`, `page.GoForward()`
- Wait for navigation events from CDP

### 2.2 JavaScript Evaluation
- `page.Evaluate(expression)` → returns Go value
- `page.EvaluateHandle(expression)` → returns JSHandle
- Handle serialization/deserialization (RemoteObject → Go types)
- Execution context management (main world vs utility world)

### 2.3 Input
- Mouse: move, click, dblclick, wheel
- Keyboard: press, type, insertText
- Touch: tap
- Map to CDP Input.dispatch* commands

### 2.4 Screenshots & PDF
- `page.Screenshot()` → []byte (PNG)
- `page.PDF()` → []byte
- Element-level screenshots

### Milestone: Can navigate, click buttons, fill forms, take screenshots.

---

## Phase 3: Locators & Auto-waiting (Week 5-6)

### 3.1 Selector Engines
- CSS selectors (via DOM.querySelector)
- XPath (via Runtime.evaluate)
- Text selectors (`text=Submit`)
- Role selectors (`role=button[name="Submit"]`)
- TestID selectors (`data-testid=login-btn`)

### 3.2 Locator API
```go
loc := page.Locator("button.submit")
loc = page.GetByRole("button", LocatorOptions{Name: "Submit"})
loc = page.GetByTestId("login-btn")
loc = page.GetByText("Click me")

// Chaining
loc = page.Locator("form").Locator("button")

// Actions (auto-wait built in)
loc.Click()
loc.Fill("hello")
loc.Press("Enter")
```

### 3.3 Auto-Wait Implementation
Before every action:
1. Wait for element to be attached to DOM
2. Wait for element to be visible
3. Wait for element to be stable (not animating)
4. Wait for element to receive events (not obscured)
5. Wait for element to be enabled

Use configurable sleeper (backoff with jitter, like Rod).

### 3.4 Assertions (expect)
```go
expect.Locator(loc).ToBeVisible()
expect.Locator(loc).ToHaveText("Hello")
expect.Locator(loc).ToHaveAttribute("class", "active")
expect.Page(page).ToHaveURL("https://example.com/dashboard")
```

### Milestone: Full locator-based interaction with auto-waiting.

---

## Phase 4: Network (Week 7-8)

### 4.1 Request/Response Events
- Listen to Network.requestWillBeSent, Network.responseReceived
- Expose Request and Response objects
- `page.OnRequest(func(req *Request) { ... })`
- `page.OnResponse(func(res *Response) { ... })`

### 4.2 Route Interception
```go
page.Route("**/api/**", func(route *Route) {
    route.Continue()  // or route.Fulfill() or route.Abort()
})
```

### 4.3 Cookie Management
- `context.AddCookies(cookies)`
- `context.Cookies(urls)`
- `context.ClearCookies()`

### 4.4 API Request Context
```go
req := context.NewRequest()
resp, _ := req.Get("https://api.example.com/data")
resp, _ := req.Post("https://api.example.com/data", PostOptions{Data: payload})
```

### Milestone: Can intercept network, mock APIs, manage cookies.

---

## Phase 5: Test Runner (Week 9-12)

### 5.1 Test Discovery
- Scan Go test files for gowright test functions
- Or: embed JS runtime, discover JS/TS test files

### 5.2 Goroutine-Based Parallelism
```go
// Instead of Playwright's OS-process-per-worker:
// - Shared browser connection pool
// - Each test runs in a goroutine
// - Browser contexts provide isolation
// - Configurable concurrency limit

runner := gowright.NewRunner(RunnerOptions{
    Workers:    runtime.NumCPU(),
    Retries:    2,
    Timeout:    30 * time.Second,
})
```

### 5.3 Fixture System
```go
// Automatic browser/context/page lifecycle per test
func TestLogin(t *testing.T) {
    gw := gowright.New(t)
    page := gw.NewPage()
    defer gw.Cleanup()

    page.Goto("https://example.com/login")
    page.GetByLabel("Email").Fill("user@test.com")
    page.GetByLabel("Password").Fill("secret")
    page.GetByRole("button", Name("Sign in")).Click()

    expect.Page(page).ToHaveURL("/dashboard")
}
```

### 5.4 Reporters
- Console (default, with colors)
- JSON
- JUnit XML
- HTML report

### Milestone: Full test framework with parallel execution.

---

## Phase 6: Polish & Advanced Features (Week 13+)

### 6.1 Tracing
- Record actions, screenshots, network for debugging
- Trace viewer (web UI or CLI)

### 6.2 Video Recording
- CDP-based screen recording
- Attach to test results on failure

### 6.3 Codegen
- Record user interactions → generate Go test code

### 6.4 CI Integration
- GitHub Actions helpers
- Docker image
- Sharding support

### 6.5 JS Test Authoring (Optional)
- Embed Goja (pure Go JS engine) or V8
- Write tests in JS/TS, execute with Go runner
- k6-style architecture

---

## Dependencies

| Package | Purpose |
|---------|---------|
| `nhooyr.io/websocket` | WebSocket client for CDP |
| `encoding/json` | CDP message serialization |
| `os/exec` | Chrome process management |
| `context` | Cancellation, timeouts |
| `sync` | Concurrency primitives |
| `testing` | Go test integration |

## Naming

- Module: `github.com/PeterStoica/gowright`
- Binary: `gowright`
- Test API: `gowright.New(t)`, `page.Locator()`, `expect.Locator()`

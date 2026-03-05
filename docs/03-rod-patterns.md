# Rod Architecture Patterns (Reference for Gowright)

Rod is the best existing Go browser automation library. These patterns inform our design.

## CDP Client (lib/cdp/client.go)

### Request-Response Matching
```go
// Atomic counter for unique request IDs
type Client struct {
    pending sync.Map  // map[int]func(result)
    count   int64     // atomic counter
}

func (c *Client) Call(ctx context.Context, sessionID, method string, params json.RawMessage) (json.RawMessage, error) {
    id := atomic.AddInt64(&c.count, 1)

    done := make(chan result)
    once := sync.Once{}
    c.pending.Store(id, func(res result) {
        once.Do(func() {
            select {
            case <-ctx.Done():
            case done <- res:
            }
        })
    })

    // Send request
    c.ws.Send(marshal(Request{ID: id, SessionID: sessionID, Method: method, Params: params}))

    // Wait for response
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case res := <-done:
        return res.data, res.err
    }
}
```

### Message Routing (consumeMessages goroutine)
```go
// Single goroutine reads all messages, routes by ID or event
func (c *Client) consumeMessages() {
    for {
        msg := c.ws.Read()
        parsed := parse(msg)

        if parsed.ID > 0 {
            // Response - find and call pending callback
            if cb, ok := c.pending.LoadAndDelete(parsed.ID); ok {
                cb.(func(result))(result{data: parsed.Result, err: parsed.Error})
            }
        } else {
            // Event - publish to event channel
            c.eventChan <- parsed
        }
    }
}
```

### Key Insight
Rod uses a single goroutine for message consumption (no lock contention on reads) and sync.Map for pending callbacks (lock-free reads for the common path).

## WebSocket (lib/cdp/websocket.go)

- Custom minimal WebSocket implementation (RFC 6455 subset)
- Text frames only, no compression
- In-place XOR masking for client→server frames
- Mutex-protected Read/Write
- ~239 lines total

**For Gowright:** Consider using `github.com/gorilla/websocket` or `nhooyr.io/websocket` instead of custom implementation. Performance difference is negligible for CDP message volumes.

## Browser Launch (lib/launcher/)

### Binary Discovery
```
LookPath() sequence:
  1. macOS: /Applications/Google Chrome.app, Chromium.app
  2. Linux: chrome, google-chrome, chromium, chromium-browser
  3. Windows: APPDATA, ProgramFiles locations

Validate() → Run: chrome --headless --dump-dom about:blank
  Expected: <html><head></head><body></body></html>
```

### Auto-Download
- Downloads specific Chromium revision to `$HOME/.cache/rod/browser/chromium-{revision}/`
- Tries multiple hosts in parallel (Google, NPM, Playwright CDN)
- Race-based host selection

### Launch Flow
```
1. Get/download binary
2. Create temp user-data-dir
3. Format CLI args from flag map
4. Start process (leakless wrapper or direct exec)
5. Parse stderr for "DevTools listening on ws://..."
6. Return WebSocket URL
```

## Object Hierarchy

```go
// Browser - top level, one per Chrome process
type Browser struct {
    ctx       context.Context
    cdp       *cdp.Client        // WebSocket connection
    event     goob.Observable    // pub/sub for CDP events
    states    sync.Map           // cached browser state
}

// Page - one per tab
type Page struct {
    ctx       context.Context
    browser   *Browser
    targetID  string
    sessionID string
    frameID   string
    mouse     *Mouse
    keyboard  *Keyboard
    touch     *Touch
}

// Element - a DOM node reference
type Element struct {
    ctx    context.Context
    page   *Page
    object *proto.RuntimeRemoteObject  // JS handle
}
```

## Context Chaining Pattern

Rod uses struct cloning for immutable context propagation:

```go
func (b *Browser) Context(ctx context.Context) *Browser {
    newObj := *b  // shallow copy
    newObj.ctx = ctx
    return &newObj
}

func (b *Browser) Timeout(d time.Duration) *Browser {
    ctx, _ := context.WithTimeout(b.ctx, d)
    return b.Context(ctx)
}

// Usage:
page := browser.Timeout(30 * time.Second).MustPage("https://example.com")
```

**Why this works:** Creates a new Browser value with modified context without mutating the original. Thread-safe because it's a value copy.

## Auto-Waiting (Sleeper Pattern)

### Sleeper Interface
```go
type Sleeper func(context.Context) error
```

### Built-in Sleepers

**BackoffSleeper** (default):
- Exponential backoff with jitter
- Growth: `A(n) = A(n-1) * random[1.9, 2.1)`
- Starts: 100ms, caps at 1 second

**CountSleeper**: Fixed number of retries, returns immediately each time

**Composable:**
- `EachSleepers(s1, s2)` - all must succeed
- `RaceSleepers(s1, s2)` - any can succeed

### Retry Loop
```go
func Retry(ctx context.Context, s Sleeper, fn func() (stop bool, err error)) error {
    for {
        stop, err := fn()
        if stop { return err }
        if err := s(ctx); err != nil { return err }
    }
}
```

### Element Wait Methods

| Method | What it Checks |
|--------|---------------|
| `WaitVisible()` | JS visibility check via evaluate |
| `WaitInteractable()` | Shape + pointer-events + not covered |
| `WaitEnabled()` | `!this.disabled` |
| `WaitWritable()` | `!this.readonly` |
| `WaitStable(d)` | Bounding box unchanged for duration d |
| `WaitStableRAF()` | Bounds unchanged for 2 animation frames |

### Interactable Check (before clicking)
```
1. pointerEvents !== 'none'
2. Element has visible shape (bounds exist)
3. Element is at given point (not covered by another element)
4. Parent contains element at that point
```

## Event System (goob Observable)

```
CDP WebSocket
    ↓
cdp.Client.consumeMessages() [single goroutine]
    ↓
Browser.initEvents() [goroutine]
    ├─ Creates goob.Observable
    ├─ Subscribes to cdp.Client.Event()
    └─ Republishes as *Message

Page.initEvents() [goroutine]
    ├─ Creates page-local Observable
    ├─ Subscribes to browser observable
    ├─ Filters by SessionID
    └─ Republishes to page listeners
```

Multiple subscribers don't block each other. Context-aware lifecycle.

## Selector Implementation

```go
const (
    SelectorTypeCSS   = "css-selector"
    SelectorTypeRegex = "regex"
    SelectorTypeText  = "text"
)
```

| Method | Implementation |
|--------|---------------|
| `Element(sel)` | CSS querySelector + retry with sleeper |
| `ElementX(xpath)` | XPath via JS + retry |
| `ElementR(sel, regex)` | CSS + text regex match + retry |
| `Elements(sel)` | querySelectorAll (no retry) |
| `Has(sel)` | Immediate check, returns bool |

### Race Pattern
```go
// First matching element wins
race := page.Race()
race.Element("button.submit")
race.ElementX("//a[@href='/login']")
race.Element("input[type=submit]")
el := race.MustDo()
```

## Key Takeaways for Gowright

1. **Single goroutine for message mux** - eliminates races on CDP read
2. **sync.Map for pending callbacks** - lock-free on the hot path
3. **Struct cloning for context** - clean, thread-safe API
4. **Configurable sleepers** - composable retry strategies
5. **Observable pub/sub for events** - multiple listeners, no callback hell
6. **Exponential backoff with jitter** - default retry that handles most cases
7. **Direct CDP, no relay** - the whole point of building in Go

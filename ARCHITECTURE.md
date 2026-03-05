# Gowright Architecture

A Go-native browser automation framework inspired by Playwright.

## Design Principles

1. **Direct CDP** - No relay servers, no Node.js subprocess. Go talks directly to Chrome via WebSocket/pipe.
2. **Goroutine-per-page** - Each page gets its own goroutine. No event loop bottleneck.
3. **Chromium-first** - Start with Chrome/Chromium via CDP. Firefox/WebKit later via their protocols.
4. **Playwright-compatible API** - Same mental model: Browser → Context → Page → Locator.
5. **Auto-waiting built-in** - Like Playwright, not like raw chromedp.

## Package Structure

```
gowright/
├── cmd/gowright/         # CLI (test runner, codegen, etc.)
├── pkg/
│   ├── cdp/              # CDP WebSocket client, message framing, session multiplexing
│   │   ├── client.go     # WebSocket connection, request-response matching
│   │   ├── session.go    # CDP session (multiplexed over single connection)
│   │   └── protocol.go   # CDP protocol types (generated from spec)
│   ├── browser/          # Browser lifecycle management
│   │   ├── browser.go    # Browser type, launch, connect
│   │   ├── context.go    # BrowserContext (cookies, permissions, isolation)
│   │   └── chromium/     # Chromium-specific launcher, args, binary finder
│   ├── api/              # Public API surface
│   │   ├── page.go       # Page (navigation, evaluation, screenshots)
│   │   ├── frame.go      # Frame (main + iframes)
│   │   ├── input.go      # Mouse, Keyboard, Touch
│   │   └── network.go    # Request, Response, Route
│   ├── locator/          # Locator resolution + auto-waiting
│   │   ├── locator.go    # Locator type with fluent API
│   │   ├── selector.go   # Selector parsing (css, xpath, text, role, testid)
│   │   └── wait.go       # Auto-wait strategies (visible, enabled, stable)
│   ├── expect/           # Assertion library
│   │   └── expect.go     # expect(locator).ToBeVisible(), etc.
│   └── runner/           # Test runner
│       ├── runner.go     # Test discovery, goroutine-based parallelism
│       ├── fixture.go    # Fixture system (browser, context, page)
│       └── reporter.go   # Test result reporting
└── _reference/           # Playwright source (gitignored)
```

## CDP Communication Flow

```
Go Code
  ↓
Locator.Click()
  ↓
Page.mouse.Click(x, y) → calls Input.dispatchMouseEvent via CDP
  ↓
cdp.Session.Send("Input.dispatchMouseEvent", params)
  ↓
cdp.Client (WebSocket connection, request ID tracking)
  ↓
Chrome/Chromium process (headless or headed)
```

## Session Multiplexing

Single WebSocket carries multiple CDP sessions via `sessionId` field:

```
cdp.Client (one WebSocket)
  ├── rootSession (sessionId: "")     → Browser-level commands
  ├── pageSession1 (sessionId: "abc") → Page 1 commands
  └── pageSession2 (sessionId: "def") → Page 2 commands
```

Messages are routed by sessionId. Each session has its own callback map for matching responses to requests.

## Key Differences from Playwright

| Aspect | Playwright | Gowright |
|--------|-----------|---------|
| Language | TypeScript/Node.js | Go |
| Parallelism | OS process per worker | Goroutine per test |
| CDP transport | Via Node.js relay (for non-JS clients) | Direct WebSocket |
| Browser support | Chromium + Firefox + WebKit | Chromium (initially) |
| Event handling | Single-threaded event loop | Multi-goroutine with channels |
| Memory per worker | ~50-100 MB | ~2-4 KB (goroutine) + shared browser |

## Multi-Language Architecture

Gowright follows Playwright's dual-access model:

### How Playwright Does It

Playwright's Node.js server is the brain. Client libraries in Python, Java, and .NET
are thin wrappers that spawn the Node.js server and talk to it over stdin/stdout using
a JSON-based channel protocol. All browser logic lives server-side.

### How Gowright Does It

```
┌─────────────────────────────────────────────────┐
│  Native Go API (go test)                        │
│  ─ Direct function calls, zero overhead         │
│  ─ Full access to all types and interfaces      │
└──────────────────┬──────────────────────────────┘
                   │ same code
                   ▼
┌─────────────────────────────────────────────────┐
│  gowright core (pkg/browser, pkg/cdp)           │
│  ─ CDP over WebSocket to Chrome                 │
│  ─ All browser logic lives here                 │
└──────────────────┬──────────────────────────────┘
                   │ exposed via
                   ▼
┌─────────────────────────────────────────────────┐
│  Server Protocol (JSON-RPC over stdin/stdout)   │
│  ─ `gowright server` command                    │
│  ─ Stateful: tracks browsers, pages, locators   │
│  ─ Bidirectional: events pushed to client       │
└──────────────────┬──────────────────────────────┘
                   │ consumed by
                   ▼
┌─────────────────────────────────────────────────┐
│  Client Libraries (thin wrappers)               │
│  ─ Python: gowright (pip install gowright)       │
│  ─ JS/TS: @gowright/test (npm)                  │
│  ─ Ruby, Java, etc.                             │
│  ─ Each spawns `gowright server`, sends JSON     │
└─────────────────────────────────────────────────┘
```

### Protocol Design

JSON-RPC over stdin/stdout (same approach as Playwright):

```
Client → Server:  {"id":1, "method":"browser.launch", "params":{}}
Server → Client:  {"id":1, "result":{"browserId":"b1"}}

Client → Server:  {"id":2, "method":"page.goto", "params":{"pageId":"p1","url":"..."}}
Server → Client:  {"id":2, "result":{}}

Server → Client:  {"method":"page.onLoad", "params":{"pageId":"p1"}}  (event, no id)
```

### Design Rules

1. **Every feature must work through the protocol.** When adding a new API (e.g. `page.Goto`),
   also define its protocol message. The Go-native API and the protocol are two views of the
   same capability.

2. **The Go binary is the single source of truth.** Client libraries never contain browser
   logic — they only serialize/deserialize protocol messages.

3. **Objects are referenced by ID.** The server assigns string IDs to browsers, contexts,
   pages, locators, etc. Clients pass these IDs in subsequent calls.

4. **Events flow server → client.** Page load, console messages, network events, etc. are
   pushed as JSON messages without an `id` field.

## Build Phases

### Phase 1: CDP Foundation
- WebSocket client with request-response matching
- Session multiplexing
- Chrome binary finder + launcher
- Basic Browser → Context → Page hierarchy

### Phase 2: Page Operations
- Navigation (goto, reload, back, forward)
- JavaScript evaluation
- Input (mouse, keyboard)
- Screenshots

### Phase 3: Locators & Auto-waiting
- CSS, XPath, text, role, testid selectors
- Auto-wait (visible, enabled, stable)
- Locator fluent API

### Phase 4: Network
- Request/response interception
- Route handling
- Cookie management

### Phase 5: Test Runner
- Test discovery
- Goroutine-based parallel execution
- Fixture system
- Reporters (console, JSON, JUnit)

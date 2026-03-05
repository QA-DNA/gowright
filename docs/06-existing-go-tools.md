# Existing Go Browser Automation Tools

## chromedp

**Repo:** github.com/chromedp/chromedp (11,500+ stars)

### Architecture
- Pure Go, zero external dependencies
- Direct CDP via WebSocket
- Chrome/Chromium only
- Uses DOM node IDs (not remote object IDs)
- Single event loop with fixed-size event buffer

### Strengths
- Mature, large community
- No runtime dependency
- Good for single-browser automation

### Weaknesses
- Fixed event buffer → deadlocks under high concurrency
- No auto-waiting (WaitVisible/WaitReady hang ~50% of time in some scenarios)
- Single event loop - slow handlers block all events
- Requires system Chrome (version drift risk)
- ~5 concurrent browser instance limit (memory)

### What to learn from
- Don't use fixed-size event buffers
- Don't use DOM node IDs (use remote object IDs like Puppeteer/Rod)
- Must have auto-waiting built-in

---

## Rod (go-rod)

**Repo:** github.com/go-rod/rod (~5,000 stars)

### Architecture
- Pure Go, direct CDP
- Chrome/Chromium only
- Uses remote object IDs (same as Puppeteer)
- Decode-on-demand JSON (not eager like chromedp)
- Dynamic event buffer (goob library)
- Bundles specific Chromium revision

### Strengths
- Better than chromedp in every measurable way
- Built-in auto-waiting
- Thread-safe by design
- Lower memory than chromedp in concurrent scenarios
- Built-in Chrome version management
- Both high-level and low-level APIs

### Weaknesses
- Chrome/Chromium only
- Smaller ecosystem than chromedp
- No codegen equivalent

### What to learn from (a lot!)
- Struct cloning for context chaining
- Configurable sleeper pattern
- Observable pub/sub for events
- Decode-on-demand for performance
- Single-goroutine message mux

---

## playwright-go (community)

**Repo:** github.com/playwright-community/playwright-go (3,200+ stars)

### Architecture
- Go bindings over Node.js Playwright driver
- Spawns Node.js subprocess, communicates via stdio
- Length-prefixed binary JSON protocol: [4-byte LE length][UTF-8 JSON]
- NOT a native Go implementation

### Performance Overhead
- Every method call: Go serialize → pipe write → Node.js process → CDP → pipe read → Go deserialize
- 1-5ms per-call overhead from pipe I/O + JSON
- Startup: ~100-200ms process spawn + ~50-100ms handshake
- First run: downloads ~50MB Node.js driver + ~200MB per browser

### Strengths
- Multi-browser (Chromium, Firefox, WebKit) - only Go option
- Full Playwright feature parity
- Auto-waiting, network interception, etc.

### Weaknesses
- Node.js dependency defeats purpose of Go
- Version coupling (Go package must match driver version exactly)
- Single Node.js process per Playwright instance
- Currently seeking maintainers (2026)
- 68 open issues, 7 open PRs

### What to learn from
- API surface design (same as Playwright)
- What the Go community wants from browser automation
- The demand exists (3,200 stars despite Node.js dependency)

---

## mafredri/cdp

**Repo:** github.com/mafredri/cdp

### Architecture
- Low-level type-safe CDP bindings for Go
- Generated from CDP protocol spec
- Not a high-level automation tool

### Useful for
- Understanding how to generate Go types from CDP spec
- Type-safe CDP command building

---

## Comparison Table

| Feature | chromedp | Rod | playwright-go | Gowright (goal) |
|---------|----------|-----|---------------|-----------------|
| Stars | 11,500 | ~5,000 | 3,200 | N/A |
| Pure Go | Yes | Yes | No (Node.js) | Yes |
| Browser | Chrome only | Chrome only | Chrome+FF+WebKit | Chrome (initially) |
| Auto-wait | No | Yes | Yes | Yes |
| Event model | Fixed buffer | Dynamic (goob) | N/A (Node.js) | Channels + pub/sub |
| Test runner | No | No | No | Yes (goroutine-based) |
| Codegen | No | No | No | Planned |
| Assertions | No | No | No | Yes (expect API) |
| Concurrency | Deadlock-prone | Thread-safe | Single process | Goroutine per test |

## The Gap Gowright Fills

None of the existing Go tools provide:
1. A **test runner** with parallel execution
2. A **fixture system** (auto browser/page lifecycle)
3. An **assertion library** (expect API)
4. **Codegen** (record → generate tests)
5. **Tracing/debugging** tools

Rod is excellent as a library. Gowright aims to be a framework (like Playwright Test is to Playwright Core).

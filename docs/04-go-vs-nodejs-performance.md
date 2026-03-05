# Go vs Node.js Performance Analysis (Browser Automation Context)

## Executive Summary

| Dimension | Winner | Magnitude |
|-----------|--------|-----------|
| Per-action browser command | ~Equal | Browser is the bottleneck |
| Orchestration overhead (relay elimination) | Go | 15-20% faster |
| Parallel test execution | Go | 3-5x more concurrent tests per machine |
| Memory per worker | Go | ~50x less (2-4 KB goroutine vs 50-100 MB process) |
| High concurrency (3000+ req/s) | Go | 118x lower latency at saturation |
| Moderate load (2000 req/s) | Node.js | 3x lower latency (counterintuitive) |
| Pure I/O wait (page loading) | Equal | Network is bottleneck |
| Ecosystem maturity | Node.js | Playwright has 68K stars, Microsoft backing |

## Concurrency Model

### Node.js
- Single-threaded event loop (V8 + libuv)
- All JS runs on one thread
- Parallelism requires explicit worker threads (~1 MB each) or child processes (~50 MB)
- 11 OS threads at startup, 15 under load

### Go
- GMP scheduler: Goroutines multiplexed over OS threads
- GOMAXPROCS defaults to CPU count
- Each goroutine starts at 2-4 KB stack
- Preemptive scheduling since Go 1.14
- Work-stealing across processor queues
- 4-5 stable OS threads under load

## Benchmark Data

### HTTP Throughput (Go vs Node.js)
From API benchmark (identical PostgreSQL-backed API, EC2 t2.micro):

| Load | Node.js Latency | Go Latency | Winner |
|------|----------------|------------|--------|
| 2,000 req/s | 147 ms | 459 ms | Node.js (3.1x) |
| 3,000 req/s | 7,079 ms | 60.5 ms | Go (118x) |
| 5,000 req/s | 5-10 sec | stable | Go |

Node.js hits a cliff at ~2,500 req/s. Go degrades gracefully.

### CPU-Bound Parallelism (goroutines vs worker threads)
From i9-9900K benchmark, 10 concurrent users, 30s intervals:

| Metric | Go | Node.js Worker Threads |
|--------|-----|----------------------|
| Iterations | ~90-91 | ~40 |
| P95 Latency | 3.5s | 8.5s |
| Memory | 0.0% host | 1.4% host (448 MB) |

Go: 125% more throughput, negligible memory.

### Browser Automation Specific

Playwright's performance profile:
- 290ms per action (baseline)
- ~2.1 GB for 10 parallel tests
- 15-30 concurrent tests on 8-core machine
- Each worker = full OS process

What Go could achieve:
- ~240ms per action (eliminating Node.js relay = 15-20% faster)
- ~0.5 GB orchestration + same browser memory
- 50-100+ concurrent tests (goroutine-based)
- Each "worker" = goroutine (2-4 KB)

## Where Go Wins for Browser Automation

### 1. No Event Loop Bottleneck
Node.js processes all CDP messages, callbacks, and event dispatch on one thread.
With 50 browser pages, a slow JSON.parse on a large CDP response blocks all other pages.
Go goroutines run on multiple cores - one slow page doesn't block others.

### 2. Eliminating the Relay Server
Playwright's non-JS clients go: Client → Node.js relay → Browser (3 hops).
Go talks directly to Chrome: Go → Browser (2 hops).
Browser-use.com documented "massively increased speed" after this change.

### 3. Lightweight Parallelism
50 parallel tests:
- Playwright: 50 OS processes × ~100 MB = ~5 GB just for orchestration
- Go: 50 goroutines × ~4 KB = ~200 KB for orchestration (browser memory is shared)

### 4. Better Scheduling Fairness Under Load
Go's work-stealing scheduler distributes work across cores automatically.
Node.js event loop processes callbacks FIFO - burst from one page delays all others.

## Where Go Does NOT Help

### 1. Browser is the Real Bottleneck
Headless Chromium: 50-150 MB on startup. Each tab adds a renderer process.
Whether Go or Node.js sends the CDP command, Chrome takes the same time.

### 2. Pure I/O Wait is Identical
`await page.goto(url)` vs goroutine waiting for navigation event = same wall clock.
The network is the bottleneck, not the language runtime.

### 3. Cross-Browser Protocol Tax
Playwright sends 326 KB of messages per interaction (vs Puppeteer's 11 KB).
This is because Playwright injects JS for cross-browser compatibility.
A Go rewrite maintaining cross-browser support would have the same overhead.

### 4. Ecosystem Gap
Playwright: codegen, trace viewer, test runner, fixtures, reporters, 4 language SDKs.
Go: chromedp (Chrome-only, no auto-wait), Rod (Chrome-only, auto-wait), playwright-go (Node.js subprocess).

## Memory Deep Dive

### Runtime Baseline
- Go: 76% less memory than Node.js for equivalent applications
- Go goroutine: 2-4 KB initial stack (grows dynamically)
- Node.js worker thread: ~1 MB per thread
- Node.js child process: ~50 MB

### Browser Memory (language-independent)
- Cold-boot headless Chromium: 50-150 MB
- New browser context: single-digit KB (shares browser process)
- Each tab with loaded page: varies widely (10-200 MB depending on page)

### File Descriptors
Go: intelligent pre-allocation and reuse via network poller
Node.js: linear, unmanaged FD growth with connection count
This matters when managing 50+ WebSocket connections to browser pages.

## Real-World Case Study: Browser-Use.com

Browser-use.com migrated from Playwright (Node.js relay) to direct CDP:
- "Massively increased speed" for element extraction and screenshots
- Root cause: Node.js WebSocket relay = second network hop on every CDP call
- State synchronization deadlocks between three runtimes (browser, Node.js, Python)
- After migration: 2 hops instead of 3, eliminated deadlock class entirely

## Conclusion for Gowright

The strongest arguments for Go:
1. **Test runner parallelism** - goroutines vs OS processes is a generational improvement
2. **Direct CDP** - eliminate 15-20% relay overhead
3. **Memory efficiency** - run 3-5x more tests on same hardware
4. **Predictable scaling** - no cliff at high concurrency

The honest limitations:
1. Browser memory dominates at scale regardless of language
2. Per-action speedup is bounded at ~15-20% (browser is the bottleneck)
3. Cross-browser support requires massive engineering investment
4. Starting from scratch vs Playwright's 5+ years of maturity

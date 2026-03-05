# Playwright Internals Reference

## Architecture Overview

Playwright has a 4-layer communication stack:

```
Layer 1: User API (client process) - thin wrappers, no logic
Layer 2: Channel Protocol (JSON-RPC over pipe/WebSocket)
Layer 3: Core Engine (Node.js server) - auto-waiting, actionability, frame mgmt
Layer 4: Browser Protocol - CDP / Juggler / WebKit Inspector
```

For non-JS clients (Python, Java, Go), there's an extra hop:
`Client → Node.js relay → Browser` (3 hops vs 2 for JS)

## Key Files Map

### Server Layer (Core Automation)

| File | Size | Purpose |
|------|------|---------|
| `server/browser.ts` | 10.6K | Abstract Browser base class |
| `server/browserContext.ts` | 33.6K | Context isolation (cookies, permissions, network) |
| `server/browserType.ts` | 33.6K | Factory for launching/connecting browsers |
| `server/page.ts` | 21.5K | Single tab - coordinates frames, network, events |
| `server/frames.ts` | 35.1K | FrameManager + Frame - navigation, JS execution, DOM queries |
| `server/dom.ts` | 18.8K | ElementHandle - click, fill, type, screenshot |
| `server/input.ts` | ~15K | Mouse, Keyboard, Touch high-level API |
| `server/selectors.ts` | 3K | Selector engine registry |
| `server/javascript.ts` | 9.4K | ExecutionContext, JSHandle |
| `server/network.ts` | 16.8K | Network utilities, cookie/header management |
| `server/fetch.ts` | 15.1K | APIRequestContext (HTTP client) |
| `server/progress.ts` | ~3K | ProgressController - timeout, abort, logging |
| `server/instrumentation.ts` | 19K | SdkObject base, telemetry hooks |

### Chromium Implementation

| File | Size | Purpose |
|------|------|---------|
| `server/chromium/chromium.ts` | 20K | Chromium launcher |
| `server/chromium/crConnection.ts` | 8.7K | CRConnection + CRSession (CDP multiplexing) |
| `server/chromium/crBrowser.ts` | 24K | CRBrowser + CRBrowserContext |
| `server/chromium/crPage.ts` | 52K | CRPage + FrameSession (LARGEST file) |
| `server/chromium/crInput.ts` | ~5K | RawMouseImpl, RawKeyboardImpl, RawTouchscreenImpl |
| `server/chromium/crNetworkManager.ts` | 40K | Network interception for Chrome |
| `server/chromium/protocol.d.ts` | 24K lines | CDP type definitions |

### Test Runner

| File | Size | Purpose |
|------|------|---------|
| `runner/testRunner.ts` | 496 lines | Main orchestrator |
| `runner/dispatcher.ts` | 670 lines | Parallel test distribution to workers |
| `runner/tasks.ts` | 444 lines | Task pipeline (setup, load, execute, report) |
| `runner/workerHost.ts` | 112 lines | Worker process lifecycle |
| `runner/loadUtils.ts` | 382 lines | Test file loading, TS compilation |
| `worker/workerMain.ts` | - | Worker process entry point |
| `worker/testInfo.ts` | - | TestInfoImpl - test metadata/attachments |

### Protocol & Transport

| File | Purpose |
|------|---------|
| `server/transport.ts` | WebSocketTransport + PipeTransport |
| `server/pipeTransport.ts` | Stdio pipe transport (null-byte delimited JSON) |
| `protocol/src/protocol.yml` | RPC message definitions |
| `server/dispatchers/*.ts` | ~10 dispatcher files bridging server→client |

## Object Hierarchy

```
Browser
  ├─ BrowserContext (via newContext)
  │   ├─ Page (via newPage)
  │   │   ├─ FrameManager
  │   │   │   ├─ Frame (main)
  │   │   │   └─ Frame (child - iframes)
  │   │   │       └─ ExecutionContext
  │   │   │           ├─ ElementHandle
  │   │   │           └─ JSHandle
  │   │   ├─ Mouse, Keyboard, Touch
  │   │   └─ Worker
  │   └─ Network (interception, cookies)
  └─ BrowserType (launch factory)
```

## CDP Communication Detail

### Transport Selection

```typescript
// browserType.ts lines 274-279
if (options.cdpPort !== undefined || !this.supportsPipeTransport()) {
  transport = await WebSocketTransport.connect(progress, wsEndpoint);
} else {
  // Use pipes for local launch - faster than WebSocket
  transport = new PipeTransport(stdio[3], stdio[4]);
}
```

Stdio mapping for launched process:
- `stdio[0]` = stdin (ignored)
- `stdio[1]` = stdout (piped for logging)
- `stdio[2]` = stderr (piped for logging)
- `stdio[3]` = CDP write channel (browser → client)
- `stdio[4]` = CDP read channel (client → browser)

### Message Types

```typescript
type ProtocolRequest = {
  id: number;
  method: string;
  params: any;
  sessionId?: string;  // for routing to correct session
};

type ProtocolResponse = {
  id?: number;        // present = response to request
  method?: string;    // present = event
  sessionId?: string;
  error?: { message: string; data: any; code?: number };
  params?: any;
  result?: any;
};
```

### Session Multiplexing (crConnection.ts)

```
CRConnection (single transport)
  ├── rootSession (sessionId: '') → Browser.getVersion, Target.setAutoAttach
  ├── pageSession (sessionId: 'ABC') → Page commands
  └── pageSession (sessionId: 'DEF') → Page commands
```

Each CRSession has its own `_callbacks` map matching request IDs to promises.

### Request-Response Flow

1. `CRSession.send(method, params)` → creates promise, stores in callback map
2. `CRConnection._rawSend(sessionId, method, params)` → increments ID, writes to transport
3. Browser responds with `{id, result}` or `{id, error}`
4. `CRConnection._onMessage()` → routes by sessionId to correct CRSession
5. `CRSession._onMessage()` → resolves/rejects callback by ID

### Event Flow

Events have no `id`, only `method` + `params`:
```typescript
// In CRSession._onMessage:
if (!object.id) {
  // Event - emit asynchronously
  Promise.resolve().then(() => this.emit(object.method, object.params));
}
```

## How page.click() Works (Full Trace)

```
1. ElementHandle.click(options)                    [dom.ts]
2.   → _retryPointerAction(progress, 'click', ...) [dom.ts]
3.     → waitForActionability checks:
          - scrollIntoView
          - waitForVisible
          - waitForEnabled
          - waitForStable (position not changing)
          - check not obscured by other element
4.     → Page.mouse.click(x, y, options)           [input.ts]
5.       → Mouse.move(x, y) + Mouse.down() + Mouse.up()
6.         → RawMouseImpl.move/down/up()           [crInput.ts]
7.           → CRSession.send('Input.dispatchMouseEvent', {
                type: 'mouseMoved'/'mousePressed'/'mouseReleased',
                x, y, button, buttons, modifiers, clickCount
              })
8.             → CRConnection._rawSend()          [crConnection.ts]
9.               → Transport.send(JSON)            [transport.ts]
10.                → Chrome processes the event
```

## Auto-Waiting / Actionability

Before every action, Playwright checks (in dom.ts):
1. Element is **attached** to DOM
2. Element is **visible** (not `display:none`, has size, opacity > 0)
3. Element is **stable** (position not animating)
4. Element **receives events** (not obscured by overlay)
5. Element is **enabled** (not disabled form control)

These checks are **retried** with the ProgressController until timeout.

## Test Runner Parallelism

Current model:
- Main process spawns N worker processes (default = CPU cores)
- Each worker = full Node.js process + browser instance
- Tests within a file run sequentially in same worker
- Files distributed across workers by dispatcher
- Worker restart on failure (cold boot)

Key classes:
- `Dispatcher` - distributes tests to worker pool
- `JobDispatcher` - individual job coordinator
- `WorkerHost` - manages worker process lifecycle

## Performance Numbers

- Playwright: 290ms per action, ~1,240 tests/hour
- Puppeteer (direct CDP): 15-20% faster than Playwright on Chromium
- Playwright WebSocket overhead: 326KB messages per interaction vs Puppeteer's 11KB (29x more)
- Memory: ~2.1 GB for 10 parallel tests
- Capacity: 15-30 concurrent tests on 8-core machine
- Firefox: 34-70% slower than Chromium under Playwright
- WebKit: 13-60% slower than Chromium under Playwright

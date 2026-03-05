# CDP Protocol Reference for Gowright

## What is CDP?

Chrome DevTools Protocol - a WebSocket-based JSON-RPC protocol for controlling Chromium browsers.
Spec: https://chromedevtools.github.io/devtools-protocol/

## Connection Setup

1. Launch Chrome with `--remote-debugging-port=0` (random port)
2. Chrome prints `DevTools listening on ws://127.0.0.1:PORT/devtools/browser/UUID` to stderr
3. Connect WebSocket to that URL
4. Send CDP commands, receive responses and events

Alternative: Use stdio pipes (faster, no WebSocket overhead):
- Launch with `--remote-debugging-pipe`
- Communicate via stdio fd 3 (read) and fd 4 (write)
- Messages are null-byte (`\0`) delimited JSON

## Key CDP Domains

### Target (Session Management)
```json
// Auto-attach to new pages/workers
{"id": 1, "method": "Target.setAutoAttach", "params": {
  "autoAttach": true,
  "waitForDebuggerOnStart": true,
  "flatten": true
}}

// New page appears → event:
{"method": "Target.attachedToTarget", "params": {
  "sessionId": "ABC123",
  "targetInfo": {"targetId": "...", "type": "page", "url": "..."}
}}

// All subsequent commands for this page include sessionId:
{"id": 2, "method": "Page.navigate", "params": {"url": "..."}, "sessionId": "ABC123"}
```

### Page (Navigation & Lifecycle)
```json
// Enable page events
{"method": "Page.enable"}

// Navigate
{"method": "Page.navigate", "params": {"url": "https://example.com"}}

// Get frame tree
{"method": "Page.getFrameTree"}
// Response: {frameTree: {frame: {id, url, ...}, childFrames: [...]}}

// Screenshot
{"method": "Page.captureScreenshot", "params": {"format": "png"}}

// Key events:
// Page.frameAttached, Page.frameDetached, Page.frameNavigated
// Page.loadEventFired, Page.domContentEventFired
// Page.fileChooserOpened
```

### Runtime (JavaScript Execution)
```json
// Evaluate JS
{"method": "Runtime.evaluate", "params": {
  "expression": "document.title",
  "returnByValue": true
}}

// Call function on object
{"method": "Runtime.callFunctionOn", "params": {
  "functionDeclaration": "function() { return this.textContent; }",
  "objectId": "...",
  "returnByValue": true
}}

// Key events:
// Runtime.executionContextCreated - new JS context (page load, iframe)
// Runtime.consoleAPICalled - console.log() etc.
// Runtime.exceptionThrown - uncaught exceptions
// Runtime.bindingCalled - exposed function called from page
```

### DOM (Element Queries)
```json
// Query selector
{"method": "DOM.querySelector", "params": {"nodeId": 1, "selector": ".btn"}}

// Get document
{"method": "DOM.getDocument", "params": {"depth": 0}}

// Get content quads (element position/bounds)
{"method": "DOM.getContentQuads", "params": {"objectId": "..."}}

// Get box model
{"method": "DOM.getBoxModel", "params": {"objectId": "..."}}
```

### Input (Mouse, Keyboard, Touch)
```json
// Mouse move
{"method": "Input.dispatchMouseEvent", "params": {
  "type": "mouseMoved", "x": 100, "y": 200,
  "button": "left", "buttons": 0, "modifiers": 0
}}

// Mouse click = mousePressed + mouseReleased
{"method": "Input.dispatchMouseEvent", "params": {
  "type": "mousePressed", "x": 100, "y": 200,
  "button": "left", "buttons": 1, "clickCount": 1
}}
{"method": "Input.dispatchMouseEvent", "params": {
  "type": "mouseReleased", "x": 100, "y": 200,
  "button": "left", "buttons": 0, "clickCount": 1
}}

// Keyboard
{"method": "Input.dispatchKeyEvent", "params": {
  "type": "keyDown", "key": "a", "code": "KeyA",
  "text": "a", "windowsVirtualKeyCode": 65
}}

// Insert text (faster than key events)
{"method": "Input.insertText", "params": {"text": "hello world"}}

// Touch
{"method": "Input.dispatchTouchEvent", "params": {
  "type": "touchStart", "touchPoints": [{"x": 100, "y": 200}]
}}
```

### Network (Interception)
```json
// Enable network events
{"method": "Network.enable"}

// Set request interception
{"method": "Fetch.enable", "params": {
  "patterns": [{"urlPattern": "*", "requestStage": "Request"}]
}}

// Intercepted request event:
{"method": "Fetch.requestPaused", "params": {
  "requestId": "...", "request": {"url": "...", "method": "GET", "headers": {...}}
}}

// Continue request
{"method": "Fetch.continueRequest", "params": {"requestId": "..."}}

// Fulfill request (mock)
{"method": "Fetch.fulfillRequest", "params": {
  "requestId": "...", "responseCode": 200,
  "responseHeaders": [...], "body": "base64..."
}}

// Cookie management
{"method": "Network.setCookies", "params": {"cookies": [...]}}
{"method": "Network.getCookies"}
{"method": "Network.clearBrowserCookies"}
```

### Browser (Top-level)
```json
{"method": "Browser.getVersion"}
// Response: {product: "Chrome/120.0...", revision: "...", userAgent: "..."}

{"method": "Browser.close"}

// Create browser context (incognito-like isolation)
{"method": "Target.createBrowserContext"}
// Response: {browserContextId: "..."}

{"method": "Target.disposeBrowserContext", "params": {"browserContextId": "..."}}
```

### Emulation
```json
// Set viewport
{"method": "Emulation.setDeviceMetricsOverride", "params": {
  "width": 1280, "height": 720, "deviceScaleFactor": 1, "mobile": false
}}

// Set geolocation
{"method": "Emulation.setGeolocationOverride", "params": {
  "latitude": 40.7128, "longitude": -74.0060, "accuracy": 1
}}

// Set user agent
{"method": "Emulation.setUserAgentOverride", "params": {
  "userAgent": "..."
}}
```

## Modifier Bitmasks

```go
// Keyboard modifiers
const (
    ModifierAlt     = 1
    ModifierControl = 2
    ModifierMeta    = 4
    ModifierShift   = 8
)

// Mouse buttons
const (
    ButtonLeft   = 1
    ButtonRight  = 2
    ButtonMiddle = 4
)
```

## Chrome Launch Flags (Essential)

```
--headless=new              # Headless mode (new headless, not old)
--disable-gpu               # Disable GPU acceleration
--no-sandbox                # Required in Docker/CI
--disable-setuid-sandbox    # Required in Docker/CI
--disable-dev-shm-usage     # Use /tmp instead of /dev/shm (Docker)
--remote-debugging-pipe     # Use pipes instead of WebSocket
--remote-debugging-port=0   # Random debugging port (WebSocket mode)
--disable-background-networking
--disable-background-timer-throttling
--disable-backgrounding-occluded-windows
--disable-breakpad
--disable-component-extensions-with-background-pages
--disable-component-update
--disable-default-apps
--disable-extensions
--disable-hang-monitor
--disable-ipc-flooding-protection
--disable-popup-blocking
--disable-prompt-on-repost
--disable-renderer-backgrounding
--disable-sync
--enable-features=NetworkService,NetworkServiceInProcess
--force-color-profile=srgb
--metrics-recording-only
--no-first-run
--password-store=basic
--use-mock-keychain
--user-data-dir=/tmp/gowright-profile-XXXX
```

## Chrome Binary Locations

### macOS
```
/Applications/Google Chrome.app/Contents/MacOS/Google Chrome
/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary
/Applications/Chromium.app/Contents/MacOS/Chromium
```

### Linux
```
google-chrome
google-chrome-stable
chromium
chromium-browser
/usr/bin/google-chrome
```

### Windows
```
%LOCALAPPDATA%\Google\Chrome\Application\chrome.exe
%PROGRAMFILES%\Google\Chrome\Application\chrome.exe
%PROGRAMFILES(X86)%\Google\Chrome\Application\chrome.exe
```

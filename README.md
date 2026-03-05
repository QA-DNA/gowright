# Gowright

A Go-native browser automation framework inspired by [Playwright](https://playwright.dev). Direct CDP communication with Chrome — no Node.js relay, no subprocess overhead.

## Features

- **Direct CDP over WebSocket** — Go talks directly to Chrome, no intermediary
- **Goroutine-per-page** — native Go concurrency, no event loop bottleneck
- **Playwright-compatible API** — same mental model: Browser → Context → Page → Locator
- **Auto-waiting built-in** — locators wait for elements to be actionable
- **Lightweight** — goroutines instead of OS processes (~2-4 KB vs ~50-100 MB per worker)

## Installation

```bash
go get github.com/QA-DNA/gowright
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/QA-DNA/gowright"
)

func main() {
	ctx := context.Background()

	browser, err := gowright.Launch(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer browser.Close()

	page, err := browser.NewPage(ctx)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := page.Goto(ctx, "https://example.com"); err != nil {
		log.Fatal(err)
	}

	title, err := page.Title(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Page title:", title)
}
```

## Project Structure

```
gowright/
├── pkg/
│   ├── cdp/          # CDP WebSocket client, session multiplexing
│   ├── browser/      # Browser lifecycle, Page, Locator, input
│   ├── config/       # Launch and viewport configuration
│   ├── locator/      # Selector parsing, auto-wait strategies
│   ├── expect/       # Assertion library
│   ├── api/          # Public API surface
│   └── runner/       # Test runner with fixture system
├── examples/         # Usage examples
└── docs/             # Design documents and references
```

## Architecture

Gowright communicates directly with Chrome via the Chrome DevTools Protocol (CDP) over a single WebSocket connection, multiplexing multiple sessions (one per page) over that connection.

```
Go Code → Locator.Click() → CDP Session → WebSocket → Chrome
```

See [ARCHITECTURE.md](ARCHITECTURE.md) for full details.

## Requirements

- Go 1.25+
- Chrome or Chromium installed locally

## License

MIT

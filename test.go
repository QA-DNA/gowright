package gowright

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/PeterStoica/gowright/pkg/browser/chromium"
)

// TestConfig holds configuration for the test runner.
type TestConfig struct {
	BaseURL  string
	Headless bool
	Timeout  time.Duration
	Viewport *Viewport
	SlowMo   time.Duration
}

// Test is the entry point for Playwright-style Go tests.
type Test struct {
	config TestConfig
}

// NewTest creates a new Test with sensible defaults.
// Pass optional TestConfig to override defaults.
func NewTest(configs ...TestConfig) *Test {
	cfg := TestConfig{
		Headless: true,
		Timeout:  30 * time.Second,
	}
	if len(configs) > 0 {
		c := configs[0]
		if c.BaseURL != "" {
			cfg.BaseURL = c.BaseURL
		}
		if c.Timeout != 0 {
			cfg.Timeout = c.Timeout
		}
		if c.Viewport != nil {
			cfg.Viewport = c.Viewport
		}
		if c.SlowMo != 0 {
			cfg.SlowMo = c.SlowMo
		}
		// Headless: only override if explicitly set in the provided config.
		// We check by seeing if the caller provided a config at all.
		cfg.Headless = c.Headless
	}
	return &Test{config: cfg}
}

// Run creates a t.Run sub-test, calls t.Parallel(), launches a browser,
// creates a context and page, and passes a TestContext to fn.
func (tt *Test) Run(t *testing.T, name string, fn func(pw *TestContext)) {
	t.Run(name, func(t *testing.T) {
		t.Parallel()
		tc := tt.createContext(t)
		defer tc.cleanup()
		fn(tc)
	})
}

// createContext launches a browser, creates a context and page,
// and returns a fully-wired TestContext.
func (tt *Test) createContext(t *testing.T) *TestContext {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), tt.config.Timeout)

	opts := chromium.DefaultLaunchOptions()
	opts.Headless = tt.config.Headless
	if tt.config.SlowMo > 0 {
		opts.SlowMo = tt.config.SlowMo
	}

	b, err := Launch(ctx, opts)
	if err != nil {
		cancel()
		t.Fatalf("gowright: failed to launch browser: %v", err)
	}

	if tt.config.Viewport != nil {
		b.SetDefaultViewport(tt.config.Viewport.Width, tt.config.Viewport.Height)
	}

	bCtx, err := b.NewContext()
	if err != nil {
		b.Close()
		cancel()
		t.Fatalf("gowright: failed to create browser context: %v", err)
	}

	page, err := bCtx.NewPage()
	if err != nil {
		bCtx.Close()
		b.Close()
		cancel()
		t.Fatalf("gowright: failed to create page: %v", err)
	}

	baseURL := strings.TrimRight(tt.config.BaseURL, "/")

	tc := &TestContext{
		t:       t,
		Page:    newTestPage(t, page, baseURL),
		Context: bCtx,
		Browser: b,
		cancel:  cancel,
		ctx:     ctx,
	}
	return tc
}

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

// Suite represents a test group created by Describe.
type Suite struct {
	tt         *Test
	t          *testing.T
	beforeEach []func(*TestContext)
	afterEach  []func(*TestContext)
	parent     *Suite
}

// Describe creates a grouped test suite with hooks.
func (tt *Test) Describe(t *testing.T, name string, fn func(s *Suite)) {
	t.Run(name, func(t *testing.T) {
		s := &Suite{tt: tt, t: t}
		fn(s)
	})
}

// Describe creates a nested suite inheriting parent hooks.
func (s *Suite) Describe(name string, fn func(s *Suite)) {
	s.t.Run(name, func(t *testing.T) {
		child := &Suite{tt: s.tt, t: t, parent: s}
		fn(child)
	})
}

// BeforeEach registers a hook that runs before each test.
func (s *Suite) BeforeEach(fn func(pw *TestContext)) {
	s.beforeEach = append(s.beforeEach, fn)
}

// AfterEach registers a hook that runs after each test.
func (s *Suite) AfterEach(fn func(pw *TestContext)) {
	s.afterEach = append(s.afterEach, fn)
}

// Test runs a test within the suite, executing hooks.
func (s *Suite) Test(name string, fn func(pw *TestContext)) {
	s.t.Run(name, func(t *testing.T) {
		t.Parallel()
		pw := s.tt.createContext(t)
		defer pw.cleanup()

		// Collect hooks from parent chain (outermost first)
		befores, afters := s.collectHooks()
		for _, hook := range befores {
			hook(pw)
		}
		// Register afters as cleanup (reverse order)
		for i := len(afters) - 1; i >= 0; i-- {
			after := afters[i]
			t.Cleanup(func() { after(pw) })
		}
		fn(pw)
	})
}

// collectHooks walks parent chain, returns hooks outermost-first.
func (s *Suite) collectHooks() (befores []func(*TestContext), afters []func(*TestContext)) {
	var chain []*Suite
	for cur := s; cur != nil; cur = cur.parent {
		chain = append(chain, cur)
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	for _, suite := range chain {
		befores = append(befores, suite.beforeEach...)
		afters = append(afters, suite.afterEach...)
	}
	return
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

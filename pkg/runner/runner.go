package runner

import (
	"context"
	"testing"
	"time"

	"github.com/PeterStoica/gowright/pkg/browser"
	"github.com/PeterStoica/gowright/pkg/browser/chromium"
	"github.com/PeterStoica/gowright/pkg/config"
)

type TestOptions struct {
	Headless         bool
	SlowMo           time.Duration
	Timeout          time.Duration
	BaseURL          string
	ViewportWidth    int
	ViewportHeight   int
	ContextOptions   func(*browser.BrowserContext)
}

var DefaultOptions = TestOptions{
	Headless:       true,
	Timeout:        30 * time.Second,
	ViewportWidth:  1280,
	ViewportHeight: 720,
}

type Fixture struct {
	T       *testing.T
	Browser *browser.Browser
	Context *browser.BrowserContext
	Page    *browser.Page
	opts    TestOptions
}

func New(t *testing.T, opts ...TestOptions) *Fixture {
	t.Helper()
	cfg, _ := config.Load()
	opt := TestOptions{
		Headless:       cfg.IsHeadless(),
		Timeout:        cfg.TimeoutDuration(),
		BaseURL:        cfg.BaseURL,
		ViewportWidth:  1280,
		ViewportHeight: 720,
	}
	if cfg.Viewport != nil {
		opt.ViewportWidth = cfg.Viewport.Width
		opt.ViewportHeight = cfg.Viewport.Height
	}
	if cfg.SlowMoDuration() > 0 {
		opt.SlowMo = cfg.SlowMoDuration()
	}
	if len(opts) > 0 {
		o := opts[0]
		opt.Headless = o.Headless
		if o.Timeout > 0 {
			opt.Timeout = o.Timeout
		}
		if o.SlowMo > 0 {
			opt.SlowMo = o.SlowMo
		}
		if o.BaseURL != "" {
			opt.BaseURL = o.BaseURL
		}
		if o.ViewportWidth > 0 {
			opt.ViewportWidth = o.ViewportWidth
		}
		if o.ViewportHeight > 0 {
			opt.ViewportHeight = o.ViewportHeight
		}
		opt.ContextOptions = o.ContextOptions
	}
	if opt.Timeout == 0 {
		opt.Timeout = DefaultOptions.Timeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), opt.Timeout)
	t.Cleanup(cancel)

	launchOpts := chromium.DefaultLaunchOptions()
	launchOpts.Headless = opt.Headless
	if opt.SlowMo > 0 {
		launchOpts.SlowMo = opt.SlowMo
	}

	bt := browser.Chromium()
	b, err := bt.Launch(ctx, launchOpts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	return &Fixture{
		T:       t,
		Browser: b,
		Context: bc,
		Page:    page,
		opts:    opt,
	}
}

func (f *Fixture) NewPage() *browser.Page {
	f.T.Helper()
	page, err := f.Context.NewPage()
	if err != nil {
		f.T.Fatal(err)
	}
	return page
}

func (f *Fixture) NewContext() *browser.BrowserContext {
	f.T.Helper()
	bc, err := f.Browser.NewContext()
	if err != nil {
		f.T.Fatal(err)
	}
	f.T.Cleanup(func() { bc.Close() })
	return bc
}

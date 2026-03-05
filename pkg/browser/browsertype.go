package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/PeterStoica/gowright/pkg/browser/chromium"
)

type BrowserType struct {
	name string
}

func NewBrowserType(name string) *BrowserType {
	return &BrowserType{name: name}
}

func Chromium() *BrowserType {
	return &BrowserType{name: "chromium"}
}

func (bt *BrowserType) Name() string { return bt.name }

func (bt *BrowserType) ExecutablePath() (string, error) {
	return chromium.FindChrome()
}

func (bt *BrowserType) Launch(ctx context.Context, opts ...chromium.LaunchOptions) (*Browser, error) {
	opt := chromium.DefaultLaunchOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}

	result, err := chromium.Launch(ctx, opt)
	if err != nil {
		return nil, err
	}

	b, err := Connect(ctx, result.WSEndpoint)
	if err != nil {
		result.Cleanup()
		return nil, err
	}
	b.SetCleanup(result.Cleanup)
	if opt.SlowMo > 0 {
		b.SetSlowMo(opt.SlowMo)
	}
	return b, nil
}

func (bt *BrowserType) Connect(ctx context.Context, wsEndpoint string) (*Browser, error) {
	return Connect(ctx, wsEndpoint)
}

func (bt *BrowserType) ConnectOverCDP(ctx context.Context, endpointURL string) (*Browser, error) {
	return Connect(ctx, endpointURL)
}

func (bt *BrowserType) LaunchPersistentContext(ctx context.Context, userDataDir string, opts ...chromium.LaunchOptions) (*BrowserContext, error) {
	opt := chromium.DefaultLaunchOptions()
	if len(opts) > 0 {
		opt = opts[0]
	}
	opt.UserDataDir = userDataDir

	result, err := chromium.Launch(ctx, opt)
	if err != nil {
		return nil, err
	}

	b, err := Connect(ctx, result.WSEndpoint)
	if err != nil {
		result.Cleanup()
		return nil, err
	}
	b.SetCleanup(result.Cleanup)

	bc, err := b.NewContext()
	if err != nil {
		b.Close()
		return nil, err
	}
	return bc, nil
}

type Selectors struct {
	mu              sync.RWMutex
	testIdAttribute string
	engines         map[string]SelectorEngine
}

type SelectorEngine struct {
	QueryAll string
}

var globalSelectors = &Selectors{
	testIdAttribute: "data-testid",
	engines:         make(map[string]SelectorEngine),
}

func GetSelectors() *Selectors {
	return globalSelectors
}

func (s *Selectors) Register(name string, engine SelectorEngine) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.engines[name]; exists {
		return fmt.Errorf("selector engine %q already registered", name)
	}
	s.engines[name] = engine
	return nil
}

func (s *Selectors) SetTestIdAttribute(attr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.testIdAttribute = attr
}

func (s *Selectors) TestIdAttribute() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.testIdAttribute
}

func (s *Selectors) engine(name string) (SelectorEngine, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.engines[name]
	return e, ok
}

func (p *Page) DragAndDrop(source, target string) error {
	return p.Locator(source).DragTo(p.Locator(target))
}

func (p *Page) Tap(selector string) error {
	return p.Locator(selector).Tap()
}

func (p *Page) SetInputFiles(selector string, files ...string) error {
	return p.Locator(selector).SetInputFiles(files...)
}

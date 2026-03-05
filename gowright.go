package gowright

import (
	"context"
	"sync"

	"github.com/QA-DNA/gowright/pkg/browser"
	"github.com/QA-DNA/gowright/pkg/browser/chromium"
	"github.com/QA-DNA/gowright/pkg/cdp"
	"github.com/QA-DNA/gowright/pkg/config"
)

var _ = (*cdp.Conn)(nil)

type Browser = browser.Browser
type BrowserContext = browser.BrowserContext
type Page = browser.Page
type Locator = browser.Locator
type ByRoleOption = browser.ByRoleOption
type Dialog = browser.Dialog
type ScreenshotOptions = browser.ScreenshotOptions
type Rect = browser.Rect
type WaitForSelectorOptions = browser.WaitForSelectorOptions
type Route = browser.Route
type Request = browser.Request
type Response = browser.Response
type FulfillOptions = browser.FulfillOptions
type Cookie = browser.Cookie
type BoundingBox = browser.BoundingBox
type Config = config.Config
type Viewport = config.Viewport
type LaunchOptions = chromium.LaunchOptions

type Frame = browser.Frame
type FrameLocator = browser.FrameLocator
type Keyboard = browser.Keyboard
type Mouse = browser.Mouse
type Touchscreen = browser.Touchscreen
type ConsoleMessage = browser.ConsoleMessage
type ConsoleMessageLocation = browser.ConsoleMessageLocation
type FileChooser = browser.FileChooser
type Download = browser.Download
type WebSocket = browser.WebSocket
type WebSocketFrame = browser.WebSocketFrame
type Worker = browser.Worker
type Tracing = browser.Tracing
type Coverage = browser.Coverage
type CoverageEntry = browser.CoverageEntry
type CoverageRange = browser.CoverageRange
type JSHandle = browser.JSHandle
type BrowserType = browser.BrowserType
type Selectors = browser.Selectors
type SelectorEngine = browser.SelectorEngine
type StorageState = browser.StorageState
type OriginState = browser.OriginState
type Geolocation = browser.Geolocation
type SecurityDetails = browser.SecurityDetails
type ServerAddr = browser.ServerAddr
type Position = browser.Position

type WaitForOptions = browser.WaitForOptions
type FilterOptions = browser.FilterOptions
type WaitForFunctionOptions = browser.WaitForFunctionOptions
type WaitForPopupOptions = browser.WaitForPopupOptions
type AddScriptTagOptions = browser.AddScriptTagOptions
type AddStyleTagOptions = browser.AddStyleTagOptions
type EmulateMediaOptions = browser.EmulateMediaOptions
type PDFOptions = browser.PDFOptions
type PressSequentiallyOptions = browser.PressSequentiallyOptions
type DragToOptions = browser.DragToOptions
type ClickOptions = browser.ClickOptions
type FillOptions = browser.FillOptions
type GrantPermissionsOptions = browser.GrantPermissionsOptions
type ContinueOptions = browser.ContinueOptions
type RouteFetchOptions = browser.RouteFetchOptions
type TracingStartOptions = browser.TracingStartOptions
type JSCoverageOptions = browser.JSCoverageOptions
type CSSCoverageOptions = browser.CSSCoverageOptions
type MouseClickOptions = browser.MouseClickOptions
type MouseMoveOptions = browser.MouseMoveOptions
type MouseDownOptions = browser.MouseDownOptions
type MouseUpOptions = browser.MouseUpOptions
type KeyPressOptions = browser.KeyPressOptions
type KeyTypeOptions = browser.KeyTypeOptions
type Clock = browser.Clock
type ClockInstallOptions = browser.ClockInstallOptions
type Accessibility = browser.Accessibility
type AccessibilityNode = browser.AccessibilityNode
type AccessibilitySnapshotOptions = browser.AccessibilitySnapshotOptions
type Video = browser.Video
type VideoOptions = browser.VideoOptions
type VideoSize = browser.VideoSize
type WaitForNavigationOptions = browser.WaitForNavigationOptions
type ConnectOptions = browser.ConnectOptions
type Credentials = browser.Credentials
type HarRecorder = browser.HarRecorder
type HarEntry = browser.HarEntry
type HarRequest = browser.HarRequest
type HarResponse = browser.HarResponse
type RequestTiming = browser.RequestTiming
type RequestSizes = browser.RequestSizes
type WaitForEventOptions = browser.WaitForEventOptions
type WaitForURLOptions = browser.WaitForURLOptions
type HarResponseContent = browser.HarResponseContent
type RouteFromHAROptions = browser.RouteFromHAROptions

func Chromium() *BrowserType {
	return browser.Chromium()
}

func GetSelectors() *Selectors {
	return browser.GetSelectors()
}

var (
	loadOnce   sync.Once
	loadedConf config.Config
)

func loadConfig() config.Config {
	loadOnce.Do(func() {
		loadedConf, _ = config.Load()
	})
	return loadedConf
}

func LoadConfig(dir ...string) (Config, error) {
	return config.Load(dir...)
}

func Launch(ctx context.Context, opts ...LaunchOptions) (*Browser, error) {
	cfg := loadConfig()

	opt := chromium.DefaultLaunchOptions()
	if len(opts) > 0 {
		opt = opts[0]
	} else {
		opt.Headless = cfg.IsHeadless()
		opt.SlowMo = cfg.SlowMoDuration()
		opt.Args = cfg.LaunchArgs
		opt.NoSandbox = cfg.NoSandbox
	}

	result, err := chromium.Launch(ctx, opt)
	if err != nil {
		return nil, err
	}

	b, err := browser.Connect(ctx, result.WSEndpoint)
	if err != nil {
		result.Cleanup()
		return nil, err
	}

	b.SetCleanup(result.Cleanup)
	if opt.SlowMo > 0 {
		b.SetSlowMo(opt.SlowMo)
	}

	if cfg.Viewport != nil {
		b.SetDefaultViewport(cfg.Viewport.Width, cfg.Viewport.Height)
	}

	return b, nil
}

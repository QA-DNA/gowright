package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

type Mouse struct {
	session  *cdp.Session
	x, y     float64
	button   string
	buttons  int
	mu       sync.Mutex
}

func newMouse(session *cdp.Session) *Mouse {
	return &Mouse{session: session}
}

func (m *Mouse) Move(ctx context.Context, x, y float64, opts ...MouseMoveOptions) error {
	steps := 1
	if len(opts) > 0 && opts[0].Steps > 0 {
		steps = opts[0].Steps
	}

	startX, startY := m.x, m.y
	for i := 1; i <= steps; i++ {
		ix := startX + (x-startX)*float64(i)/float64(steps)
		iy := startY + (y-startY)*float64(i)/float64(steps)
		_, err := m.session.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
			"type":    "mouseMoved",
			"x":      ix,
			"y":      iy,
			"buttons": m.buttons,
		})
		if err != nil {
			return fmt.Errorf("mouse move: %w", err)
		}
	}
	m.mu.Lock()
	m.x = x
	m.y = y
	m.mu.Unlock()
	return nil
}

func (m *Mouse) Click(ctx context.Context, x, y float64, opts ...MouseClickOptions) error {
	o := MouseClickOptions{Button: "left", ClickCount: 1}
	if len(opts) > 0 {
		if opts[0].Button != "" {
			o.Button = opts[0].Button
		}
		if opts[0].ClickCount > 0 {
			o.ClickCount = opts[0].ClickCount
		}
		o.Delay = opts[0].Delay
	}

	if err := m.Move(ctx, x, y); err != nil {
		return err
	}
	if err := m.Down(ctx, MouseDownOptions{Button: o.Button, ClickCount: o.ClickCount}); err != nil {
		return err
	}
	if o.Delay > 0 {
		time.Sleep(o.Delay)
	}
	return m.Up(ctx, MouseUpOptions{Button: o.Button, ClickCount: o.ClickCount})
}

func (m *Mouse) DblClick(ctx context.Context, x, y float64, opts ...MouseClickOptions) error {
	if err := m.Click(ctx, x, y); err != nil {
		return err
	}
	o := MouseClickOptions{Button: "left"}
	if len(opts) > 0 && opts[0].Button != "" {
		o.Button = opts[0].Button
	}
	return m.Click(ctx, x, y, MouseClickOptions{Button: o.Button, ClickCount: 2})
}

func (m *Mouse) Down(ctx context.Context, opts ...MouseDownOptions) error {
	button := "left"
	clickCount := 1
	if len(opts) > 0 {
		if opts[0].Button != "" {
			button = opts[0].Button
		}
		if opts[0].ClickCount > 0 {
			clickCount = opts[0].ClickCount
		}
	}
	m.mu.Lock()
	m.button = button
	m.buttons |= buttonBit(button)
	buttons := m.buttons
	m.mu.Unlock()

	_, err := m.session.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":       "mousePressed",
		"x":          m.x,
		"y":          m.y,
		"button":     button,
		"buttons":    buttons,
		"clickCount": clickCount,
	})
	if err != nil {
		return fmt.Errorf("mouse down: %w", err)
	}
	return nil
}

func (m *Mouse) Up(ctx context.Context, opts ...MouseUpOptions) error {
	button := "left"
	clickCount := 1
	if len(opts) > 0 {
		if opts[0].Button != "" {
			button = opts[0].Button
		}
		if opts[0].ClickCount > 0 {
			clickCount = opts[0].ClickCount
		}
	}
	m.mu.Lock()
	m.buttons &^= buttonBit(button)
	buttons := m.buttons
	m.mu.Unlock()

	_, err := m.session.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":       "mouseReleased",
		"x":          m.x,
		"y":          m.y,
		"button":     button,
		"buttons":    buttons,
		"clickCount": clickCount,
	})
	if err != nil {
		return fmt.Errorf("mouse up: %w", err)
	}
	return nil
}

func (m *Mouse) Wheel(ctx context.Context, deltaX, deltaY float64) error {
	_, err := m.session.Call(ctx, "Input.dispatchMouseEvent", map[string]any{
		"type":   "mouseWheel",
		"x":      m.x,
		"y":      m.y,
		"deltaX": deltaX,
		"deltaY": deltaY,
	})
	if err != nil {
		return fmt.Errorf("mouse wheel: %w", err)
	}
	return nil
}

type MouseMoveOptions struct {
	Steps int
}

type MouseClickOptions struct {
	Button     string
	ClickCount int
	Delay      time.Duration
}

type MouseDownOptions struct {
	Button     string
	ClickCount int
}

type MouseUpOptions struct {
	Button     string
	ClickCount int
}

func buttonBit(button string) int {
	switch button {
	case "left":
		return 1
	case "right":
		return 2
	case "middle":
		return 4
	default:
		return 1
	}
}

type Keyboard struct {
	session   *cdp.Session
	mu        sync.Mutex
	modifiers int
	pressed   map[string]bool
}

func newKeyboard(session *cdp.Session) *Keyboard {
	return &Keyboard{session: session, pressed: make(map[string]bool)}
}

func (k *Keyboard) Down(ctx context.Context, key string) error {
	desc := keyDescriptionForString(key)
	k.mu.Lock()
	k.pressed[key] = true
	k.modifiers |= modifierBit(key)
	modifiers := k.modifiers
	k.mu.Unlock()

	_, err := k.session.Call(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type":                  ternary(desc.text != "", "keyDown", "rawKeyDown"),
		"key":                   desc.key,
		"code":                  desc.code,
		"text":                  desc.text,
		"unmodifiedText":        desc.text,
		"windowsVirtualKeyCode": desc.keyCode,
		"modifiers":             modifiers,
	})
	if err != nil {
		return fmt.Errorf("key down: %w", err)
	}
	return nil
}

func (k *Keyboard) Up(ctx context.Context, key string) error {
	desc := keyDescriptionForString(key)
	k.mu.Lock()
	delete(k.pressed, key)
	k.modifiers &^= modifierBit(key)
	modifiers := k.modifiers
	k.mu.Unlock()

	_, err := k.session.Call(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type":                  "keyUp",
		"key":                   desc.key,
		"code":                  desc.code,
		"windowsVirtualKeyCode": desc.keyCode,
		"modifiers":             modifiers,
	})
	if err != nil {
		return fmt.Errorf("key up: %w", err)
	}
	return nil
}

func (k *Keyboard) Press(ctx context.Context, key string, opts ...KeyPressOptions) error {
	var delay time.Duration
	if len(opts) > 0 {
		delay = opts[0].Delay
	}
	if err := k.Down(ctx, key); err != nil {
		return err
	}
	if delay > 0 {
		time.Sleep(delay)
	}
	return k.Up(ctx, key)
}

func (k *Keyboard) Type(ctx context.Context, text string, opts ...KeyTypeOptions) error {
	var delay time.Duration
	if len(opts) > 0 {
		delay = opts[0].Delay
	}
	for _, ch := range text {
		if delay > 0 {
			time.Sleep(delay)
		}
		s := string(ch)
		desc := keyDescriptionForString(s)
		// If the character has a known key code (US keyboard layout), use keyDown/keyUp
		// like Playwright does. Otherwise fall back to Input.insertText.
		if desc.keyCode != 0 && desc.text != "" {
			if err := k.Press(ctx, s); err != nil {
				return fmt.Errorf("type text: %w", err)
			}
		} else {
			_, err := k.session.Call(ctx, "Input.insertText", map[string]any{
				"text": s,
			})
			if err != nil {
				return fmt.Errorf("type text: %w", err)
			}
		}
	}
	return nil
}

func (k *Keyboard) InsertText(ctx context.Context, text string) error {
	_, err := k.session.Call(ctx, "Input.insertText", map[string]any{
		"text": text,
	})
	if err != nil {
		return fmt.Errorf("insert text: %w", err)
	}
	return nil
}

type KeyPressOptions struct {
	Delay time.Duration
}

type KeyTypeOptions struct {
	Delay time.Duration
}

func modifierBit(key string) int {
	switch key {
	case "Alt":
		return 1
	case "Control":
		return 2
	case "Meta":
		return 4
	case "Shift":
		return 8
	default:
		return 0
	}
}

type Touchscreen struct {
	session *cdp.Session
}

func newTouchscreen(session *cdp.Session) *Touchscreen {
	return &Touchscreen{session: session}
}

func (t *Touchscreen) Tap(ctx context.Context, x, y float64) error {
	_, err := t.session.Call(ctx, "Input.dispatchTouchEvent", map[string]any{
		"type": "touchStart",
		"touchPoints": []map[string]any{
			{"x": x, "y": y},
		},
	})
	if err != nil {
		return fmt.Errorf("touch start: %w", err)
	}
	_, err = t.session.Call(ctx, "Input.dispatchTouchEvent", map[string]any{
		"type":        "touchEnd",
		"touchPoints": []map[string]any{},
	})
	if err != nil {
		return fmt.Errorf("touch end: %w", err)
	}
	return nil
}

func (t *Touchscreen) TouchStart(ctx context.Context, x, y float64) error {
	_, err := t.session.Call(ctx, "Input.dispatchTouchEvent", map[string]any{
		"type": "touchStart",
		"touchPoints": []map[string]any{
			{"x": x, "y": y},
		},
	})
	return err
}

func (t *Touchscreen) TouchMove(ctx context.Context, x, y float64) error {
	_, err := t.session.Call(ctx, "Input.dispatchTouchEvent", map[string]any{
		"type": "touchMove",
		"touchPoints": []map[string]any{
			{"x": x, "y": y},
		},
	})
	return err
}

func (t *Touchscreen) TouchEnd(ctx context.Context) error {
	_, err := t.session.Call(ctx, "Input.dispatchTouchEvent", map[string]any{
		"type":        "touchEnd",
		"touchPoints": []map[string]any{},
	})
	return err
}

func (p *Page) Keyboard() *Keyboard {
	if p.keyboard == nil {
		p.keyboard = newKeyboard(p.session)
	}
	return p.keyboard
}

func (p *Page) Mouse() *Mouse {
	if p.mouse == nil {
		p.mouse = newMouse(p.session)
	}
	return p.mouse
}

func (p *Page) Touchscreen() *Touchscreen {
	if p.touchscreen == nil {
		p.touchscreen = newTouchscreen(p.session)
	}
	return p.touchscreen
}

type keyDescription struct {
	key     string
	code    string
	text    string
	keyCode int
}

func keyDescriptionForString(key string) keyDescription {
	switch key {
	case "Enter":
		return keyDescription{key: "Enter", code: "Enter", text: "\r", keyCode: 13}
	case "Tab":
		return keyDescription{key: "Tab", code: "Tab", text: "", keyCode: 9}
	case "Backspace":
		return keyDescription{key: "Backspace", code: "Backspace", text: "", keyCode: 8}
	case "Delete":
		return keyDescription{key: "Delete", code: "Delete", text: "", keyCode: 46}
	case "Escape":
		return keyDescription{key: "Escape", code: "Escape", text: "", keyCode: 27}
	case "ArrowUp":
		return keyDescription{key: "ArrowUp", code: "ArrowUp", text: "", keyCode: 38}
	case "ArrowDown":
		return keyDescription{key: "ArrowDown", code: "ArrowDown", text: "", keyCode: 40}
	case "ArrowLeft":
		return keyDescription{key: "ArrowLeft", code: "ArrowLeft", text: "", keyCode: 37}
	case "ArrowRight":
		return keyDescription{key: "ArrowRight", code: "ArrowRight", text: "", keyCode: 39}
	case "Home":
		return keyDescription{key: "Home", code: "Home", text: "", keyCode: 36}
	case "End":
		return keyDescription{key: "End", code: "End", text: "", keyCode: 35}
	case "PageUp":
		return keyDescription{key: "PageUp", code: "PageUp", text: "", keyCode: 33}
	case "PageDown":
		return keyDescription{key: "PageDown", code: "PageDown", text: "", keyCode: 34}
	case "Control":
		return keyDescription{key: "Control", code: "ControlLeft", text: "", keyCode: 17}
	case "Shift":
		return keyDescription{key: "Shift", code: "ShiftLeft", text: "", keyCode: 16}
	case "Alt":
		return keyDescription{key: "Alt", code: "AltLeft", text: "", keyCode: 18}
	case "Meta":
		return keyDescription{key: "Meta", code: "MetaLeft", text: "", keyCode: 91}
	case "F1":
		return keyDescription{key: "F1", code: "F1", text: "", keyCode: 112}
	case "F2":
		return keyDescription{key: "F2", code: "F2", text: "", keyCode: 113}
	case "F3":
		return keyDescription{key: "F3", code: "F3", text: "", keyCode: 114}
	case "F4":
		return keyDescription{key: "F4", code: "F4", text: "", keyCode: 115}
	case "F5":
		return keyDescription{key: "F5", code: "F5", text: "", keyCode: 116}
	case "F6":
		return keyDescription{key: "F6", code: "F6", text: "", keyCode: 117}
	case "F7":
		return keyDescription{key: "F7", code: "F7", text: "", keyCode: 118}
	case "F8":
		return keyDescription{key: "F8", code: "F8", text: "", keyCode: 119}
	case "F9":
		return keyDescription{key: "F9", code: "F9", text: "", keyCode: 120}
	case "F10":
		return keyDescription{key: "F10", code: "F10", text: "", keyCode: 121}
	case "F11":
		return keyDescription{key: "F11", code: "F11", text: "", keyCode: 122}
	case "F12":
		return keyDescription{key: "F12", code: "F12", text: "", keyCode: 123}
	case " ":
		return keyDescription{key: " ", code: "Space", text: " ", keyCode: 32}
	default:
		if len(key) == 1 {
			c := key[0]
			code := "Key" + string(c&^0x20)
			return keyDescription{key: key, code: code, text: key, keyCode: int(c & ^byte(0x20))}
		}
		return keyDescription{key: key, code: key, text: "", keyCode: 0}
	}
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func evaluateHandle(ctx context.Context, session *cdp.Session, expression string) (string, error) {
	result, err := session.Call(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": false,
	})
	if err != nil {
		return "", err
	}

	var resp struct {
		Result struct {
			ObjectID string `json:"objectId"`
			Type     string `json:"type"`
			Subtype  string `json:"subtype"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return "", err
	}
	if resp.ExceptionDetails != nil {
		return "", fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}
	if resp.Result.Subtype == "null" || resp.Result.ObjectID == "" {
		return "", nil
	}
	return resp.Result.ObjectID, nil
}

func getBoxModel(ctx context.Context, session *cdp.Session, objectID string) (x, y float64, err error) {
	result, err := session.Call(ctx, "DOM.getBoxModel", map[string]any{
		"objectId": objectID,
	})
	if err != nil {
		return 0, 0, fmt.Errorf("get box model: %w", err)
	}

	var resp struct {
		Model struct {
			Content []float64 `json:"content"`
		} `json:"model"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return 0, 0, err
	}

	pts := resp.Model.Content
	if len(pts) < 8 {
		return 0, 0, fmt.Errorf("invalid box model: %v", pts)
	}

	cx := (pts[0] + pts[2] + pts[4] + pts[6]) / 4
	cy := (pts[1] + pts[3] + pts[5] + pts[7]) / 4
	return cx, cy, nil
}

func scrollIntoView(ctx context.Context, session *cdp.Session, objectID string) error {
	_, err := session.Call(ctx, "DOM.scrollIntoViewIfNeeded", map[string]any{
		"objectId": objectID,
	})
	return err
}

func focusElement(ctx context.Context, session *cdp.Session, objectID string) error {
	_, err := session.Call(ctx, "DOM.focus", map[string]any{
		"objectId": objectID,
	})
	return err
}

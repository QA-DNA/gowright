package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PeterStoica/gowright/pkg/cdp"
)

type Locator struct {
	page        *Page
	selector    string
	selectorAll string
	desc        string
	frame       *Frame
	strict      bool
}

// visibilityFilterJS is the JS function Playwright uses to check if an element is visible.
// Checks computed style and bounding rect, matching Playwright's isElementVisible().
const visibilityFilterJS = `function(el) {
	var style = window.getComputedStyle(el);
	if (style.display === 'none' || style.visibility === 'hidden' || style.opacity === '0') return false;
	if (style.display === 'contents') {
		for (var c = el.firstChild; c; c = c.nextSibling) {
			if (c.nodeType === 1 && arguments.callee(c)) return true;
			if (c.nodeType === 3 && c.textContent.trim()) {
				var r = document.createRange(); r.selectNodeContents(c);
				var rects = r.getClientRects();
				for (var i = 0; i < rects.length; i++) { if (rects[i].width > 0 && rects[i].height > 0) return true; }
			}
		}
		return false;
	}
	var rect = el.getBoundingClientRect();
	return rect.width > 0 && rect.height > 0;
}`

// stripVisiblePseudo strips the :visible pseudo-class from a CSS selector
// and returns the clean CSS + whether :visible was present.
// Like Playwright, :visible is not standard CSS — it's intercepted and handled in JS.
func stripVisiblePseudo(selector string) (string, bool) {
	// Simple approach: find and remove ":visible" from the selector
	if !strings.Contains(selector, ":visible") {
		return selector, false
	}
	clean := strings.ReplaceAll(selector, ":visible", "")
	return strings.TrimSpace(clean), true
}

// buildLocatorJS builds querySelector/querySelectorAll JS expressions,
// handling the :visible pseudo-selector like Playwright does.
func buildLocatorJS(selector string) (js, jsAll string, hasVisible bool) {
	cleanCSS, hasVisible := stripVisiblePseudo(selector)
	if hasVisible {
		// Playwright pattern: querySelectorAll(cleanCSS) then filter by visibility
		isVis := visibilityFilterJS
		js = fmt.Sprintf(`(function(){
			var isVis = %s;
			var els = document.querySelectorAll(%s);
			for (var i = 0; i < els.length; i++) { if (isVis(els[i])) return els[i]; }
			return null;
		})()`, isVis, jsQuote(cleanCSS))
		jsAll = fmt.Sprintf(`(function(){
			var isVis = %s;
			var els = document.querySelectorAll(%s), results = [];
			for (var i = 0; i < els.length; i++) { if (isVis(els[i])) results.push(els[i]); }
			return results;
		})()`, isVis, jsQuote(cleanCSS))
	} else {
		js = fmt.Sprintf(`document.querySelector(%s)`, jsQuote(cleanCSS))
		jsAll = fmt.Sprintf(`document.querySelectorAll(%s)`, jsQuote(cleanCSS))
	}
	return js, jsAll, hasVisible
}

// Locator creates a locator. Supports CSS selectors, custom engine selectors
// in the form "engine=value", and the :visible pseudo-selector (like Playwright).
func (p *Page) Locator(selector string) *Locator {
	if idx := strings.Index(selector, "="); idx > 0 {
		engineName := selector[:idx]
		if engine, ok := globalSelectors.engine(engineName); ok {
			value := selector[idx+1:]
			js := fmt.Sprintf(`(function(){ var results = (function(root, selector) { %s })(document.body, %s); return results && results.length > 0 ? results[0] : null; })()`, engine.QueryAll, jsQuote(value))
			jsAll := fmt.Sprintf(`(function(root, selector) { %s })(document.body, %s)`, engine.QueryAll, jsQuote(value))
			return &Locator{page: p, selector: js, selectorAll: jsAll, desc: selector, strict: true}
		}
	}
	js, jsAll, _ := buildLocatorJS(selector)
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: selector, strict: true}
}

// GetByText finds an element by its text content.
func (p *Page) GetByText(text string, exact ...bool) *Locator {
	isExact := len(exact) > 0 && exact[0]
	var js, jsAll string
	if isExact {
		js = fmt.Sprintf(`
			(function() {
				const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
				while (walker.nextNode()) {
					if (walker.currentNode.textContent.trim() === %s) return walker.currentNode.parentElement;
				}
				return null;
			})()`, jsQuote(text))
		jsAll = fmt.Sprintf(`
			(function() {
				var results = [], walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
				while (walker.nextNode()) {
					if (walker.currentNode.textContent.trim() === %s) results.push(walker.currentNode.parentElement);
				}
				return results;
			})()`, jsQuote(text))
	} else {
		js = fmt.Sprintf(`
			(function() {
				const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
				const lower = %s.toLowerCase();
				while (walker.nextNode()) {
					if (walker.currentNode.textContent.toLowerCase().includes(lower)) return walker.currentNode.parentElement;
				}
				return null;
			})()`, jsQuote(text))
		jsAll = fmt.Sprintf(`
			(function() {
				var results = [], walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
				var lower = %s.toLowerCase();
				while (walker.nextNode()) {
					if (walker.currentNode.textContent.toLowerCase().includes(lower)) results.push(walker.currentNode.parentElement);
				}
				return results;
			})()`, jsQuote(text))
	}
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByText(%q)", text), strict: true}
}

// GetByRole finds an element by its ARIA role.
func (p *Page) GetByRole(role string, opts ...ByRoleOption) *Locator {
	var nameFilter string
	if len(opts) > 0 && opts[0].Name != "" {
		nameFilter = opts[0].Name
	}

	cssAll := fmt.Sprintf(`[role=%s], %s`, jsQuote(role), implicitRoleSelector(role))
	var js, jsAll string
	if nameFilter != "" {
		js = fmt.Sprintf(`
			(function() {
				const els = document.querySelectorAll('%s');
				const name = %s.toLowerCase();
				for (const el of els) {
					const label = (el.getAttribute('aria-label') || el.textContent || '').toLowerCase();
					if (label.includes(name)) return el;
				}
				return null;
			})()`, cssAll, jsQuote(nameFilter))
		jsAll = fmt.Sprintf(`
			(function() {
				var els = document.querySelectorAll('%s');
				var name = %s.toLowerCase(), results = [];
				for (var i = 0; i < els.length; i++) {
					var label = (els[i].getAttribute('aria-label') || els[i].textContent || '').toLowerCase();
					if (label.includes(name)) results.push(els[i]);
				}
				return results;
			})()`, cssAll, jsQuote(nameFilter))
	} else {
		js = fmt.Sprintf(`document.querySelector('%s')`, cssAll)
		jsAll = fmt.Sprintf(`document.querySelectorAll('%s')`, cssAll)
	}
	desc := fmt.Sprintf("getByRole(%q)", role)
	if nameFilter != "" {
		desc = fmt.Sprintf("getByRole(%q, {name: %q})", role, nameFilter)
	}
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: desc, strict: true}
}

// ByRoleOption configures GetByRole.
type ByRoleOption struct {
	Name string
}

// GetByTestId finds an element by data-testid attribute.
func (p *Page) GetByTestId(testID string) *Locator {
	attr := globalSelectors.TestIdAttribute()
	css := fmt.Sprintf(`[%s=%s]`, attr, jsQuote(testID))
	js := fmt.Sprintf(`document.querySelector('%s')`, css)
	jsAll := fmt.Sprintf(`document.querySelectorAll('%s')`, css)
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByTestId(%q)", testID), strict: true}
}

// GetByLabel finds a form element by its associated label text.
func (p *Page) GetByLabel(text string) *Locator {
	js := fmt.Sprintf(`
		(function() {
			const labels = document.querySelectorAll('label');
			const target = %s.toLowerCase();
			for (const label of labels) {
				if (label.textContent.toLowerCase().includes(target)) {
					if (label.htmlFor) return document.getElementById(label.htmlFor);
					return label.querySelector('input, textarea, select');
				}
			}
			return null;
		})()`, jsQuote(text))
	jsAll := fmt.Sprintf(`
		(function() {
			var labels = document.querySelectorAll('label');
			var target = %s.toLowerCase(), results = [];
			for (var i = 0; i < labels.length; i++) {
				if (labels[i].textContent.toLowerCase().includes(target)) {
					if (labels[i].htmlFor) { var el = document.getElementById(labels[i].htmlFor); if (el) results.push(el); }
					else { var el = labels[i].querySelector('input, textarea, select'); if (el) results.push(el); }
				}
			}
			return results;
		})()`, jsQuote(text))
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByLabel(%q)", text), strict: true}
}

// GetByPlaceholder finds an input by its placeholder text.
func (p *Page) GetByPlaceholder(text string) *Locator {
	css := fmt.Sprintf(`[placeholder=%s]`, jsQuote(text))
	js := fmt.Sprintf(`document.querySelector('%s')`, css)
	jsAll := fmt.Sprintf(`document.querySelectorAll('%s')`, css)
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByPlaceholder(%q)", text), strict: true}
}

// Locator chains a child selector onto this locator.
func (l *Locator) Locator(selector string) *Locator {
	parentJS := l.selector
	cleanCSS, hasVisible := stripVisiblePseudo(selector)
	var js string
	if hasVisible {
		isVis := visibilityFilterJS
		js = fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; var isVis = %s; var els = root.querySelectorAll(%s); for (var i = 0; i < els.length; i++) { if (isVis(els[i])) return els[i]; } return null; })()`,
			parentJS, isVis, jsQuote(cleanCSS))
	} else {
		js = fmt.Sprintf(`(function(){ const el = %s; if (!el) return null; return el.querySelector(%s); })()`,
			parentJS, jsQuote(cleanCSS))
	}
	return &Locator{
		page:     l.page,
		selector: js,
		desc:     l.desc + " >> " + selector,
	}
}

// First returns a locator for the first match (same as default behavior, but explicit).
func (l *Locator) First() *Locator {
	cp := *l
	cp.strict = false
	return &cp
}

// Nth returns a locator for the nth matching element (0-indexed).
func (l *Locator) Nth(index int) *Locator {
	// Use the parent's selectorAll JS (which handles :visible etc.) instead of
	// trying to querySelectorAll on the description string.
	allJS := l.selectorAll
	if allJS == "" {
		allJS = l.selector
	}
	js := fmt.Sprintf(`(function(){ var all = %s; if (!all) return null; if (!all.length && all.length !== 0) return (0 === %d) ? all : null; return all[%d] || null; })()`,
		allJS, index, index)
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf("%s.nth(%d)", l.desc, index), frame: l.frame}
}

// --- Actions (all auto-wait) ---

// Click clicks the element. Waits for it to be visible and enabled.
func (l *Locator) Click() error {
	return l.withElement("click", func(ctx context.Context, objectID string) error {
		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}
		x, y, err := getBoxModel(ctx, l.page.session, objectID)
		if err != nil {
			return err
		}
		mouse := newMouse(l.page.session)
		return mouse.Click(ctx, x, y)
	})
}

// DblClick double-clicks the element.
func (l *Locator) DblClick() error {
	return l.withElement("dblclick", func(ctx context.Context, objectID string) error {
		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}
		x, y, err := getBoxModel(ctx, l.page.session, objectID)
		if err != nil {
			return err
		}
		mouse := newMouse(l.page.session)
		return mouse.DblClick(ctx, x, y)
	})
}

// Fill clears the input and types new text.
// Matches Playwright's approach: focus → select all → Input.insertText CDP command.
// This works correctly with React/Vue/Angular controlled inputs.
func (l *Locator) Fill(value string) error {
	return l.withElement("fill", func(ctx context.Context, objectID string) error {
		// Verify the element is an input, textarea, or contenteditable
		_, err := callFunctionOn(ctx, l.page.session, objectID, `function() {
			const tag = this.tagName.toLowerCase();
			if (tag !== 'input' && tag !== 'textarea' && !this.isContentEditable) {
				throw new Error('Element is not an <input>, <textarea> or [contenteditable] element');
			}
			if (tag === 'input') {
				const type = (this.getAttribute('type') || '').toLowerCase();
				const fillable = new Set(['', 'email', 'number', 'password', 'search', 'tel', 'text', 'url']);
				if (!fillable.has(type)) {
					throw new Error('Input of type "' + type + '" cannot be filled');
				}
			}
		}`, "")
		if err != nil {
			return err
		}

		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}

		// Focus and select all existing text (Playwright pattern)
		if err := focusElement(ctx, l.page.session, objectID); err != nil {
			return err
		}
		_, err = callFunctionOn(ctx, l.page.session, objectID, `function() {
			if (this.select) {
				this.select();
			} else if (this.isContentEditable) {
				var range = document.createRange();
				range.selectNodeContents(this);
				var sel = window.getSelection();
				sel.removeAllRanges();
				sel.addRange(range);
			}
			this.focus();
		}`, "")
		if err != nil {
			return err
		}

		if value == "" {
			// Delete selected text via keyboard
			kb := newKeyboard(l.page.session)
			return kb.Press(ctx, "Delete")
		}

		// Use Input.insertText CDP command — this is what Playwright uses.
		// It triggers proper browser-level input events that React/Vue pick up.
		_, err = l.page.session.Call(ctx, "Input.insertText", map[string]any{
			"text": value,
		})
		return err
	})
}

// Type types text into the element character by character.
func (l *Locator) Type(text string) error {
	return l.withElement("type", func(ctx context.Context, objectID string) error {
		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}
		if err := focusElement(ctx, l.page.session, objectID); err != nil {
			return err
		}
		kb := newKeyboard(l.page.session)
		return kb.Type(ctx, text)
	})
}

// Press presses a keyboard key while the element is focused.
func (l *Locator) Press(key string) error {
	return l.withElement("press", func(ctx context.Context, objectID string) error {
		if err := focusElement(ctx, l.page.session, objectID); err != nil {
			return err
		}
		kb := newKeyboard(l.page.session)
		return kb.Press(ctx, key)
	})
}

// SelectOption selects an <option> by value in a <select> element.
func (l *Locator) SelectOption(values ...string) error {
	return l.withElement("selectOption", func(ctx context.Context, objectID string) error {
		valuesJSON, _ := json.Marshal(values)
		_, err := callFunctionOn(ctx, l.page.session, objectID, `
			function(values) {
				const options = Array.from(this.options);
				this.value = '';
				for (const opt of options) {
					opt.selected = values.includes(opt.value) || values.includes(opt.textContent.trim());
				}
				this.dispatchEvent(new Event('input', {bubbles: true}));
				this.dispatchEvent(new Event('change', {bubbles: true}));
			}
		`, string(valuesJSON))
		return err
	})
}

// Check checks a checkbox or radio button.
func (l *Locator) Check() error {
	return l.withElement("check", func(ctx context.Context, objectID string) error {
		_, err := callFunctionOn(ctx, l.page.session, objectID, `
			function() {
				if (!this.checked) { this.click(); }
			}
		`, "")
		return err
	})
}

// Uncheck unchecks a checkbox.
func (l *Locator) Uncheck() error {
	return l.withElement("uncheck", func(ctx context.Context, objectID string) error {
		_, err := callFunctionOn(ctx, l.page.session, objectID, `
			function() {
				if (this.checked) { this.click(); }
			}
		`, "")
		return err
	})
}

// Hover moves the mouse over the element.
func (l *Locator) Hover() error {
	return l.withElement("hover", func(ctx context.Context, objectID string) error {
		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}
		x, y, err := getBoxModel(ctx, l.page.session, objectID)
		if err != nil {
			return err
		}
		mouse := newMouse(l.page.session)
		return mouse.Move(ctx, x, y)
	})
}

// Focus focuses the element.
func (l *Locator) Focus() error {
	return l.withElement("focus", func(ctx context.Context, objectID string) error {
		return focusElement(ctx, l.page.session, objectID)
	})
}

// Clear clears the input value.
func (l *Locator) Clear() error {
	return l.Fill("")
}

// DispatchEvent dispatches a DOM event on the element.
func (l *Locator) DispatchEvent(eventType string, eventInit ...map[string]any) error {
	return l.withElement("dispatchEvent", func(ctx context.Context, objectID string) error {
		initJSON := "{bubbles: true}"
		if len(eventInit) > 0 {
			b, _ := json.Marshal(eventInit[0])
			initJSON = string(b)
		}
		_, err := callFunctionOn(ctx, l.page.session, objectID,
			fmt.Sprintf(`function() { this.dispatchEvent(new Event(%s, %s)); }`, jsQuote(eventType), initJSON), "")
		return err
	})
}

// WaitFor waits for the element to reach the given state.
func (l *Locator) WaitFor(opts ...WaitForOptions) error {
	state := "visible"
	timeout := 30 * time.Second
	if len(opts) > 0 {
		if opts[0].State != "" {
			state = opts[0].State
		}
		if opts[0].Timeout > 0 {
			timeout = opts[0].Timeout
		}
	}

	ctx, cancel := context.WithTimeout(l.page.browser.ctx, timeout)
	defer cancel()

	return retryWithBackoff(ctx, func() error {
		objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
		if err != nil {
			return fmt.Errorf("locator %s: %w", l.desc, err)
		}

		switch state {
		case "attached":
			if objectID != "" {
				releaseObject(ctx, l.page.session, objectID)
				return nil
			}
		case "detached":
			if objectID == "" {
				return nil
			}
			releaseObject(ctx, l.page.session, objectID)
		case "visible":
			if objectID != "" {
				vis, _ := isElementVisible(ctx, l.page.session, objectID)
				releaseObject(ctx, l.page.session, objectID)
				if vis {
					return nil
				}
			}
		case "hidden":
			if objectID == "" {
				return nil
			}
			vis, _ := isElementVisible(ctx, l.page.session, objectID)
			releaseObject(ctx, l.page.session, objectID)
			if !vis {
				return nil
			}
		}
		return fmt.Errorf("locator %s: waitFor(state=%s) not satisfied", l.desc, state)
	})
}

// WaitForOptions configures WaitFor.
type WaitForOptions struct {
	State   string
	Timeout time.Duration
}

// Filter narrows a locator with additional criteria.
// Uses innerText (like Playwright) which respects rendering and ignores hidden elements.
func (l *Locator) Filter(opts FilterOptions) *Locator {
	result := l
	if opts.HasText != "" {
		selectorAllJS := result.selectorAll
		if selectorAllJS == "" {
			selectorAllJS = result.selector
		}
		js := fmt.Sprintf(`(function(){
			var els = %s;
			if (!els) return null;
			if (!els.length && els.length !== 0) els = [els];
			var target = %s.toLowerCase();
			for (var i = 0; i < els.length; i++) {
				if ((els[i].innerText || els[i].textContent || '').toLowerCase().includes(target)) return els[i];
			}
			return null;
		})()`, selectorAllJS, jsQuote(opts.HasText))
		jsAll := fmt.Sprintf(`(function(){
			var els = %s;
			if (!els) return [];
			if (!els.length && els.length !== 0) els = [els];
			var target = %s.toLowerCase(), results = [];
			for (var i = 0; i < els.length; i++) {
				if ((els[i].innerText || els[i].textContent || '').toLowerCase().includes(target)) results.push(els[i]);
			}
			return results;
		})()`, selectorAllJS, jsQuote(opts.HasText))
		result = &Locator{page: l.page, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("%s.filter(hasText=%q)", l.desc, opts.HasText)}
	}
	if opts.HasNotText != "" {
		selectorAllJS := result.selectorAll
		if selectorAllJS == "" {
			selectorAllJS = result.selector
		}
		js := fmt.Sprintf(`(function(){
			var els = %s;
			if (!els) return null;
			if (!els.length && els.length !== 0) els = [els];
			var target = %s.toLowerCase();
			for (var i = 0; i < els.length; i++) {
				if (!(els[i].innerText || els[i].textContent || '').toLowerCase().includes(target)) return els[i];
			}
			return null;
		})()`, selectorAllJS, jsQuote(opts.HasNotText))
		result = &Locator{page: l.page, selector: js, desc: fmt.Sprintf("%s.filter(hasNotText=%q)", result.desc, opts.HasNotText)}
	}
	return result
}

// FilterOptions configures Filter.
type FilterOptions struct {
	HasText    string
	HasNotText string
}

// Last returns a locator for the last matching element.
func (l *Locator) Last() *Locator {
	allJS := l.selectorAll
	if allJS == "" {
		allJS = l.selector
	}
	js := fmt.Sprintf(`(function(){ var all = %s; if (!all) return null; if (!all.length && all.length !== 0) return all; return all.length ? all[all.length-1] : null; })()`,
		allJS)
	return &Locator{page: l.page, selector: js, desc: l.desc + ".last()", frame: l.frame}
}

// --- Queries (auto-wait, return values) ---

// TextContent returns the element's textContent.
func (l *Locator) TextContent() (string, error) {
	var text string
	err := l.withElement("textContent", func(ctx context.Context, objectID string) error {
		val, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return this.textContent; }`, "")
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &text)
	})
	return text, err
}

// InnerText returns the element's innerText.
func (l *Locator) InnerText() (string, error) {
	var text string
	err := l.withElement("innerText", func(ctx context.Context, objectID string) error {
		val, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return this.innerText; }`, "")
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &text)
	})
	return text, err
}

// InputValue returns the value of an input/textarea/select element.
func (l *Locator) InputValue() (string, error) {
	var val string
	err := l.withElement("inputValue", func(ctx context.Context, objectID string) error {
		raw, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return this.value; }`, "")
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &val)
	})
	return val, err
}

// GetAttribute returns the value of an element's attribute.
func (l *Locator) GetAttribute(name string) (string, error) {
	var val string
	err := l.withElement("getAttribute", func(ctx context.Context, objectID string) error {
		raw, err := callFunctionOn(ctx, l.page.session, objectID,
			fmt.Sprintf(`function() { return this.getAttribute(%s); }`, jsQuote(name)), "")
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &val)
	})
	return val, err
}

// IsVisible returns true if the element exists and is visible.
func (l *Locator) IsVisible() (bool, error) {
	ctx := l.page.browser.ctx
	objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
	if err != nil || objectID == "" {
		return false, err
	}
	defer releaseObject(ctx, l.page.session, objectID)

	raw, err := callFunctionOn(ctx, l.page.session, objectID, `
		function() {
			const style = window.getComputedStyle(this);
			return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0'
				&& this.offsetWidth > 0 && this.offsetHeight > 0;
		}
	`, "")
	if err != nil {
		return false, err
	}
	var visible bool
	json.Unmarshal(raw, &visible)
	return visible, nil
}

// IsEnabled returns true if the element is not disabled.
func (l *Locator) IsEnabled() (bool, error) {
	ctx := l.page.browser.ctx
	objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
	if err != nil || objectID == "" {
		return false, err
	}
	defer releaseObject(ctx, l.page.session, objectID)

	raw, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return !this.disabled; }`, "")
	if err != nil {
		return false, err
	}
	var enabled bool
	json.Unmarshal(raw, &enabled)
	return enabled, nil
}

// IsChecked returns true if a checkbox/radio is checked.
func (l *Locator) IsChecked() (bool, error) {
	ctx := l.page.browser.ctx
	objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
	if err != nil || objectID == "" {
		return false, err
	}
	defer releaseObject(ctx, l.page.session, objectID)

	raw, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return !!this.checked; }`, "")
	if err != nil {
		return false, err
	}
	var checked bool
	json.Unmarshal(raw, &checked)
	return checked, nil
}

// Count returns the number of matching elements (no waiting).
func (l *Locator) Count() (int, error) {
	// Convert to querySelectorAll for counting
	countJS := strings.Replace(l.selector, "querySelector(", "querySelectorAll(", 1)
	val, err := l.page.Evaluate(fmt.Sprintf(`(function(){ const r = %s; return r ? (r.length !== undefined ? r.length : 1) : 0; })()`, countJS))
	if err != nil {
		return 0, err
	}
	var count int
	json.Unmarshal(val, &count)
	return count, nil
}

// InnerHTML returns the element's innerHTML.
func (l *Locator) InnerHTML() (string, error) {
	var html string
	err := l.withElement("innerHTML", func(ctx context.Context, objectID string) error {
		val, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return this.innerHTML; }`, "")
		if err != nil {
			return err
		}
		return json.Unmarshal(val, &html)
	})
	return html, err
}

// BoundingBox returns the element's bounding box.
type BoundingBox struct {
	X, Y, Width, Height float64
}

func (l *Locator) BoundingBox() (*BoundingBox, error) {
	var box BoundingBox
	err := l.withElement("boundingBox", func(ctx context.Context, objectID string) error {
		raw, err := callFunctionOn(ctx, l.page.session, objectID, `function() {
			const r = this.getBoundingClientRect();
			return {x: r.x, y: r.y, width: r.width, height: r.height};
		}`, "")
		if err != nil {
			return err
		}
		return json.Unmarshal(raw, &box)
	})
	return &box, err
}

// IsHidden returns true if the element is not visible or not in the DOM.
func (l *Locator) IsHidden() (bool, error) {
	vis, err := l.IsVisible()
	return !vis, err
}

// IsEditable returns true if the element is not read-only.
func (l *Locator) IsEditable() (bool, error) {
	ctx := l.page.browser.ctx
	objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
	if err != nil || objectID == "" {
		return false, err
	}
	defer releaseObject(ctx, l.page.session, objectID)

	raw, err := callFunctionOn(ctx, l.page.session, objectID, `function() { return !this.readOnly && !this.disabled; }`, "")
	if err != nil {
		return false, err
	}
	var editable bool
	json.Unmarshal(raw, &editable)
	return editable, nil
}

// IsDisabled returns true if the element is disabled.
func (l *Locator) IsDisabled() (bool, error) {
	enabled, err := l.IsEnabled()
	return !enabled, err
}

// AllTextContents returns the text content of all matching elements.
func (l *Locator) AllTextContents() ([]string, error) {
	countJS := strings.Replace(l.selector, "querySelector(", "querySelectorAll(", 1)
	val, err := l.page.Evaluate(fmt.Sprintf(`(function(){ const els = %s; return els ? Array.from(els).map(e => e.textContent) : []; })()`, countJS))
	if err != nil {
		return nil, err
	}
	var texts []string
	json.Unmarshal(val, &texts)
	return texts, nil
}

// AllInnerTexts returns the inner text of all matching elements.
func (l *Locator) AllInnerTexts() ([]string, error) {
	countJS := strings.Replace(l.selector, "querySelector(", "querySelectorAll(", 1)
	val, err := l.page.Evaluate(fmt.Sprintf(`(function(){ const els = %s; return els ? Array.from(els).map(e => e.innerText) : []; })()`, countJS))
	if err != nil {
		return nil, err
	}
	var texts []string
	json.Unmarshal(val, &texts)
	return texts, nil
}

// Evaluate runs JavaScript in the context of the matched element.
func (l *Locator) Evaluate(expression string, arg ...any) (json.RawMessage, error) {
	var result json.RawMessage
	err := l.withElement("evaluate", func(ctx context.Context, objectID string) error {
		var argsJSON string
		if len(arg) > 0 {
			b, _ := json.Marshal(arg[0])
			argsJSON = string(b)
		}
		val, err := callFunctionOn(ctx, l.page.session, objectID, expression, argsJSON)
		if err != nil {
			return err
		}
		result = val
		return nil
	})
	return result, err
}

// Screenshot captures a screenshot of just this element.
func (l *Locator) Screenshot() ([]byte, error) {
	box, err := l.BoundingBox()
	if err != nil {
		return nil, err
	}
	return l.page.ScreenshotWithOptions(ScreenshotOptions{
		Clip: &Rect{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height},
	})
}

const defaultTimeout = 30 * time.Second

// actionChecks specifies which actionability checks to run, matching Playwright's
// per-action requirements. See: https://playwright.dev/docs/actionability
type actionChecks struct {
	visible   bool
	enabled   bool
	stable    bool
	hitTarget bool
}

// Playwright's actionability requirements per action type:
var (
	checksClick    = actionChecks{visible: true, enabled: true, stable: true, hitTarget: true}
	checksFill     = actionChecks{visible: true, enabled: true}   // fill: no stable/hitTarget
	checksType     = actionChecks{visible: true, enabled: true}   // type: no stable/hitTarget
	checksHover    = actionChecks{visible: true, stable: true}    // hover: no enabled
	checksFocus    = actionChecks{}                                // focus: just needs element
	checksEvaluate = actionChecks{}                                // evaluate: just needs element
	checksDefault  = actionChecks{visible: true, enabled: true}   // safe default
)

func (l *Locator) withElement(action string, fn func(ctx context.Context, objectID string) error) error {
	// Determine which actionability checks to run based on action type
	checks := checksDefault
	switch action {
	case "click", "dblclick", "tap", "dragTo":
		checks = checksClick
	case "fill", "clear", "selectOption", "setInputFiles", "setChecked", "check", "uncheck":
		checks = checksFill
	case "type", "press", "pressSequentially":
		checks = checksType
	case "hover":
		checks = checksHover
	case "focus", "blur":
		checks = checksFocus
	case "evaluate", "evaluateHandle", "textContent", "innerText", "innerHTML",
		"inputValue", "getAttribute", "boundingBox", "screenshot", "isVisible",
		"isEnabled", "isChecked", "isEditable", "isDisabled", "isHidden",
		"selectText", "highlight", "scrollIntoViewIfNeeded", "dispatchEvent":
		checks = checksEvaluate
	}

	return l.withElementChecks(action, checks, fn)
}

func (l *Locator) withElementChecks(action string, checks actionChecks, fn func(ctx context.Context, objectID string) error) error {
	ctx := l.page.browser.ctx
	timeout := defaultTimeout
	if l.page.defaultTimeout > 0 {
		timeout = l.page.defaultTimeout
	}
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error
	return retryWithBackoff(ctx, func() error {
		if l.strict && l.selectorAll != "" {
			countExpr := fmt.Sprintf(`(function(){ var r = %s; return r ? r.length : 0; })()`, l.selectorAll)
			result, err := l.page.session.Call(ctx, "Runtime.evaluate", map[string]any{
				"expression":    countExpr,
				"returnByValue": true,
			})
			if err == nil {
				var resp struct {
					Result struct {
						Value json.RawMessage `json:"value"`
					} `json:"result"`
				}
				json.Unmarshal(result, &resp)
				var count int
				json.Unmarshal(resp.Result.Value, &count)
				if count > 1 {
					return &nonRetriableError{fmt.Errorf("locator %s: strict mode violation: resolved to %d elements", l.desc, count)}
				}
			}
		}

		objectID, err := evaluateHandle(ctx, l.page.session, l.selector)
		if err != nil {
			lastErr = fmt.Errorf("locator %s: %w", l.desc, err)
			return lastErr
		}
		if objectID == "" {
			lastErr = fmt.Errorf("locator %s: %s timed out: element not found", l.desc, action)
			return lastErr
		}

		if checks.visible {
			if err := checkVisible(ctx, l.page.session, objectID); err != nil {
				releaseObject(ctx, l.page.session, objectID)
				lastErr = fmt.Errorf("locator %s: %s timed out: %v", l.desc, action, err)
				return lastErr
			}
		}

		if checks.enabled {
			if err := checkEnabled(ctx, l.page.session, objectID); err != nil {
				releaseObject(ctx, l.page.session, objectID)
				lastErr = fmt.Errorf("locator %s: %s timed out: %v", l.desc, action, err)
				return lastErr
			}
		}

		if checks.stable {
			if err := checkStable(ctx, l.page.session, objectID); err != nil {
				releaseObject(ctx, l.page.session, objectID)
				lastErr = fmt.Errorf("locator %s: %s timed out: %v", l.desc, action, err)
				return lastErr
			}
		}

		if checks.hitTarget {
			if err := checkHitTarget(ctx, l.page.session, objectID); err != nil {
				releaseObject(ctx, l.page.session, objectID)
				lastErr = fmt.Errorf("locator %s: %s timed out: %v", l.desc, action, err)
				return lastErr
			}
		}

		if l.page.browser.slowMo > 0 {
			time.Sleep(l.page.browser.slowMo)
		}

		err = fn(ctx, objectID)
		releaseObject(ctx, l.page.session, objectID)
		if err != nil {
			lastErr = err
			return err
		}
		return nil
	})
}

func isElementVisible(ctx context.Context, session *cdp.Session, objectID string) (bool, error) {
	raw, err := callFunctionOn(ctx, session, objectID, `
		function() {
			const style = window.getComputedStyle(this);
			return style.display !== 'none' && style.visibility !== 'hidden' && style.opacity !== '0'
				&& this.offsetWidth > 0 && this.offsetHeight > 0;
		}
	`, "")
	if err != nil {
		return false, err
	}
	var v bool
	json.Unmarshal(raw, &v)
	return v, nil
}

func isElementEnabled(ctx context.Context, session *cdp.Session, objectID string) (bool, error) {
	raw, err := callFunctionOn(ctx, session, objectID, `function() { return !this.disabled; }`, "")
	if err != nil {
		return false, err
	}
	var v bool
	json.Unmarshal(raw, &v)
	return v, nil
}

// callFunctionOn executes a function on a remote object and returns the result.
func callFunctionOn(ctx context.Context, session *cdp.Session, objectID, fn string, argsJSON string) (json.RawMessage, error) {
	params := map[string]any{
		"functionDeclaration": fn,
		"objectId":            objectID,
		"returnByValue":       true,
		"awaitPromise":        true,
	}

	if argsJSON != "" {
		params["arguments"] = []map[string]any{{"value": json.RawMessage(argsJSON)}}
	}

	result, err := session.Call(ctx, "Runtime.callFunctionOn", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result struct {
			Value json.RawMessage `json:"value"`
		} `json:"result"`
		ExceptionDetails *struct {
			Text string `json:"text"`
		} `json:"exceptionDetails"`
	}
	if err := json.Unmarshal(result, &resp); err != nil {
		return nil, err
	}
	if resp.ExceptionDetails != nil {
		return nil, fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
	}
	return resp.Result.Value, nil
}

// releaseObject releases a remote object reference.
func releaseObject(ctx context.Context, session *cdp.Session, objectID string) {
	session.Call(ctx, "Runtime.releaseObject", map[string]any{
		"objectId": objectID,
	})
}

// jsQuote quotes a string for safe inclusion in JavaScript.
func jsQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// implicitRoleSelector returns a CSS selector for elements that have the given
// implicit ARIA role (e.g. "button" matches <button>, "link" matches <a[href]>).
func implicitRoleSelector(role string) string {
	switch role {
	case "button":
		return "button, input[type=button], input[type=submit], input[type=reset]"
	case "link":
		return "a[href]"
	case "textbox":
		return "input:not([type]), input[type=text], input[type=email], input[type=password], input[type=search], input[type=tel], input[type=url], textarea"
	case "checkbox":
		return "input[type=checkbox]"
	case "radio":
		return "input[type=radio]"
	case "combobox":
		return "select"
	case "heading":
		return "h1, h2, h3, h4, h5, h6"
	case "list":
		return "ul, ol"
	case "listitem":
		return "li"
	case "navigation":
		return "nav"
	case "img":
		return "img"
	default:
		return fmt.Sprintf("[role=%s]", jsQuote(role))
	}
}

func (l *Locator) And(other *Locator) *Locator {
	js := fmt.Sprintf(`(function(){ var a = %s; var b = %s; if (a && b && a === b) return a; return null; })()`,
		l.selector, other.selector)
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf("%s.and(%s)", l.desc, other.desc)}
}

func (l *Locator) Or(other *Locator) *Locator {
	js := fmt.Sprintf(`(function(){ var a = %s; if (a) return a; return %s; })()`,
		l.selector, other.selector)
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf("%s.or(%s)", l.desc, other.desc)}
}

func (l *Locator) PressSequentially(text string, opts ...PressSequentiallyOptions) error {
	return l.withElement("pressSequentially", func(ctx context.Context, objectID string) error {
		if err := focusElement(ctx, l.page.session, objectID); err != nil {
			return err
		}
		var delay time.Duration
		if len(opts) > 0 {
			delay = opts[0].Delay
		}
		kb := l.page.Keyboard()
		for _, ch := range text {
			if delay > 0 {
				time.Sleep(delay)
			}
			if err := kb.Press(ctx, string(ch)); err != nil {
				return err
			}
		}
		return nil
	})
}

type PressSequentiallyOptions struct {
	Delay time.Duration
}

func (l *Locator) ScrollIntoViewIfNeeded() error {
	return l.withElement("scrollIntoViewIfNeeded", func(ctx context.Context, objectID string) error {
		return scrollIntoView(ctx, l.page.session, objectID)
	})
}

func (l *Locator) SetInputFiles(files ...string) error {
	return l.withElement("setInputFiles", func(ctx context.Context, objectID string) error {
		result, err := l.page.session.Call(ctx, "DOM.describeNode", map[string]any{
			"objectId": objectID,
		})
		if err != nil {
			return err
		}
		var resp struct {
			Node struct {
				BackendNodeID int `json:"backendNodeId"`
			} `json:"node"`
		}
		json.Unmarshal(result, &resp)

		_, err = l.page.session.Call(ctx, "DOM.setFileInputFiles", map[string]any{
			"files":         files,
			"backendNodeId": resp.Node.BackendNodeID,
		})
		return err
	})
}

func (l *Locator) Tap() error {
	return l.withElement("tap", func(ctx context.Context, objectID string) error {
		if err := scrollIntoView(ctx, l.page.session, objectID); err != nil {
			return err
		}
		x, y, err := getBoxModel(ctx, l.page.session, objectID)
		if err != nil {
			return err
		}
		ts := l.page.Touchscreen()
		return ts.Tap(ctx, x, y)
	})
}

func (l *Locator) DragTo(target *Locator, opts ...DragToOptions) error {
	return l.withElement("dragTo", func(ctx context.Context, sourceID string) error {
		if err := scrollIntoView(ctx, l.page.session, sourceID); err != nil {
			return err
		}
		sx, sy, err := getBoxModel(ctx, l.page.session, sourceID)
		if err != nil {
			return err
		}

		targetID, err := evaluateHandle(ctx, l.page.session, target.selector)
		if err != nil || targetID == "" {
			return fmt.Errorf("drag target not found")
		}
		defer releaseObject(ctx, l.page.session, targetID)
		if err := scrollIntoView(ctx, l.page.session, targetID); err != nil {
			return err
		}
		tx, ty, err := getBoxModel(ctx, l.page.session, targetID)
		if err != nil {
			return err
		}

		mouse := l.page.Mouse()
		if err := mouse.Move(ctx, sx, sy); err != nil {
			return err
		}
		if err := mouse.Down(ctx); err != nil {
			return err
		}
		steps := 10
		if len(opts) > 0 && opts[0].SourcePosition != nil {
			sx = opts[0].SourcePosition.X
			sy = opts[0].SourcePosition.Y
		}
		if len(opts) > 0 && opts[0].TargetPosition != nil {
			tx = opts[0].TargetPosition.X
			ty = opts[0].TargetPosition.Y
		}
		if err := mouse.Move(ctx, tx, ty, MouseMoveOptions{Steps: steps}); err != nil {
			return err
		}
		return mouse.Up(ctx)
	})
}

type DragToOptions struct {
	SourcePosition *Position
	TargetPosition *Position
}

type Position struct {
	X, Y float64
}

func (l *Locator) Blur() error {
	return l.withElement("blur", func(ctx context.Context, objectID string) error {
		_, err := callFunctionOn(ctx, l.page.session, objectID, `function() { this.blur(); }`, "")
		return err
	})
}

func (l *Locator) SetChecked(checked bool) error {
	if checked {
		return l.Check()
	}
	return l.Uncheck()
}

func (l *Locator) SelectText() error {
	return l.withElement("selectText", func(ctx context.Context, objectID string) error {
		_, err := callFunctionOn(ctx, l.page.session, objectID, `function() {
			if (this.select) { this.select(); return; }
			var range = document.createRange();
			range.selectNodeContents(this);
			var sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
		}`, "")
		return err
	})
}

func (l *Locator) EvaluateAll(expression string, arg ...any) (json.RawMessage, error) {
	allJS := strings.Replace(l.selector, "querySelector(", "querySelectorAll(", 1)
	var argsStr string
	if len(arg) > 0 {
		b, _ := json.Marshal(arg[0])
		argsStr = ", " + string(b)
	}
	js := fmt.Sprintf(`(function(){ var els = %s; return (%s)(Array.from(els || [])%s); })()`, allJS, expression, argsStr)
	return l.page.Evaluate(js)
}

func (l *Locator) All() ([]*Locator, error) {
	count, err := l.Count()
	if err != nil {
		return nil, err
	}
	locators := make([]*Locator, count)
	for i := 0; i < count; i++ {
		locators[i] = l.Nth(i)
	}
	return locators, nil
}

func (l *Locator) Highlight() error {
	return l.withElement("highlight", func(ctx context.Context, objectID string) error {
		_, err := callFunctionOn(ctx, l.page.session, objectID, `function() {
			this.style.outline = '2px solid red';
			this.style.outlineOffset = '-2px';
		}`, "")
		return err
	})
}

func (p *Page) GetByAltText(text string, exact ...bool) *Locator {
	isExact := len(exact) > 0 && exact[0]
	var js, jsAll string
	if isExact {
		css := fmt.Sprintf(`[alt=%s]`, jsQuote(text))
		js = fmt.Sprintf(`document.querySelector('%s')`, css)
		jsAll = fmt.Sprintf(`document.querySelectorAll('%s')`, css)
	} else {
		js = fmt.Sprintf(`(function() {
			var els = document.querySelectorAll('[alt]');
			var t = %s.toLowerCase();
			for (var i = 0; i < els.length; i++) {
				if (els[i].getAttribute('alt').toLowerCase().includes(t)) return els[i];
			}
			return null;
		})()`, jsQuote(text))
		jsAll = fmt.Sprintf(`(function() {
			var els = document.querySelectorAll('[alt]');
			var t = %s.toLowerCase(), results = [];
			for (var i = 0; i < els.length; i++) {
				if (els[i].getAttribute('alt').toLowerCase().includes(t)) results.push(els[i]);
			}
			return results;
		})()`, jsQuote(text))
	}
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByAltText(%q)", text), strict: true}
}

func (p *Page) GetByTitle(text string, exact ...bool) *Locator {
	isExact := len(exact) > 0 && exact[0]
	var js, jsAll string
	if isExact {
		css := fmt.Sprintf(`[title=%s]`, jsQuote(text))
		js = fmt.Sprintf(`document.querySelector('%s')`, css)
		jsAll = fmt.Sprintf(`document.querySelectorAll('%s')`, css)
	} else {
		js = fmt.Sprintf(`(function() {
			var els = document.querySelectorAll('[title]');
			var t = %s.toLowerCase();
			for (var i = 0; i < els.length; i++) {
				if (els[i].getAttribute('title').toLowerCase().includes(t)) return els[i];
			}
			return null;
		})()`, jsQuote(text))
		jsAll = fmt.Sprintf(`(function() {
			var els = document.querySelectorAll('[title]');
			var t = %s.toLowerCase(), results = [];
			for (var i = 0; i < els.length; i++) {
				if (els[i].getAttribute('title').toLowerCase().includes(t)) results.push(els[i]);
			}
			return results;
		})()`, jsQuote(text))
	}
	return &Locator{page: p, selector: js, selectorAll: jsAll, desc: fmt.Sprintf("getByTitle(%q)", text), strict: true}
}

func (l *Locator) ContentFrame() *FrameLocator {
	return &FrameLocator{page: l.page, selector: l.desc}
}

type ClickOptions struct {
	Button     string
	ClickCount int
	Delay      time.Duration
	Position   *Position
	Force      bool
	NoWaitAfter bool
	Modifiers  []string
	Trial      bool
	Timeout    time.Duration
}

type FillOptions struct {
	Force       bool
	NoWaitAfter bool
	Timeout     time.Duration
}

func (l *Locator) GetByText(text string, exact ...bool) *Locator {
	isExact := len(exact) > 0 && exact[0]
	var matchExpr string
	if isExact {
		matchExpr = fmt.Sprintf(`n.textContent.trim() === %s`, jsQuote(text))
	} else {
		matchExpr = fmt.Sprintf(`n.textContent.toLowerCase().includes(%s.toLowerCase())`, jsQuote(text))
	}
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; var walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT); while (walker.nextNode()) { var n = walker.currentNode; if (%s) return n.parentElement; } return null; })()`, l.selector, matchExpr)
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByText(%q)`, l.desc, text), frame: l.frame}
}

func (l *Locator) GetByRole(role string, opts ...ByRoleOption) *Locator {
	cssSelector := fmt.Sprintf("[role=%s], %s", jsQuote(role), implicitRoleSelector(role))
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; return root.querySelector(%s); })()`, l.selector, jsQuote(cssSelector))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByRole(%q)`, l.desc, role), frame: l.frame}
}

func (l *Locator) GetByTestId(testID string) *Locator {
	attr := globalSelectors.TestIdAttribute()
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; return root.querySelector('[%s=' + %s + ']'); })()`, l.selector, attr, jsQuote(testID))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByTestId(%q)`, l.desc, testID), frame: l.frame}
}

func (l *Locator) GetByLabel(text string) *Locator {
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; var labels = root.querySelectorAll('label'); for (var i = 0; i < labels.length; i++) { if (labels[i].textContent.includes(%s)) { var f = labels[i].getAttribute('for'); if (f) { var el = root.querySelector('#' + f); if (el) return el; } return labels[i].querySelector('input, textarea, select'); } } return null; })()`, l.selector, jsQuote(text))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByLabel(%q)`, l.desc, text), frame: l.frame}
}

func (l *Locator) GetByPlaceholder(text string) *Locator {
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; return root.querySelector('[placeholder=' + %s + ']'); })()`, l.selector, jsQuote(text))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByPlaceholder(%q)`, l.desc, text), frame: l.frame}
}

func (l *Locator) GetByAltText(text string, exact ...bool) *Locator {
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; return root.querySelector('[alt=' + %s + ']'); })()`, l.selector, jsQuote(text))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByAltText(%q)`, l.desc, text), frame: l.frame}
}

func (l *Locator) GetByTitle(text string, exact ...bool) *Locator {
	js := fmt.Sprintf(`(function(){ var root = %s; if (!root) return null; return root.querySelector('[title=' + %s + ']'); })()`, l.selector, jsQuote(text))
	return &Locator{page: l.page, selector: js, desc: fmt.Sprintf(`%s >> getByTitle(%q)`, l.desc, text), frame: l.frame}
}

func (l *Locator) EvaluateHandle(expression string, arg ...any) (*JSHandle, error) {
	var result *JSHandle
	err := l.withElement("evaluateHandle", func(ctx context.Context, objectID string) error {
		params := map[string]any{
			"functionDeclaration": expression,
			"objectId":            objectID,
			"returnByValue":       false,
			"awaitPromise":        true,
		}
		if len(arg) > 0 {
			b, _ := json.Marshal(arg[0])
			params["arguments"] = []map[string]any{{"value": json.RawMessage(b)}}
		}
		raw, err := l.page.session.Call(ctx, "Runtime.callFunctionOn", params)
		if err != nil {
			return err
		}
		var resp struct {
			Result struct {
				ObjectID string          `json:"objectId"`
				Value    json.RawMessage `json:"value"`
			} `json:"result"`
			ExceptionDetails *struct {
				Text string `json:"text"`
			} `json:"exceptionDetails"`
		}
		json.Unmarshal(raw, &resp)
		if resp.ExceptionDetails != nil {
			return fmt.Errorf("js exception: %s", resp.ExceptionDetails.Text)
		}
		result = &JSHandle{session: l.page.session, page: l.page, objectID: resp.Result.ObjectID, value: resp.Result.Value}
		return nil
	})
	return result, err
}

func (l *Locator) AriaSnapshot() (*AccessibilityNode, error) {
	return l.page.Accessibility().Snapshot(AccessibilitySnapshotOptions{Root: l})
}

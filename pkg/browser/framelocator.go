package browser

import (
	"encoding/json"
	"fmt"
)

type FrameLocator struct {
	page        *Page
	parentFrame *Frame
	selector    string
}

func (fl *FrameLocator) Locator(selector string) *Locator {
	iframeJS := fl.iframeContentDocumentJS()
	js := fmt.Sprintf(`(function(){ var doc = %s; if (!doc) return null; return doc.querySelector(%s); })()`,
		iframeJS, jsQuote(selector))
	return &Locator{page: fl.page, selector: js, desc: fmt.Sprintf("frameLocator(%q) >> %s", fl.selector, selector)}
}

func (fl *FrameLocator) GetByText(text string, exact ...bool) *Locator {
	loc := fl.Locator("*")
	loc.desc = fmt.Sprintf("frameLocator(%q) >> getByText(%q)", fl.selector, text)
	return loc
}

func (fl *FrameLocator) GetByRole(role string, opts ...ByRoleOption) *Locator {
	loc := fl.Locator(fmt.Sprintf("[role=%s], %s", jsQuote(role), implicitRoleSelector(role)))
	loc.desc = fmt.Sprintf("frameLocator(%q) >> getByRole(%q)", fl.selector, role)
	return loc
}

func (fl *FrameLocator) GetByTestId(testID string) *Locator {
	loc := fl.Locator(fmt.Sprintf("[data-testid=%s]", jsQuote(testID)))
	loc.desc = fmt.Sprintf("frameLocator(%q) >> getByTestId(%q)", fl.selector, testID)
	return loc
}

func (fl *FrameLocator) First() *FrameLocator {
	return &FrameLocator{page: fl.page, parentFrame: fl.parentFrame, selector: fl.selector + " >> nth=0"}
}

func (fl *FrameLocator) Nth(index int) *FrameLocator {
	return &FrameLocator{page: fl.page, parentFrame: fl.parentFrame, selector: fmt.Sprintf("%s >> nth=%d", fl.selector, index)}
}

func (fl *FrameLocator) Last() *FrameLocator {
	return &FrameLocator{page: fl.page, parentFrame: fl.parentFrame, selector: fl.selector + " >> nth=-1"}
}

func (fl *FrameLocator) GetByLabel(text string) *Locator {
	return fl.Locator(fmt.Sprintf(`[for=%s], label`, jsQuote(text)))
}

func (fl *FrameLocator) GetByPlaceholder(text string) *Locator {
	return fl.Locator(fmt.Sprintf(`[placeholder=%s]`, jsQuote(text)))
}

func (fl *FrameLocator) GetByAltText(text string) *Locator {
	return fl.Locator(fmt.Sprintf(`[alt=%s]`, jsQuote(text)))
}

func (fl *FrameLocator) GetByTitle(text string) *Locator {
	return fl.Locator(fmt.Sprintf(`[title=%s]`, jsQuote(text)))
}

func (fl *FrameLocator) FrameLocator(selector string) *FrameLocator {
	return &FrameLocator{
		page:     fl.page,
		selector: fmt.Sprintf("%s >> %s", fl.selector, selector),
	}
}

func (fl *FrameLocator) iframeContentDocumentJS() string {
	return fmt.Sprintf(`(function(){ var iframe = document.querySelector(%s); if (!iframe || !iframe.contentDocument) return null; return iframe.contentDocument; })()`,
		jsQuote(fl.selector))
}

func (fl *FrameLocator) Owner() *Locator {
	js := fmt.Sprintf(`document.querySelector(%s)`, jsQuote(fl.selector))
	return &Locator{page: fl.page, selector: js, desc: fmt.Sprintf("frameLocator(%q).owner()", fl.selector)}
}

func (fl *FrameLocator) resolveFrame() *Frame {
	ctx := fl.page.browser.ctx
	objectID, err := evaluateHandle(ctx, fl.page.session, fmt.Sprintf(`document.querySelector(%s)`, jsQuote(fl.selector)))
	if err != nil || objectID == "" {
		return nil
	}
	defer releaseObject(ctx, fl.page.session, objectID)

	raw, err := callFunctionOn(ctx, fl.page.session, objectID, `function() {
		if (this.tagName !== 'IFRAME' && this.tagName !== 'FRAME') return null;
		return this.contentWindow ? true : null;
	}`, "")
	if err != nil {
		return nil
	}

	var ok bool
	json.Unmarshal(raw, &ok)
	if !ok {
		return nil
	}

	nameResult, _ := callFunctionOn(ctx, fl.page.session, objectID, `function() { return this.name || this.id || ''; }`, "")
	var frameName string
	json.Unmarshal(nameResult, &frameName)

	srcResult, _ := callFunctionOn(ctx, fl.page.session, objectID, `function() { return this.src || ''; }`, "")
	var frameSrc string
	json.Unmarshal(srcResult, &frameSrc)

	for _, f := range fl.page.Frames() {
		if (frameName != "" && f.name == frameName) || (frameSrc != "" && f.url == frameSrc) {
			return f
		}
	}
	return nil
}

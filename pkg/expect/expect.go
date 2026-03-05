package expect

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/PeterStoica/gowright/pkg/browser"
)

var DefaultTimeout = 5 * time.Second

type LocatorAssertions struct {
	locator *browser.Locator
	not     bool
	timeout time.Duration
}

func Locator(l *browser.Locator) *LocatorAssertions {
	return &LocatorAssertions{locator: l, timeout: DefaultTimeout}
}

func (a *LocatorAssertions) Not() *LocatorAssertions {
	return &LocatorAssertions{locator: a.locator, not: true, timeout: a.timeout}
}

func (a *LocatorAssertions) WithTimeout(d time.Duration) *LocatorAssertions {
	return &LocatorAssertions{locator: a.locator, not: a.not, timeout: d}
}

func (a *LocatorAssertions) retry(check func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	var lastErr error
	for {
		err := check()
		if a.not {
			if err != nil {
				return nil
			}
			lastErr = fmt.Errorf("expected condition to be false, but it was true")
		} else {
			if err == nil {
				return nil
			}
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (a *LocatorAssertions) ToBeVisible() error {
	return a.retry(func() error {
		visible, err := a.locator.IsVisible()
		if err != nil {
			return err
		}
		if !visible {
			return fmt.Errorf("expected element to be visible")
		}
		return nil
	})
}

func (a *LocatorAssertions) ToBeHidden() error {
	return a.retry(func() error {
		visible, err := a.locator.IsVisible()
		if err != nil {
			return err
		}
		if visible {
			return fmt.Errorf("expected element to be hidden")
		}
		return nil
	})
}

func (a *LocatorAssertions) ToBeEnabled() error {
	return a.retry(func() error {
		disabled, err := a.locator.IsDisabled()
		if err != nil {
			return err
		}
		if disabled {
			return fmt.Errorf("expected element to be enabled")
		}
		return nil
	})
}

func (a *LocatorAssertions) ToBeDisabled() error {
	return a.retry(func() error {
		disabled, err := a.locator.IsDisabled()
		if err != nil {
			return err
		}
		if !disabled {
			return fmt.Errorf("expected element to be disabled")
		}
		return nil
	})
}

func (a *LocatorAssertions) ToBeChecked() error {
	return a.retry(func() error {
		checked, err := a.locator.IsChecked()
		if err != nil {
			return err
		}
		if !checked {
			return fmt.Errorf("expected element to be checked")
		}
		return nil
	})
}

func (a *LocatorAssertions) ToHaveText(expected string) error {
	return a.retry(func() error {
		text, err := a.locator.TextContent()
		if err != nil {
			return err
		}
		if text != expected {
			return fmt.Errorf("expected text %q, got %q", expected, text)
		}
		return nil
	})
}

func (a *LocatorAssertions) ToContainText(expected string) error {
	return a.retry(func() error {
		text, err := a.locator.TextContent()
		if err != nil {
			return err
		}
		if !strings.Contains(text, expected) {
			return fmt.Errorf("expected text to contain %q, got %q", expected, text)
		}
		return nil
	})
}

func (a *LocatorAssertions) ToHaveValue(expected string) error {
	return a.retry(func() error {
		val, err := a.locator.InputValue()
		if err != nil {
			return err
		}
		if val != expected {
			return fmt.Errorf("expected value %q, got %q", expected, val)
		}
		return nil
	})
}

func (a *LocatorAssertions) ToHaveAttribute(name, expected string) error {
	return a.retry(func() error {
		val, err := a.locator.GetAttribute(name)
		if err != nil {
			return err
		}
		if val != expected {
			return fmt.Errorf("expected attribute %q to be %q, got %q", name, expected, val)
		}
		return nil
	})
}

func (a *LocatorAssertions) ToHaveCount(expected int) error {
	return a.retry(func() error {
		count, err := a.locator.Count()
		if err != nil {
			return err
		}
		if count != expected {
			return fmt.Errorf("expected count %d, got %d", expected, count)
		}
		return nil
	})
}

type PageAssertions struct {
	page    *browser.Page
	not     bool
	timeout time.Duration
}

func Page(p *browser.Page) *PageAssertions {
	return &PageAssertions{page: p, timeout: DefaultTimeout}
}

func (a *PageAssertions) Not() *PageAssertions {
	return &PageAssertions{page: a.page, not: true, timeout: a.timeout}
}

func (a *PageAssertions) WithTimeout(d time.Duration) *PageAssertions {
	return &PageAssertions{page: a.page, not: a.not, timeout: d}
}

func (a *PageAssertions) retry(check func() error) error {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()
	var lastErr error
	for {
		err := check()
		if a.not {
			if err != nil {
				return nil
			}
			lastErr = fmt.Errorf("expected condition to be false, but it was true")
		} else {
			if err == nil {
				return nil
			}
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return lastErr
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (a *PageAssertions) ToHaveTitle(expected string) error {
	return a.retry(func() error {
		title, err := a.page.Title()
		if err != nil {
			return err
		}
		if title != expected {
			return fmt.Errorf("expected title %q, got %q", expected, title)
		}
		return nil
	})
}

func (a *PageAssertions) ToHaveURL(expected string) error {
	return a.retry(func() error {
		val, err := a.page.Evaluate("window.location.href")
		if err != nil {
			return err
		}
		var url string
		json.Unmarshal(val, &url)
		if !strings.Contains(url, expected) {
			return fmt.Errorf("expected URL to contain %q, got %q", expected, url)
		}
		return nil
	})
}

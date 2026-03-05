package gowright

import (
	"testing"
	"time"

	"github.com/PeterStoica/gowright/pkg/expect"
)

// Expectable is satisfied by *TestPage and *TestLocator.
type Expectable interface {
	expectTarget()
}

// Expectation holds the state for an expect chain and dispatches
// to the underlying pkg/expect assertions, calling t.Fatal on failure.
type Expectation struct {
	t       *testing.T
	target  Expectable
	negate  bool
	timeout time.Duration
}

// Expect starts an assertion chain. target must be a *TestPage or *TestLocator.
func (tc *TestContext) Expect(target Expectable) *Expectation {
	return &Expectation{
		t:      tc.t,
		target: target,
	}
}

// Not negates the following assertion.
func (e *Expectation) Not() *Expectation {
	return &Expectation{t: e.t, target: e.target, negate: true, timeout: e.timeout}
}

// WithTimeout sets a custom timeout for the assertion.
func (e *Expectation) WithTimeout(d time.Duration) *Expectation {
	return &Expectation{t: e.t, target: e.target, negate: e.negate, timeout: d}
}

// --- Locator assertions ---

func (e *Expectation) locatorAssertions() *expect.LocatorAssertions {
	tl, ok := e.target.(*TestLocator)
	if !ok {
		e.t.Fatalf("Expect: target is not a *TestLocator")
	}
	a := expect.Locator(tl.raw)
	if e.negate {
		a = a.Not()
	}
	if e.timeout > 0 {
		a = a.WithTimeout(e.timeout)
	}
	return a
}

func (e *Expectation) pageAssertions() *expect.PageAssertions {
	tp, ok := e.target.(*TestPage)
	if !ok {
		e.t.Fatalf("Expect: target is not a *TestPage")
	}
	a := expect.Page(tp.raw)
	if e.negate {
		a = a.Not()
	}
	if e.timeout > 0 {
		a = a.WithTimeout(e.timeout)
	}
	return a
}

func (e *Expectation) ToBeVisible() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeVisible(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeHidden() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeHidden(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeEnabled() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeEnabled(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeDisabled() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeDisabled(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToBeChecked() {
	e.t.Helper()
	if err := e.locatorAssertions().ToBeChecked(); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveText(expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveText(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToContainText(expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToContainText(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveValue(expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveValue(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveAttribute(name, expected string) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveAttribute(name, expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveCount(n int) {
	e.t.Helper()
	if err := e.locatorAssertions().ToHaveCount(n); err != nil {
		e.t.Fatal(err)
	}
}

// --- Page assertions ---

func (e *Expectation) ToHaveTitle(expected string) {
	e.t.Helper()
	if err := e.pageAssertions().ToHaveTitle(expected); err != nil {
		e.t.Fatal(err)
	}
}

func (e *Expectation) ToHaveURL(expected string) {
	e.t.Helper()
	if err := e.pageAssertions().ToHaveURL(expected); err != nil {
		e.t.Fatal(err)
	}
}

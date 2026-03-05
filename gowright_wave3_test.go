package gowright_test

import (
	"encoding/json"
	"testing"
)

func TestLocatorAnd(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `
		<div id="target" class="special">both</div>
		<div id="other">other</div>
	`)

	loc := page.Locator("#target").And(page.Locator(".special"))
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "both" {
		t.Errorf("expected 'both', got %q", text)
	}
}

func TestLocatorOr(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="a">first</div>`)

	loc := page.Locator("#missing").Or(page.Locator("#a"))
	text, err := loc.TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "first" {
		t.Errorf("expected 'first', got %q", text)
	}
}

func TestPressSequentially(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp" type="text" />`)

	err := page.Locator("#inp").Focus()
	if err != nil {
		t.Fatal(err)
	}

	err = page.Locator("#inp").PressSequentially("abc")
	if err != nil {
		t.Fatal(err)
	}

	val, err := page.Locator("#inp").InputValue()
	if err != nil {
		t.Fatal(err)
	}
	if val != "abc" {
		t.Errorf("expected 'abc', got %q", val)
	}
}

func TestScrollIntoViewIfNeeded(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="el">scroll test</div>`)

	err := page.Locator("#el").ScrollIntoViewIfNeeded()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocatorTap(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<button id="btn" onclick="document.title='tapped'">Tap</button>`)

	err := page.Locator("#btn").Tap()
	if err != nil {
		t.Fatal(err)
	}
}

func TestLocatorBlur(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp" type="text" />`)

	err := page.Locator("#inp").Focus()
	if err != nil {
		t.Fatal(err)
	}

	err = page.Locator("#inp").Blur()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetChecked(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="cb" type="checkbox" />`)

	err := page.Locator("#cb").SetChecked(true)
	if err != nil {
		t.Fatal(err)
	}

	checked, err := page.Locator("#cb").IsChecked()
	if err != nil {
		t.Fatal(err)
	}
	if !checked {
		t.Errorf("expected checkbox to be checked after SetChecked(true)")
	}

	err = page.Locator("#cb").SetChecked(false)
	if err != nil {
		t.Fatal(err)
	}

	checked, err = page.Locator("#cb").IsChecked()
	if err != nil {
		t.Fatal(err)
	}
	if checked {
		t.Errorf("expected checkbox to be unchecked after SetChecked(false)")
	}
}

func TestLocatorAll(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<ul><li>a</li><li>b</li><li>c</li></ul>`)

	locators, err := page.Locator("li").All()
	if err != nil {
		t.Fatal(err)
	}
	if len(locators) != 3 {
		t.Errorf("expected 3 locators, got %d", len(locators))
	}
}

func TestGetByAltText(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<img alt="Logo" src="" />`)

	vis, err := page.GetByAltText("Logo").IsVisible()
	if err != nil {
		t.Fatal(err)
	}
	_ = vis
}

func TestGetByTitle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<span title="Tooltip text">hover me</span>`)

	text, err := page.GetByTitle("Tooltip").TextContent()
	if err != nil {
		t.Fatal(err)
	}
	if text != "hover me" {
		t.Errorf("expected 'hover me', got %q", text)
	}
}

func TestLocatorHighlight(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="el">highlight me</div>`)

	err := page.Locator("#el").Highlight()
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateAll(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<ul><li>a</li><li>b</li><li>c</li></ul>`)

	val, err := page.Locator("li").EvaluateAll("function(els){ return els.length; }")
	if err != nil {
		t.Fatal(err)
	}
	var count int
	json.Unmarshal(val, &count)
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}
}

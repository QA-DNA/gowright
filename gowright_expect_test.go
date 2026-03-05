package gowright_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright/pkg/expect"
	"github.com/QA-DNA/gowright/pkg/runner"
)

func TestExpectToBeVisible(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="vis">visible</div><div id="hid" style="display:none">hidden</div>`)
	if err := expect.Locator(page.Locator("#vis").First()).ToBeVisible(); err != nil {
		t.Fatal(err)
	}
	if err := expect.Locator(page.Locator("#hid").First()).Not().ToBeVisible(); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveText(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="msg">hello world</div>`)
	if err := expect.Locator(page.Locator("#msg").First()).ToHaveText("hello world"); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToContainText(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="msg">hello world</div>`)
	if err := expect.Locator(page.Locator("#msg").First()).ToContainText("world"); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveValue(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input id="inp" value="test">`)
	if err := expect.Locator(page.Locator("#inp").First()).ToHaveValue("test"); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToBeChecked(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<input type="checkbox" id="cb" checked>`)
	if err := expect.Locator(page.Locator("#cb").First()).ToBeChecked(); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveCount(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<ul><li>a</li><li>b</li><li>c</li></ul>`)
	if err := expect.Locator(page.Locator("li").First()).ToHaveCount(3); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveTitle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<title>Test Page</title><div>content</div>`)
	if err := expect.Page(page).ToHaveTitle("Test Page"); err != nil {
		t.Fatal(err)
	}
}

func TestExpectToHaveURL(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div>content</div>`)
	if err := expect.Page(page).ToHaveURL("data:text/html"); err != nil {
		t.Fatal(err)
	}
}

func TestExpectAutoRetry(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="target" style="display:none">hidden</div>`)
	go func() {
		time.Sleep(500 * time.Millisecond)
		page.Evaluate(`document.getElementById('target').style.display = 'block'`)
	}()
	if err := expect.Locator(page.Locator("#target").First()).WithTimeout(3 * time.Second).ToBeVisible(); err != nil {
		t.Fatal(err)
	}
}

func TestExpectNotAutoRetry(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="target">visible</div>`)
	go func() {
		time.Sleep(500 * time.Millisecond)
		page.Evaluate(`document.getElementById('target').style.display = 'none'`)
	}()
	if err := expect.Locator(page.Locator("#target").First()).Not().WithTimeout(3 * time.Second).ToBeVisible(); err != nil {
		t.Fatal(err)
	}
}

func TestExpectTimeout(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, `<div id="target" style="display:none">hidden</div>`)
	err := expect.Locator(page.Locator("#target").First()).WithTimeout(500 * time.Millisecond).ToBeVisible()
	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRunnerFixture(t *testing.T) {
	t.Parallel()
	f := runner.New(t)
	if err := f.Page.Goto("data:text/html,<div>fixture test</div>"); err != nil {
		t.Fatal(err)
	}
	title, _ := f.Page.Title()
	_ = title
}

func TestRunnerNewPage(t *testing.T) {
	t.Parallel()
	f := runner.New(t)
	p2 := f.NewPage()
	if err := p2.Goto("data:text/html,<div>second page</div>"); err != nil {
		t.Fatal(err)
	}
}

func TestHTMLReporter(t *testing.T) {
	t.Parallel()
	r := runner.NewReport("Test Suite")
	r.Add("TestOne", "pass", 100*time.Millisecond, "")
	r.Add("TestTwo", "fail", 200*time.Millisecond, "assertion failed: expected true")
	r.Add("TestThree", "skip", 0, "")

	tmpDir := t.TempDir()
	htmlPath := tmpDir + "/report.html"
	if err := r.WriteHTML(htmlPath); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(htmlPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "TestOne") {
		t.Error("report missing TestOne")
	}
	if !strings.Contains(content, "assertion failed") {
		t.Error("report missing error message")
	}
	if !strings.Contains(content, "1 passed") {
		t.Error("report missing pass count")
	}

	jsonPath := tmpDir + "/report.json"
	if err := r.WriteJSON(jsonPath); err != nil {
		t.Fatal(err)
	}
	jsonData, _ := os.ReadFile(jsonPath)
	if !strings.Contains(string(jsonData), "TestTwo") {
		t.Error("JSON report missing TestTwo")
	}
}

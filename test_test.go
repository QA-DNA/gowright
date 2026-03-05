package gowright_test

import (
	"testing"
	"time"

	"github.com/PeterStoica/gowright"
)

func TestNewTestDefaults(t *testing.T) {
	tt := gowright.NewTest()
	if tt == nil {
		t.Fatal("NewTest() returned nil")
	}
}

func TestNewTestWithConfig(t *testing.T) {
	tt := gowright.NewTest(gowright.TestConfig{
		BaseURL:  "http://localhost:3000",
		Headless: true,
		Timeout:  60 * time.Second,
		SlowMo:   100 * time.Millisecond,
		Viewport: &gowright.Viewport{Width: 1280, Height: 720},
	})
	if tt == nil {
		t.Fatal("NewTest(config) returned nil")
	}
}

func TestRunSimple(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "navigates to data URL and checks title", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>Hello</title><h1>World</h1>")
		title := pw.Page.Title()
		if title != "Hello" {
			t.Errorf("expected title %q, got %q", "Hello", title)
		}
	})
}

func TestRunLocator(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "locator TextContent works", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<h1 id='heading'>GoWright</h1>")
		heading := pw.Page.Locator("#heading")
		text := heading.TextContent()
		if text != "GoWright" {
			t.Errorf("expected %q, got %q", "GoWright", text)
		}
	})
}

func TestRunLocatorClick(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "click changes title", func(pw *gowright.TestContext) {
		pw.Page.Goto(`data:text/html,<button onclick="document.title='Clicked'">Click me</button>`)
		pw.Page.Locator("button").Click()
		title := pw.Page.Title()
		if title != "Clicked" {
			t.Errorf("expected title %q, got %q", "Clicked", title)
		}
	})
}

func TestRunLocatorFill(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "fill input and check InputValue", func(pw *gowright.TestContext) {
		pw.Page.Goto(`data:text/html,<input id="name" type="text"/>`)
		input := pw.Page.Locator("#name")
		input.Fill("Alice")
		val := input.InputValue()
		if val != "Alice" {
			t.Errorf("expected %q, got %q", "Alice", val)
		}
	})
}

func TestExpectToHaveTextWrapper(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "Expect locator ToHaveText", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<span id='msg'>Hello World</span>")
		loc := pw.Page.Locator("#msg")
		pw.Expect(loc).ToHaveText("Hello World")
	})
}

func TestExpectToBeVisibleWrapper(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "Expect locator ToBeVisible", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<div id='box'>Visible</div>")
		loc := pw.Page.Locator("#box")
		pw.Expect(loc).ToBeVisible()
	})
}

func TestExpectNotToBeVisible(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "Expect locator Not ToBeVisible", func(pw *gowright.TestContext) {
		pw.Page.Goto(`data:text/html,<div id='hidden' style='display:none'>Hidden</div>`)
		loc := pw.Page.Locator("#hidden")
		pw.Expect(loc).Not().ToBeVisible()
	})
}

func TestExpectPageTitle(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "Expect page ToHaveTitle", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>My Page</title>")
		pw.Expect(pw.Page).ToHaveTitle("My Page")
	})
}

func TestExpectPageURL(t *testing.T) {
	tt := gowright.NewTest()
	tt.Run(t, "Expect page ToHaveURL", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>URL Test</title>")
		pw.Expect(pw.Page).ToHaveURL("data:text/html")
	})
}

func TestDescribeWithHooks(t *testing.T) {
	test := gowright.NewTest()
	test.Describe(t, "suite with hooks", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			pw.Page.Goto("data:text/html,<title>Hook Page</title><div id='content'>ready</div>")
		})

		s.Test("first test", func(pw *gowright.TestContext) {
			title := pw.Page.Title()
			if title != "Hook Page" {
				t.Errorf("expected 'Hook Page', got %q", title)
			}
		})

		s.Test("second test", func(pw *gowright.TestContext) {
			text := pw.Page.Locator("#content").TextContent()
			if text != "ready" {
				t.Errorf("expected 'ready', got %q", text)
			}
		})
	})
}

func TestDescribeNested(t *testing.T) {
	test := gowright.NewTest()
	test.Describe(t, "outer", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			pw.Page.Goto("data:text/html,<title>Nested</title>")
		})

		s.Describe("inner", func(s *gowright.Suite) {
			s.Test("has title", func(pw *gowright.TestContext) {
				title := pw.Page.Title()
				if title != "Nested" {
					t.Errorf("expected 'Nested', got %q", title)
				}
			})
		})
	})
}

func TestPlaywrightStyle(t *testing.T) {
	test := gowright.NewTest()

	test.Run(t, "simple navigation", func(pw *gowright.TestContext) {
		pw.Page.Goto("data:text/html,<title>Example</title><h1>Hello World</h1><p>Content here</p>")
		pw.Expect(pw.Page).ToHaveTitle("Example")
		pw.Expect(pw.Page.Locator("h1")).ToHaveText("Hello World")
		pw.Expect(pw.Page.Locator("p")).ToContainText("Content")
	})

	test.Describe(t, "form interactions", func(s *gowright.Suite) {
		s.BeforeEach(func(pw *gowright.TestContext) {
			pw.Page.Goto(`data:text/html,
				<form>
					<input id="name" type="text" placeholder="Name"/>
					<input id="email" type="email" placeholder="Email"/>
					<input id="agree" type="checkbox"/>
					<button type="submit" onclick="event.preventDefault();document.title='submitted'">Submit</button>
				</form>`)
		})

		s.Test("fill and submit", func(pw *gowright.TestContext) {
			pw.Page.Locator("#name").Fill("John Doe")
			pw.Page.Locator("#email").Fill("john@example.com")
			pw.Page.Locator("#agree").Check()
			pw.Page.Locator("button").Click()

			pw.Expect(pw.Page.Locator("#name")).ToHaveValue("John Doe")
			pw.Expect(pw.Page.Locator("#agree")).ToBeChecked()
			pw.Expect(pw.Page).ToHaveTitle("submitted")
		})

		s.Test("clear input", func(pw *gowright.TestContext) {
			pw.Page.Locator("#name").Fill("temp")
			pw.Page.Locator("#name").Clear()
			pw.Expect(pw.Page.Locator("#name")).ToHaveValue("")
		})
	})
}

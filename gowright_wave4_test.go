package gowright_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

func TestContextAddInitScript(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	bc.AddInitScript("window.__injected = true")

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.Goto("data:text/html,<div>test</div>"); err != nil {
		t.Fatal(err)
	}

	val, err := page.Evaluate("window.__injected")
	if err != nil {
		t.Fatal(err)
	}

	var result bool
	json.Unmarshal(val, &result)
	if !result {
		t.Errorf("expected window.__injected to be true, got %s", string(val))
	}
}

func TestContextGrantPermissions(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	if err := bc.GrantPermissions([]string{"geolocation"}); err != nil {
		t.Fatal(err)
	}
}

func TestContextClearPermissions(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	if err := bc.GrantPermissions([]string{"geolocation"}); err != nil {
		t.Fatal(err)
	}

	if err := bc.ClearPermissions(); err != nil {
		t.Fatal(err)
	}
}

func TestContextSetGeolocation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	_, err = bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	err = bc.SetGeolocation(&gowright.Geolocation{
		Latitude:  48.8566,
		Longitude: 2.3522,
		Accuracy:  1,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContextSetOffline(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	page.Goto("data:text/html,<div>offline test</div>")

	if err := bc.SetOffline(true); err != nil {
		t.Fatal(err)
	}

	val, err := page.Evaluate(`fetch('https://example.com').then(() => 'ok').catch(() => 'failed')`)
	if err != nil {
		t.Fatal(err)
	}

	var result string
	json.Unmarshal(val, &result)
	if result != "failed" {
		t.Errorf("expected fetch to fail in offline mode, got %q", result)
	}

	if err := bc.SetOffline(false); err != nil {
		t.Fatal(err)
	}
}

func TestContextSetExtraHTTPHeaders(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	_, err = bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	err = bc.SetExtraHTTPHeaders(map[string]string{
		"X-Custom-Header": "test-value",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContextStorageState(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	_, err = bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	ss, err := bc.StorageState()
	if err != nil {
		t.Fatal(err)
	}

	if ss == nil {
		t.Fatal("storage state is nil")
	}

	if ss.Cookies == nil {
		t.Error("expected cookies to be a slice, got nil")
	}
}

func TestContextSetDefaultTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	bc.SetDefaultTimeout(10 * time.Second)
}

func TestContextRoute(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { bc.Close() })

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	intercepted := make(chan bool, 1)
	bc.Route("**example.com**", func(route *gowright.Route, req *gowright.Request) {
		select {
		case intercepted <- true:
		default:
		}
		route.Fulfill(gowright.FulfillOptions{
			Status:      200,
			ContentType: "text/html",
			Body:        []byte(`<html><body>intercepted</body></html>`),
		})
	})

	page.Goto("http://example.com")

	select {
	case <-intercepted:
	case <-time.After(5 * time.Second):
		t.Fatal("context route was not intercepted")
	}

	content, err := page.Content()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "intercepted") {
		t.Errorf("expected page to contain 'intercepted', got %s", content[:min(200, len(content))])
	}
}

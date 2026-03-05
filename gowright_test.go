package gowright_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QA-DNA/gowright"
)

func TestLaunchAndNavigate(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	version, err := b.Version()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Browser: %s", version)

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.Goto("https://example.com"); err != nil {
		t.Fatal(err)
	}

	title, err := page.Title()
	if err != nil {
		t.Fatal(err)
	}

	if title != "Example Domain" {
		t.Errorf("expected title 'Example Domain', got %q", title)
	}
}

func TestEvaluateJS(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.Goto("https://example.com"); err != nil {
		t.Fatal(err)
	}

	val, err := page.Evaluate(`document.querySelector("h1").textContent`)
	if err != nil {
		t.Fatal(err)
	}

	// val is JSON, so it includes quotes
	if string(val) != `"Example Domain"` {
		t.Errorf("expected h1 text '\"Example Domain\"', got %s", string(val))
	}
}

func TestScreenshot(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()

	page, err := bc.NewPage()
	if err != nil {
		t.Fatal(err)
	}

	if err := page.Goto("https://example.com"); err != nil {
		t.Fatal(err)
	}

	data, err := page.Screenshot()
	if err != nil {
		t.Fatal(err)
	}

	if len(data) < 100 {
		t.Errorf("screenshot too small: %d bytes", len(data))
	}

	// PNG magic bytes
	if data[0] != 0x89 || data[1] != 'P' || data[2] != 'N' || data[3] != 'G' {
		t.Error("screenshot is not a valid PNG")
	}
}

func TestMultiplePages(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	b, err := gowright.Launch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()

	bc, err := b.NewContext()
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()

	// Create 3 pages concurrently
	pages := make([]*gowright.Page, 3)
	for i := range pages {
		p, err := bc.NewPage()
		if err != nil {
			t.Fatal(err)
		}
		pages[i] = p
	}

	if len(bc.Pages()) != 3 {
		t.Errorf("expected 3 pages, got %d", len(bc.Pages()))
	}
}

func TestParallelBrowsers(t *testing.T) {
	t.Parallel()
	const N = 5
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errors := make(chan error, N)
	times := make(chan time.Duration, N)

	start := time.Now()

	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			t.Logf("[%v] browser %d: starting", time.Since(start).Round(time.Millisecond), idx)
			bStart := time.Now()

			b, err := gowright.Launch(ctx)
			if err != nil {
				errors <- fmt.Errorf("browser %d launch: %w", idx, err)
				return
			}
			defer b.Close()

			bc, err := b.NewContext()
			if err != nil {
				errors <- fmt.Errorf("browser %d context: %w", idx, err)
				return
			}
			defer bc.Close()

			page, err := bc.NewPage()
			if err != nil {
				errors <- fmt.Errorf("browser %d page: %w", idx, err)
				return
			}

			html := fmt.Sprintf(`<title>Browser %d</title><h1>Hello from %d</h1>`, idx, idx)
			if err := page.Goto("data:text/html," + html); err != nil {
				errors <- fmt.Errorf("browser %d goto: %w", idx, err)
				return
			}

			title, err := page.Title()
			if err != nil {
				errors <- fmt.Errorf("browser %d title: %w", idx, err)
				return
			}

			expected := fmt.Sprintf("Browser %d", idx)
			if title != expected {
				errors <- fmt.Errorf("browser %d: expected title %q, got %q", idx, expected, title)
				return
			}

			d := time.Since(bStart)
			t.Logf("[%v] browser %d: done in %v", time.Since(start).Round(time.Millisecond), idx, d.Round(time.Millisecond))
			times <- d
		}(i)
	}

	wg.Wait()
	close(errors)
	close(times)

	for err := range errors {
		t.Error(err)
	}

	total := time.Since(start)
	var sum time.Duration
	var count int
	for d := range times {
		sum += d
		count++
	}

	t.Logf("Launched %d browsers in parallel:", N)
	t.Logf("  Wall time:  %v", total)
	t.Logf("  Sum of all: %v", sum)
	t.Logf("  Avg each:   %v", sum/time.Duration(count))
	t.Logf("  Speedup:    %.1fx", float64(sum)/float64(total))
}

package gowright_test

import (
	"encoding/json"
	"testing"
)

func TestJSHandle(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	handle, err := page.EvaluateHandle("({foo: 42})")
	if err != nil {
		t.Fatal(err)
	}

	val, err := handle.JsonValue()
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]int
	if err := json.Unmarshal(val, &result); err != nil {
		t.Fatal(err)
	}
	if result["foo"] != 42 {
		t.Errorf("expected foo to be 42, got %d", result["foo"])
	}

	if err := handle.Dispose(); err != nil {
		t.Fatal(err)
	}
}

func TestJSHandleGetProperties(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	handle, err := page.EvaluateHandle("({a: 1, b: 2})")
	if err != nil {
		t.Fatal(err)
	}

	props, err := handle.GetProperties()
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := props["a"]; !ok {
		t.Error("expected property 'a' in properties")
	}
	if _, ok := props["b"]; !ok {
		t.Error("expected property 'b' in properties")
	}

	if err := handle.Dispose(); err != nil {
		t.Fatal(err)
	}
}

func TestTracingType(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	tracing := page.Tracing()
	if tracing == nil {
		t.Fatal("tracing is nil")
	}
}

func TestCoverageType(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	cov := page.Coverage()
	if cov == nil {
		t.Fatal("coverage is nil")
	}
}

func TestCoverageJS(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	cov := page.Coverage()
	if err := cov.StartJSCoverage(); err != nil {
		t.Fatal(err)
	}

	page.Evaluate("1+1")

	entries, err := cov.StopJSCoverage()
	if err != nil {
		t.Fatal(err)
	}
	_ = entries
}

func TestWorkersAccessor(t *testing.T) {
	t.Parallel()
	_, page := setupPage(t, "<div>test</div>")

	workers := page.Workers()
	if len(workers) != 0 {
		t.Errorf("expected 0 workers, got %d", len(workers))
	}
}

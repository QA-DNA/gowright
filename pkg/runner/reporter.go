package runner

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"time"
)

type TestResult struct {
	Name     string        `json:"name"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Error    string        `json:"error,omitempty"`
}

type Report struct {
	Title     string        `json:"title"`
	StartTime time.Time    `json:"startTime"`
	Duration  time.Duration `json:"duration"`
	Results   []TestResult  `json:"results"`
}

func NewReport(title string) *Report {
	return &Report{
		Title:     title,
		StartTime: time.Now(),
	}
}

func (r *Report) Add(name, status string, duration time.Duration, err string) {
	r.Results = append(r.Results, TestResult{
		Name:     name,
		Status:   status,
		Duration: duration,
		Error:    err,
	})
}

func (r *Report) Finish() {
	r.Duration = time.Since(r.StartTime)
}

func (r *Report) WriteHTML(path string) error {
	r.Finish()

	passed, failed, skipped := 0, 0, 0
	for _, res := range r.Results {
		switch res.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		case "skip":
			skipped++
		}
	}

	h := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%s</title>
<style>
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
.header { background: #1a1a2e; color: white; padding: 20px 30px; border-radius: 8px; margin-bottom: 20px; }
.header h1 { margin: 0 0 10px 0; }
.stats { display: flex; gap: 20px; }
.stat { padding: 4px 12px; border-radius: 4px; font-weight: 600; }
.stat-pass { background: #27ae60; }
.stat-fail { background: #e74c3c; }
.stat-skip { background: #f39c12; }
.results { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
.result { padding: 12px 20px; border-bottom: 1px solid #eee; display: flex; align-items: center; gap: 12px; }
.result:last-child { border-bottom: none; }
.icon-pass { color: #27ae60; }
.icon-fail { color: #e74c3c; }
.icon-skip { color: #f39c12; }
.name { flex: 1; font-weight: 500; }
.duration { color: #888; font-size: 0.9em; }
.error { background: #fdf0f0; color: #c0392b; padding: 8px 20px; font-family: monospace; font-size: 0.85em; white-space: pre-wrap; }
</style>
</head>
<body>
<div class="header">
<h1>%s</h1>
<div class="stats">
<span class="stat stat-pass">%d passed</span>
<span class="stat stat-fail">%d failed</span>
<span class="stat stat-skip">%d skipped</span>
<span class="stat" style="background:#555">%s</span>
</div>
</div>
<div class="results">`,
		html.EscapeString(r.Title),
		html.EscapeString(r.Title),
		passed, failed, skipped,
		r.Duration.Round(time.Millisecond))

	for _, res := range r.Results {
		icon := "&#10003;"
		iconClass := "icon-pass"
		switch res.Status {
		case "fail":
			icon = "&#10007;"
			iconClass = "icon-fail"
		case "skip":
			icon = "&#9679;"
			iconClass = "icon-skip"
		}
		h += fmt.Sprintf(`
<div class="result">
<span class="%s">%s</span>
<span class="name">%s</span>
<span class="duration">%s</span>
</div>`,
			iconClass, icon,
			html.EscapeString(res.Name),
			res.Duration.Round(time.Millisecond))
		if res.Error != "" {
			h += fmt.Sprintf(`<div class="error">%s</div>`, html.EscapeString(res.Error))
		}
	}

	h += `
</div>
</body>
</html>`

	return os.WriteFile(path, []byte(h), 0o644)
}

func (r *Report) WriteJSON(path string) error {
	r.Finish()
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

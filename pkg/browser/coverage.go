package browser

import (
	"encoding/json"
	"fmt"
)

type Coverage struct {
	page *Page
}

func (p *Page) Coverage() *Coverage {
	if p.coverage == nil {
		p.coverage = &Coverage{page: p}
	}
	return p.coverage
}

type CoverageEntry struct {
	URL    string          `json:"url"`
	Text   string          `json:"text"`
	Ranges []CoverageRange `json:"ranges"`
}

type CoverageRange struct {
	Start int `json:"startOffset"`
	End   int `json:"endOffset"`
}

func (c *Coverage) StartJSCoverage(opts ...JSCoverageOptions) error {
	ctx := c.page.browser.ctx
	_, err := c.page.session.Call(ctx, "Profiler.enable", nil)
	if err != nil {
		return fmt.Errorf("profiler enable: %w", err)
	}
	params := map[string]any{
		"callCount": true,
		"detailed":  true,
	}
	if len(opts) > 0 && opts[0].ResetOnNavigation {
		params["resetOnNavigation"] = true
	}
	_, err = c.page.session.Call(ctx, "Profiler.startPreciseCoverage", params)
	return err
}

func (c *Coverage) StopJSCoverage() ([]CoverageEntry, error) {
	ctx := c.page.browser.ctx
	result, err := c.page.session.Call(ctx, "Profiler.takePreciseCoverage", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Result []struct {
			ScriptID string `json:"scriptId"`
			URL      string `json:"url"`
			Functions []struct {
				Ranges []struct {
					StartOffset int `json:"startOffset"`
					EndOffset   int `json:"endOffset"`
					Count       int `json:"count"`
				} `json:"ranges"`
			} `json:"functions"`
		} `json:"result"`
	}
	json.Unmarshal(result, &resp)

	c.page.session.Call(ctx, "Profiler.stopPreciseCoverage", nil)
	c.page.session.Call(ctx, "Profiler.disable", nil)

	entries := make([]CoverageEntry, 0, len(resp.Result))
	for _, r := range resp.Result {
		if r.URL == "" {
			continue
		}
		entry := CoverageEntry{URL: r.URL}
		for _, fn := range r.Functions {
			for _, rng := range fn.Ranges {
				if rng.Count > 0 {
					entry.Ranges = append(entry.Ranges, CoverageRange{Start: rng.StartOffset, End: rng.EndOffset})
				}
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (c *Coverage) StartCSSCoverage(opts ...CSSCoverageOptions) error {
	ctx := c.page.browser.ctx
	_, err := c.page.session.Call(ctx, "CSS.enable", nil)
	if err != nil {
		return fmt.Errorf("css enable: %w", err)
	}
	params := map[string]any{}
	if len(opts) > 0 && opts[0].ResetOnNavigation {
		params["resetOnNavigation"] = true
	}
	_, err = c.page.session.Call(ctx, "CSS.startRuleUsageTracking", params)
	return err
}

func (c *Coverage) StopCSSCoverage() ([]CoverageEntry, error) {
	ctx := c.page.browser.ctx
	result, err := c.page.session.Call(ctx, "CSS.stopRuleUsageTracking", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		RuleUsage []struct {
			StyleSheetID string `json:"styleSheetId"`
			StartOffset  int    `json:"startOffset"`
			EndOffset    int    `json:"endOffset"`
			Used         bool   `json:"used"`
		} `json:"ruleUsage"`
	}
	json.Unmarshal(result, &resp)

	c.page.session.Call(ctx, "CSS.disable", nil)

	entryMap := make(map[string]*CoverageEntry)
	for _, r := range resp.RuleUsage {
		if !r.Used {
			continue
		}
		entry, ok := entryMap[r.StyleSheetID]
		if !ok {
			entry = &CoverageEntry{URL: r.StyleSheetID}
			entryMap[r.StyleSheetID] = entry
		}
		entry.Ranges = append(entry.Ranges, CoverageRange{Start: r.StartOffset, End: r.EndOffset})
	}

	entries := make([]CoverageEntry, 0, len(entryMap))
	for _, e := range entryMap {
		entries = append(entries, *e)
	}
	return entries, nil
}

type JSCoverageOptions struct {
	ResetOnNavigation bool
}

type CSSCoverageOptions struct {
	ResetOnNavigation bool
}

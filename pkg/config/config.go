// Package config handles loading and applying gowright.config.json.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Config is the top-level Gowright configuration.
type Config struct {
	// Headless runs browsers without a visible window. Default: true.
	Headless *bool `json:"headless,omitempty"`

	// SlowMo adds a delay before each action (e.g. "500ms", "1s").
	SlowMo string `json:"slowMo,omitempty"`

	// Timeout is the default timeout for actions and navigation (e.g. "30s").
	Timeout string `json:"timeout,omitempty"`

	// Viewport sets the default browser viewport.
	Viewport *Viewport `json:"viewport,omitempty"`

	// BaseURL is prepended to relative URLs in page.Goto().
	BaseURL string `json:"baseURL,omitempty"`

	// Workers is the max number of parallel test workers. 0 = number of CPUs.
	Workers int `json:"workers,omitempty"`

	// Browser selects the browser engine. Currently only "chromium".
	Browser string `json:"browser,omitempty"`

	// LaunchArgs are extra Chrome CLI flags.
	LaunchArgs []string `json:"launchArgs,omitempty"`

	// NoSandbox disables the Chrome sandbox (for Docker/CI).
	NoSandbox bool `json:"noSandbox,omitempty"`

	// Projects define multiple test configurations (e.g. different viewports).
	Projects []Project `json:"projects,omitempty"`
}

// Viewport defines browser dimensions.
type Viewport struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Project is a named test configuration, like Playwright's projects.
type Project struct {
	Name     string    `json:"name"`
	Headless *bool     `json:"headless,omitempty"`
	Viewport *Viewport `json:"viewport,omitempty"`
	BaseURL  string    `json:"baseURL,omitempty"`
	Browser  string    `json:"browser,omitempty"`
}

// Defaults returns a config with sensible defaults.
func Defaults() Config {
	headless := true
	return Config{
		Headless: &headless,
		Timeout:  "30s",
		Viewport: &Viewport{Width: 1280, Height: 720},
		Browser:  "chromium",
	}
}

// Load reads gowright.config.json, searching from the given directory (or cwd)
// upward to the filesystem root, like Playwright does.
// If no config file exists, returns defaults.
func Load(dir ...string) (Config, error) {
	d := "."
	if len(dir) > 0 {
		d = dir[0]
	}

	abs, err := filepath.Abs(d)
	if err != nil {
		abs = d
	}

	cfg := Defaults()

	// Search upward for gowright.config.json
	for {
		path := filepath.Join(abs, "gowright.config.json")
		data, err := os.ReadFile(path)
		if err == nil {
			if err := json.Unmarshal(data, &cfg); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break // reached root
		}
		abs = parent
	}

	return cfg, nil // no config found, use defaults
}

// IsHeadless returns whether headless mode is enabled.
func (c Config) IsHeadless() bool {
	if c.Headless == nil {
		return true
	}
	return *c.Headless
}

// SlowMoDuration parses SlowMo as a time.Duration.
func (c Config) SlowMoDuration() time.Duration {
	if c.SlowMo == "" {
		return 0
	}
	d, _ := time.ParseDuration(c.SlowMo)
	return d
}

// TimeoutDuration parses Timeout as a time.Duration.
func (c Config) TimeoutDuration() time.Duration {
	if c.Timeout == "" {
		return 30 * time.Second
	}
	d, _ := time.ParseDuration(c.Timeout)
	if d == 0 {
		return 30 * time.Second
	}
	return d
}

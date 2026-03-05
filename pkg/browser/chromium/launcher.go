// Package chromium handles launching and connecting to Chromium browsers.
package chromium

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// LaunchOptions configure how Chrome is launched.
type LaunchOptions struct {
	// Bin is the path to the Chrome binary. Auto-detected if empty.
	Bin string

	// Headless runs Chrome without a visible window. Default: true.
	Headless bool

	// UserDataDir is the profile directory. Uses a temp dir if empty.
	UserDataDir string

	// Args are additional Chrome command-line arguments.
	Args []string

	// NoSandbox disables the Chrome sandbox. Required in Docker/CI.
	NoSandbox bool

	// SlowMo adds a delay (in milliseconds) before each action.
	// Useful for debugging headed mode.
	SlowMo time.Duration
}

// DefaultLaunchOptions returns sensible defaults.
func DefaultLaunchOptions() LaunchOptions {
	return LaunchOptions{
		Headless: true,
	}
}

// LaunchResult contains the launched browser process info.
type LaunchResult struct {
	// WSEndpoint is the WebSocket URL to connect to.
	WSEndpoint string

	// Process is the Chrome OS process.
	Process *os.Process

	// UserDataDir is the profile directory used (may be a temp dir).
	UserDataDir string

	// Cleanup kills the process and removes temp directories.
	Cleanup func()
}

// Launch starts a new Chrome process and returns the debugging WebSocket URL.
func Launch(ctx context.Context, opts LaunchOptions) (*LaunchResult, error) {
	bin := opts.Bin
	if bin == "" {
		var err error
		bin, err = FindChrome()
		if err != nil {
			return nil, err
		}
	}

	userDataDir := opts.UserDataDir
	tempDir := ""
	if userDataDir == "" {
		var err error
		tempDir, err = os.MkdirTemp("", "gowright-profile-*")
		if err != nil {
			return nil, fmt.Errorf("create temp profile dir: %w", err)
		}
		userDataDir = tempDir
	}

	args := buildArgs(opts, userDataDir)

	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = userDataDir

	// Capture stderr to parse the WebSocket URL
	stderr, err := cmd.StderrPipe()
	if err != nil {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	// Detach process on Unix so we can kill the process group
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	if err := cmd.Start(); err != nil {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		return nil, fmt.Errorf("start chrome: %w", err)
	}

	// Parse the DevTools WebSocket URL from stderr
	wsURL := ""
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "DevTools listening on ") {
			wsURL = strings.TrimPrefix(line, "DevTools listening on ")
			break
		}
	}

	if wsURL == "" {
		// Chrome didn't output a WS URL - kill it
		killProcess(cmd)
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
		return nil, fmt.Errorf("chrome did not output DevTools URL")
	}

	cleanup := func() {
		killProcess(cmd)
		cmd.Wait()
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	}

	return &LaunchResult{
		WSEndpoint:  wsURL,
		Process:     cmd.Process,
		UserDataDir: userDataDir,
		Cleanup:     cleanup,
	}, nil
}

// buildArgs constructs Chrome CLI arguments.
func buildArgs(opts LaunchOptions, userDataDir string) []string {
	args := []string{
		"--remote-debugging-port=0", // random port
		"--user-data-dir=" + userDataDir,
		"--no-first-run",
		"--no-startup-window",
		"--disable-background-networking",
		"--disable-background-timer-throttling",
		"--disable-backgrounding-occluded-windows",
		"--disable-breakpad",
		"--disable-client-side-phishing-detection",
		"--disable-component-extensions-with-background-pages",
		"--disable-component-update",
		"--disable-default-apps",
		"--disable-dev-shm-usage",
		"--disable-extensions",
		"--disable-hang-monitor",
		"--disable-ipc-flooding-protection",
		"--disable-popup-blocking",
		"--disable-prompt-on-repost",
		"--disable-renderer-backgrounding",
		"--disable-sync",
		"--enable-features=NetworkService,NetworkServiceInProcess",
		"--force-color-profile=srgb",
		"--metrics-recording-only",
		"--password-store=basic",
		"--use-mock-keychain",
	}

	if opts.Headless {
		args = append(args,
			"--headless=new",
			"--hide-scrollbars",
			"--mute-audio",
		)
	}

	if opts.NoSandbox {
		args = append(args, "--no-sandbox")
	}

	args = append(args, opts.Args...)
	return args
}

// FindChrome locates the Chrome binary on the system.
func FindChrome() (string, error) {
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Google Chrome Canary.app/Contents/MacOS/Google Chrome Canary",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		names := []string{
			"google-chrome",
			"google-chrome-stable",
			"chromium",
			"chromium-browser",
		}
		for _, name := range names {
			if p, err := exec.LookPath(name); err == nil {
				return p, nil
			}
		}
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chromium",
			"/usr/bin/chromium-browser",
		}
	case "windows":
		localAppData := os.Getenv("LOCALAPPDATA")
		programFiles := os.Getenv("PROGRAMFILES")
		programFilesX86 := os.Getenv("PROGRAMFILES(X86)")

		for _, base := range []string{localAppData, programFiles, programFilesX86} {
			if base != "" {
				paths = append(paths, filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("chrome not found. Install Chrome or set LaunchOptions.Bin")
}

func killProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	if runtime.GOOS == "windows" {
		cmd.Process.Kill()
	} else {
		// Kill the process group
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

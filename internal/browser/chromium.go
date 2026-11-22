package browser

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ChromiumCandidates lists Chromium-family browsers, most likely first.
func chromiumCandidates() []string {
	if runtime.GOOS == "darwin" {
		var out []string
		for _, app := range []string{
			"Chromium.app/Contents/MacOS/Chromium",
			"Google Chrome.app/Contents/MacOS/Google Chrome",
			"Brave Browser.app/Contents/MacOS/Brave Browser",
			"Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		} {
			home, _ := os.UserHomeDir()
			out = append(out, filepath.Join("/Applications", app),
				filepath.Join(home, "Applications", app))
		}
		return out
	}
	return []string{"chromium", "chromium-browser", "google-chrome",
		"google-chrome-stable", "brave-browser", "microsoft-edge"}
}

// FindChromiumBinary returns the browser to drive for login. The
// COURSESU_BROWSER environment variable overrides the search.
func FindChromiumBinary() string {
	if p := os.Getenv("COURSESU_BROWSER"); p != "" {
		return p
	}
	for _, c := range chromiumCandidates() {
		if filepath.IsAbs(c) {
			if _, err := os.Stat(c); err == nil {
				return c
			}
			continue
		}
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

// ChromiumProfileDir is the dedicated profile used for login. It persists
// between logins so the SSO can remember the device.
func ChromiumProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "coursesu", "chromium-profile")
}

// LaunchChromium starts a visible browser window on url with remote
// debugging on a random local port, and returns the DevTools WebSocket URL.
// It does not pass automation flags, so the window behaves like a normal
// browser and the person using it solves any human check themselves.
func LaunchChromium(ctx context.Context, binary, profile, url string) (*exec.Cmd, string, error) {
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return nil, "", err
	}
	portFile := filepath.Join(profile, "DevToolsActivePort")
	_ = os.Remove(portFile)
	cmd := exec.Command(binary,
		"--user-data-dir="+profile,
		"--remote-debugging-port=0",
		"--remote-allow-origins=http://127.0.0.1",
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
		url,
	)
	if err := cmd.Start(); err != nil {
		return nil, "", fmt.Errorf("launch %s: %w", binary, err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if ws, err := readDevToolsPort(portFile); err == nil {
			return cmd, ws, nil
		}
		select {
		case <-ctx.Done():
			_ = cmd.Process.Kill()
			return nil, "", ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	_ = cmd.Process.Kill()
	return nil, "", errors.New("browser did not open a DevTools port; " +
		"is another window already using this profile? Close it and retry")
}

func readDevToolsPort(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for sc.Scan() {
		lines = append(lines, strings.TrimSpace(sc.Text()))
	}
	if len(lines) < 2 || lines[0] == "" || lines[1] == "" {
		return "", errors.New("incomplete DevToolsActivePort")
	}
	return "ws://127.0.0.1:" + lines[0] + lines[1], nil
}

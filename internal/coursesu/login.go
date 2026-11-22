package coursesu

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/NicolasGuilloux/superu-mcp/internal/browser"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

func LoginFromCurl(_ context.Context, path string) (model.LoginResult, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("could not read %s: %v", path, err), nil)
	}
	if len(raw) == 0 {
		return model.LoginResult{}, newErr("empty input — no cURL to parse", nil)
	}
	cookies := browser.ParseCookiesFromCurl(string(raw))
	if len(cookies) == 0 {
		return model.LoginResult{}, newErr(
			"no Cookie header found in input. Make sure you copied the "+
				"full cURL with -H 'Cookie: ...'.", nil)
	}
	return finalizeLogin(cookies, "curl", nil, browser.ParseUserAgentFromCurl(string(raw)))
}

func LoginFirefox(ctx context.Context) (model.LoginResult, error) {
	firefox := browser.FindFirefoxBinary()
	if firefox == "" {
		return model.LoginResult{}, newErr(
			"firefox not found in PATH. Install Firefox or fall back to "+
				"`coursesu login --from-file curl.sh`.", nil)
	}
	profile, err := browser.EnsureProfile(firefox)
	if err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("could not provision Firefox profile: %v", err), nil)
	}
	fmt.Fprintf(os.Stderr,
		"Opening Firefox (profile: %s) on the coursesu login page...\n", profile)

	cmd := exec.Command(firefox, "--profile", profile,
		"--no-remote", "--new-instance", browser.LoginURL)
	cmd.Env = append(os.Environ(), "MOZ_NO_REMOTE=1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = detachAttr()
	if err := cmd.Start(); err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("failed to launch Firefox: %v", err), nil)
	}

	fmt.Fprintln(os.Stderr,
		"Log in to coursesu.com in the Firefox window, then press Enter "+
			"here to capture your session...")
	if !bufio.NewScanner(os.Stdin).Scan() {
		return model.LoginResult{}, newErr("login aborted", nil)
	}

	var store string
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if store = browser.FindSessionstoreFile(profile); store != "" {
			break
		}
		select {
		case <-ctx.Done():
			return model.LoginResult{}, newErr("login canceled", nil)
		case <-time.After(500 * time.Millisecond):
		}
	}
	if store == "" {
		return model.LoginResult{}, newErr(
			"could not find a sessionstore file in the Firefox profile. "+
				"Did Firefox fail to launch? Fall back to "+
				"`coursesu login --from-file curl.sh`.", nil)
	}

	rawJSON, err := browser.ReadMozLz4(store)
	if err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("failed to read sessionstore (%s): %v", store, err), nil)
	}
	var state any
	if err := json.Unmarshal(rawJSON, &state); err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("failed to parse sessionstore (%s): %v", store, err), nil)
	}
	cookies := browser.ExtractCoursesuCookies(state)
	if len(cookies) == 0 {
		return model.LoginResult{}, newErr(
			"no coursesu.com cookies found in the Firefox sessionstore. "+
				"Make sure you finished logging in before pressing Enter.",
			map[string]any{"sessionstore_path": store})
	}
	return finalizeLogin(cookies, "firefox", &store, "")
}

func finalizeLogin(cookies map[string]string, source string,
	sessionstorePath *string, userAgent string) (model.LoginResult, error) {
	var missing []string
	for _, name := range browser.RequiredCookies {
		if _, ok := cookies[name]; !ok {
			missing = append(missing, name)
		}
	}
	names := make([]string, 0, len(cookies))
	for k := range cookies {
		names = append(names, k)
	}
	sort.Strings(names)
	if len(missing) > 0 {
		sort.Strings(missing)
		return model.LoginResult{}, newErr(
			fmt.Sprintf("session cookies missing: %v. Finish login and "+
				"re-run `coursesu login`.", missing),
			map[string]any{
				"source":           source,
				"cookies_captured": len(cookies),
				"cookie_names":     names,
			})
	}
	if err := session.SaveWithUserAgent(cookies, 0, userAgent); err != nil {
		return model.LoginResult{}, newErr(
			fmt.Sprintf("could not write session file: %v", err), nil)
	}
	return model.LoginResult{
		Status:           "ok",
		Source:           source,
		CookiesCaptured:  len(cookies),
		SessionFile:      session.SessionFile,
		CookieNames:      names,
		SessionstorePath: sessionstorePath,
	}, nil
}

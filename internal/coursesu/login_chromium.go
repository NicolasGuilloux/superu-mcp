package coursesu

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

	"github.com/NicolasGuilloux/superu-mcp/internal/browser"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
)

const (
	ssoHost       = "moncompte.magasins-u.com"
	loginTimeout  = 5 * time.Minute
	coursesuHost  = "www.coursesu.com"
	emailSelector = "input[type=email]"
	passSelector  = "input[type=password]"
)

// LoginChromium opens a Chromium-family browser on the login page, fills in
// the email and password when given, and waits for the person to complete
// the human check and submit. Once the SSO sends the browser back to
// coursesu.com, it saves the session cookies and the browser's User-Agent.
func LoginChromium(ctx context.Context, email, password string) (model.LoginResult, error) {
	bin := browser.FindChromiumBinary()
	if bin == "" {
		return model.LoginResult{}, newErr("no Chromium-family browser found. "+
			"Install Chromium or Chrome, or set COURSESU_BROWSER to its binary.", nil)
	}
	profile := browser.ChromiumProfileDir()
	cmd, ws, err := browser.LaunchChromium(ctx, bin, profile, browser.LoginURL)
	if err != nil {
		return model.LoginResult{}, newErr(err.Error(), nil)
	}
	b := rod.New().ControlURL(ws).Context(ctx)
	if err := b.Connect(); err != nil {
		_ = cmd.Process.Kill()
		return model.LoginResult{}, newErr(fmt.Sprintf("connect to browser: %v", err), nil)
	}
	defer func() {
		_ = b.Close()
		_ = cmd.Wait()
	}()

	fmt.Fprintln(os.Stderr, "A browser window is open on the coursesu.com login page.")
	if email != "" {
		fmt.Fprintln(os.Stderr, "The form will be filled in for you: tick the "+
			"\"Vérifiez que vous êtes humain\" box, then click Continuer.")
	} else {
		fmt.Fprintln(os.Stderr, "Log in there. This command finishes on its own once you are logged in.")
	}

	deadline := time.Now().Add(loginTimeout)
	filled, sawSSO := false, false
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return model.LoginResult{}, newErr("login canceled", nil)
		case <-time.After(time.Second):
		}
		page := loginPage(b)
		if page == nil {
			continue
		}
		info, err := page.Info()
		if err != nil {
			continue
		}
		u, err := url.Parse(info.URL)
		if err != nil {
			continue
		}
		switch {
		case u.Host == ssoHost:
			if !sawSSO {
				fmt.Fprintln(os.Stderr, "On the login form.")
			}
			sawSSO = true
			if email != "" && !filled {
				if filled = fillLogin(page, email, password); filled {
					fmt.Fprintln(os.Stderr, "Form filled in. Waiting for you to tick the box and click Continuer...")
				}
			}
		case u.Host == coursesuHost && sawSSO && !isLoginPath(u.Path):
			return captureSession(b, page)
		}
	}
	return model.LoginResult{}, newErr("login timed out after 5 minutes", nil)
}

func isLoginPath(p string) bool {
	return strings.Contains(p, "/connexion") || strings.Contains(p, "Login-")
}

// loginPage returns the tab that is on coursesu.com or the SSO.
func loginPage(b *rod.Browser) *rod.Page {
	pages, err := b.Pages()
	if err != nil {
		return nil
	}
	for _, p := range pages {
		info, err := p.Info()
		if err != nil {
			continue
		}
		if strings.Contains(info.URL, "coursesu.com") || strings.Contains(info.URL, ssoHost) {
			return p
		}
	}
	return nil
}

// fillLogin types the credentials once the form has rendered. It reports
// whether it filled the form; it never touches the human check.
func fillLogin(page *rod.Page, email, password string) bool {
	p := page.Timeout(2 * time.Second)
	emailEl, err := p.Element(emailSelector)
	if err != nil {
		return false
	}
	passEl, err := p.Element(passSelector)
	if err != nil {
		return false
	}
	if err := emailEl.Input(email); err != nil {
		return false
	}
	if password != "" {
		if err := passEl.Input(password); err != nil {
			return false
		}
	}
	return true
}

func captureSession(b *rod.Browser, page *rod.Page) (model.LoginResult, error) {
	res, err := proto.NetworkGetCookies{
		Urls: []string{"https://" + coursesuHost + "/"},
	}.Call(page)
	if err != nil {
		return model.LoginResult{}, newErr(fmt.Sprintf("read cookies: %v", err), nil)
	}
	cookies := map[string]string{}
	for _, c := range res.Cookies {
		cookies[c.Name] = c.Value
	}
	ua := ""
	if v, err := (proto.BrowserGetVersion{}).Call(b); err == nil {
		ua = v.UserAgent
	}
	return finalizeLogin(cookies, "chromium", nil, ua)
}

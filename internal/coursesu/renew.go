package coursesu

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

const (
	// ssoCookieName is the Magasins U SSO session cookie, as published by
	// /am/json/serverinfo/* ("cookieName").
	ssoCookieName     = "b870a934f1cc0fa"
	silentRedirectURI = "https://www.coursesu.com/on/demandware.store/Sites-DigitalU-Site/fr_FR/Login-OAuthSilentReentryForgerock"
)

// ErrNoSSOCookie means silent renewal is not set up.
var ErrNoSSOCookie = errors.New("no SSO cookie saved: see `coursesu login --sso-cookie`")

// RenewSession logs in again without any interaction, the way coursesu.com
// renews its own session: an OpenID Connect request with prompt=none, sent
// with the SSO session cookie. It works as long as the SSO session is alive.
func RenewSession(ctx context.Context) (model.LoginResult, error) {
	sso := session.SSOCookie()
	if sso == "" {
		return model.LoginResult{}, ErrNoSSOCookie
	}
	state := randomString(16)
	verifier := randomString(48)
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{
		"client_id":             {oidcClientID},
		"redirect_uri":          {silentRedirectURI},
		"response_type":         {"code"},
		"scope":                 {oidcScope},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"response_mode":         {"query"},
		"prompt":                {"none"},
		"acr_values":            {oidcACR},
	}
	resp, err := client.Get(ctx, oidcAuthorize+"?"+q.Encode(), client.BaseURL+"/",
		map[string]string{ssoCookieName: sso})
	if err != nil {
		return model.LoginResult{}, newErr(fmt.Sprintf("silent renewal failed: %v", err), nil)
	}
	loc, err := url.Parse(resp.Location)
	if resp.StatusCode != 302 || err != nil || !strings.HasPrefix(resp.Location, silentRedirectURI) {
		return model.LoginResult{}, newErr("silent renewal: unexpected SSO answer",
			map[string]any{"status": resp.StatusCode})
	}
	if e := loc.Query().Get("error"); e != "" {
		return model.LoginResult{}, newErr("silent renewal refused: the SSO session has ended, "+
			"log in again (login_start / coursesu login --link)",
			map[string]any{"sso_error": e, "sso_error_description": loc.Query().Get("error_description")})
	}
	if loc.Query().Get("state") != state || loc.Query().Get("code") == "" {
		return model.LoginResult{}, newErr("silent renewal: the SSO answer has no code for this request", nil)
	}
	jar, ua, err := establishSession(ctx, loc.Query().Get("code"), verifier, silentRedirectURI)
	if err != nil {
		return model.LoginResult{}, err
	}
	return finalizeLogin(jar, "silent-renew", nil, ua)
}

// isSessionExpired reports whether err means the coursesu session is gone.
func isSessionExpired(err error) bool {
	var se *client.SessionExpiredError
	return errors.As(err, &se)
}

// SessionStatus describes the saved session.
type SessionStatus struct {
	LoggedIn       bool   `json:"logged_in"`
	LoggedInSince  string `json:"logged_in_since,omitempty"`
	Age            string `json:"age,omitempty"`
	SilentRenewal  bool   `json:"silent_renewal"`
	SessionFile    string `json:"session_file"`
	ExpiredMessage string `json:"expired,omitempty"`
}

// Status checks whether the saved session still opens an account page.
// It does not renew anything by itself; wrap it in WithClearance for that.
func Status(ctx context.Context) (SessionStatus, error) {
	st := SessionStatus{SilentRenewal: session.SSOCookie() != "", SessionFile: session.SessionFile}
	if at := session.AcquiredAt(); at > 0 {
		t := time.Unix(at, 0)
		st.LoggedInSince = t.Format(time.RFC3339)
		st.Age = time.Since(t).Round(time.Minute).String()
	}
	cookies, err := session.RequireCookies()
	if err != nil {
		st.ExpiredMessage = err.Error()
		return st, nil
	}
	resp, err := client.Get(ctx, "/mon-compte/mes-commandes", client.BaseURL+"/", cookies)
	if err != nil {
		if isSessionExpired(err) {
			return st, err // lets WithClearance renew, then report
		}
		return st, err
	}
	if err := checkOK(resp, "account page"); err != nil {
		return st, err
	}
	session.Persist(cookies, resp.SetCookies)
	st.LoggedIn = true
	return st, nil
}

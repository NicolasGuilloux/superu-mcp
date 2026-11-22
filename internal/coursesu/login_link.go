package coursesu

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/flaresolverr"
	"github.com/NicolasGuilloux/superu-mcp/internal/model"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

// OpenID Connect settings of coursesu.com's web client, as published in the
// SitePreferences of its pages and in espaceclient/auth.js.
const (
	oidcAuthorize   = "https://moncompte.magasins-u.com/am/oauth2/authorize"
	oidcToken       = "https://moncompte.magasins-u.com/am/oauth2/access_token"
	oidcClientID    = "coursesu.com-web"
	oidcRedirectURI = "https://www.coursesu.com/on/demandware.store/Sites-DigitalU-Site/fr_FR/Login-OAuthReentryForgerock"
	oidcScope       = "openid profile email profile_detail espaceclient.sitehote"
	oidcACR         = "acr_oidc_cu"
	setSessionURL   = "https://www.coursesu.com/on/demandware.store/Sites-DigitalU-Site/fr_FR/EspaceClient-SetSessionUnified"
)

// pendingLogin holds the PKCE secret of a login started with LoginLink.
type pendingLogin struct {
	State     string `json:"state"`
	Verifier  string `json:"code_verifier"`
	CreatedAt int64  `json:"created_at"`
}

const pendingMaxAge = 30 * time.Minute

func pendingFile() string { return filepath.Join(session.ConfigDir, "pending_login.json") }

// LinkResult is what `login --link` prints.
type LinkResult struct {
	Status string `json:"status"`
	Link   string `json:"link"`
	Next   string `json:"next"`
}

func randomString(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// LoginLink starts an OpenID Connect login with PKCE, the way coursesu.com's
// own login page does, and returns the authorization link to open in any
// browser. The PKCE verifier stays on this machine.
func LoginLink(_ context.Context) (LinkResult, error) {
	stateBytes := make([]byte, 16)
	_, _ = rand.Read(stateBytes)
	p := pendingLogin{
		State:     hex.EncodeToString(stateBytes),
		Verifier:  randomString(48),
		CreatedAt: time.Now().Unix(),
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	q := url.Values{
		"client_id":             {oidcClientID},
		"redirect_uri":          {oidcRedirectURI},
		"response_type":         {"code"},
		"scope":                 {oidcScope},
		"state":                 {p.State},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(sum[:])},
		"code_challenge_method": {"S256"},
		"response_mode":         {"query"},
		"acr_values":            {oidcACR},
	}
	if err := os.MkdirAll(session.ConfigDir, 0o700); err != nil {
		return LinkResult{}, newErr(err.Error(), nil)
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(pendingFile(), b, 0o600); err != nil {
		return LinkResult{}, newErr(err.Error(), nil)
	}
	return LinkResult{
		Status: "ok",
		Link:   oidcAuthorize + "?" + q.Encode(),
		Next: "Before opening the link, block the callback so your browser does not use " +
			"the code: DevTools > Network request blocking (Chrome: Cmd/Ctrl+Shift+P, " +
			"\"Show Network request blocking\"), pattern *Login-OAuthReentryForgerock?* " +
			"(uBlock: ||www.coursesu.com^*Login-OAuthReentryForgerock$document). Open the " +
			"link in that tab and log in. Copy the blocked address, which contains code= " +
			"and state=, and run coursesu login --callback '<url>' right away: the code " +
			"expires within a few minutes.",
	}, nil
}

type tokenResponse struct {
	IDToken     string `json:"id_token"`
	AccessToken string `json:"access_token"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// LoginCallback finishes a login started with LoginLink: it exchanges the
// code for tokens, then hands them to coursesu.com like its callback page
// does, and saves the resulting session.
func LoginCallback(ctx context.Context, callback string) (model.LoginResult, error) {
	u, err := url.Parse(strings.TrimSpace(callback))
	if err != nil || u.Query().Get("code") == "" || u.Query().Get("state") == "" {
		return model.LoginResult{}, newErr("the callback URL must contain code= and state=", nil)
	}
	raw, err := os.ReadFile(pendingFile())
	if err != nil {
		return model.LoginResult{}, newErr("no pending login: run `coursesu login --link` first", nil)
	}
	var p pendingLogin
	if err := json.Unmarshal(raw, &p); err != nil || p.Verifier == "" {
		return model.LoginResult{}, newErr("no usable pending login: run `coursesu login --link` again", nil)
	}
	if time.Since(time.Unix(p.CreatedAt, 0)) > pendingMaxAge {
		return model.LoginResult{}, newErr("the login link expired: run `coursesu login --link` again", nil)
	}
	if u.Query().Get("state") != p.State {
		return model.LoginResult{}, newErr("this callback belongs to another login link: "+
			"use the link printed by your last `coursesu login --link`", nil)
	}

	// coursesu.com sits behind Cloudflare: get a clearance and an anonymous
	// session first, and use that browser's User-Agent for every request.
	home, err := flaresolverr.New().Get(ctx, "", client.BaseURL+"/drive/home")
	if err != nil {
		return model.LoginResult{}, newErr(err.Error(), nil)
	}
	jar := map[string]string{}
	for _, c := range home.Cookies {
		if strings.HasSuffix("www.coursesu.com", strings.TrimPrefix(c.Domain, ".")) {
			jar[c.Name] = c.Value
		}
	}
	client.SetUserAgent(home.UserAgent)

	resp, err := client.PostURLEncoded(ctx, oidcToken, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {u.Query().Get("code")},
		"redirect_uri":  {oidcRedirectURI},
		"client_id":     {oidcClientID},
		"code_verifier": {p.Verifier},
	})
	if err != nil {
		return model.LoginResult{}, newErr(fmt.Sprintf("token request failed: %v", err), nil)
	}
	var tok tokenResponse
	_ = json.Unmarshal([]byte(resp.Body), &tok)
	if resp.StatusCode != 200 || tok.IDToken == "" || tok.AccessToken == "" {
		return model.LoginResult{}, newErr("the SSO refused the code: it expired, was "+
			"already used (did the browser load the callback?), or comes from another link",
			map[string]any{"status": resp.StatusCode, "sso_error": tok.Error,
				"sso_error_description": tok.ErrorDesc, "body_preview": preview(resp.Body, 200)})
	}

	body, _ := json.Marshal(map[string]any{"id_token": tok.IDToken,
		"access_token": tok.AccessToken, "silentCB": false})
	sres, err := client.PostJSON(ctx, setSessionURL, string(body), oidcRedirectURI, jar)
	if err != nil {
		return model.LoginResult{}, newErr(fmt.Sprintf("coursesu session request failed: %v", err), nil)
	}
	jar = client.MergeCookies(jar, sres.SetCookies)
	var sessResp struct {
		Error    bool   `json:"error"`
		ErrorMsg string `json:"errorMsg"`
	}
	_ = json.Unmarshal([]byte(sres.Body), &sessResp)
	if sres.StatusCode != 200 || sessResp.Error {
		return model.LoginResult{}, newErr("coursesu.com refused the tokens",
			map[string]any{"status": sres.StatusCode, "coursesu_error": sessResp.ErrorMsg,
				"body_preview": preview(sres.Body, 200)})
	}

	// Check that an account page loads before saving anything.
	acc, err := client.Get(ctx, "/mon-compte/mes-commandes", client.BaseURL+"/", jar)
	if err != nil || acc.StatusCode != 200 {
		status := 0
		if acc != nil {
			status = acc.StatusCode
		}
		return model.LoginResult{}, newErr("logged in at the SSO, but coursesu.com's account "+
			"page did not load", map[string]any{"status": status, "cause": fmt.Sprint(err)})
	}
	jar = client.MergeCookies(jar, acc.SetCookies)
	_ = os.Remove(pendingFile())
	return finalizeLogin(jar, "link", nil, home.UserAgent)
}

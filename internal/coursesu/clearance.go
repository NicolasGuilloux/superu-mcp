package coursesu

import (
	"context"
	"errors"
	"strings"

	"github.com/NicolasGuilloux/superu-mcp/internal/client"
	"github.com/NicolasGuilloux/superu-mcp/internal/flaresolverr"
	"github.com/NicolasGuilloux/superu-mcp/internal/session"
)

// IsCloudflareBlock reports whether err looks like Cloudflare turning a
// request away, which a fresh clearance usually fixes.
func IsCloudflareBlock(err error) bool {
	if err == nil {
		return false
	}
	var d *DomainError
	if errors.As(err, &d) {
		if strings.Contains(d.Msg, "HTTP 403") {
			return true
		}
		if bp, ok := d.Extra["body_preview"].(string); ok &&
			(strings.Contains(bp, "Just a moment") || strings.Contains(bp, "Cloudflare")) {
			return true
		}
	}
	return strings.Contains(err.Error(), "HTTP 403")
}

// RefreshClearance asks FlareSolverr for a new Cloudflare clearance with the
// saved session cookies, and saves the new cookies and User-Agent. The
// account session itself is kept.
func RefreshClearance(ctx context.Context) error {
	cookies, err := session.RequireCookies()
	if err != nil {
		return err
	}
	keep := map[string]string{}
	for k, v := range cookies {
		if k != "cf_clearance" && k != "__cf_bm" {
			keep[k] = v
		}
	}
	sol, err := flaresolverr.New().GetWithCookies(ctx, "", client.BaseURL+"/drive/home", keep)
	if err != nil {
		return err
	}
	for _, c := range sol.Cookies {
		if strings.HasSuffix("www.coursesu.com", strings.TrimPrefix(c.Domain, ".")) &&
			(c.Name == "cf_clearance" || c.Name == "__cf_bm") {
			cookies[c.Name] = c.Value
		}
	}
	if err := session.SaveWithUserAgent(cookies, session.AcquiredAt(), sol.UserAgent); err != nil {
		return err
	}
	client.SetUserAgent(sol.UserAgent)
	return nil
}

// WithClearance runs fn, and when Cloudflare turns it away, gets a fresh
// clearance through FlareSolverr and runs it once more. When the coursesu
// session has expired and an SSO cookie is saved, it renews the session
// silently and runs fn again.
func WithClearance[T any](ctx context.Context, fn func(context.Context) (T, error)) (T, error) {
	client.SetUserAgent(session.UserAgent())
	out, err := fn(ctx)
	if IsCloudflareBlock(err) {
		if rerr := RefreshClearance(ctx); rerr != nil {
			return out, errors.Join(err, rerr)
		}
		out, err = fn(ctx)
	}
	if isSessionExpired(err) && session.SSOCookie() != "" {
		if _, rerr := RenewSession(ctx); rerr != nil {
			return out, errors.Join(err, rerr)
		}
		out, err = fn(ctx)
	}
	return out, err
}

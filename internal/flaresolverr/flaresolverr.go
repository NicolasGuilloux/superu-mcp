// Package flaresolverr talks to a FlareSolverr instance, which loads pages
// in a real browser to get past Cloudflare's waiting page.
package flaresolverr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

// DefaultURL is used when COURSESU_FLARESOLVERR_URL is not set.
const DefaultURL = "http://localhost:8191/v1"

type Cookie struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

type Solution struct {
	URL       string   `json:"url"`
	Status    int      `json:"status"`
	UserAgent string   `json:"userAgent"`
	Cookies   []Cookie `json:"cookies"`
}

type Client struct {
	URL  string
	http *http.Client
}

func New() *Client {
	u := os.Getenv("COURSESU_FLARESOLVERR_URL")
	if u == "" {
		u = DefaultURL
	}
	return &Client{URL: u, http: &http.Client{Timeout: 120 * time.Second}}
}

type response struct {
	Status   string   `json:"status"`
	Message  string   `json:"message"`
	Session  string   `json:"session"`
	Solution Solution `json:"solution"`
}

func (c *Client) call(ctx context.Context, payload map[string]any) (*response, error) {
	b, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("FlareSolverr unreachable at %s (start it, or set "+
			"COURSESU_FLARESOLVERR_URL): %w", c.URL, err)
	}
	defer resp.Body.Close()
	var out response
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("FlareSolverr: bad response: %w", err)
	}
	if out.Status != "ok" {
		return nil, fmt.Errorf("FlareSolverr: %s", out.Message)
	}
	return &out, nil
}

// NewSession starts a browser session that keeps its cookies across Get calls.
func (c *Client) NewSession(ctx context.Context) (string, error) {
	r, err := c.call(ctx, map[string]any{"cmd": "sessions.create"})
	if err != nil {
		return "", err
	}
	return r.Session, nil
}

// DestroySession ends a session started with NewSession.
func (c *Client) DestroySession(ctx context.Context, session string) {
	_, _ = c.call(ctx, map[string]any{"cmd": "sessions.destroy", "session": session})
}

// Get loads url in the session's browser and returns where it ended up,
// with the cookies of that page and the browser's User-Agent.
func (c *Client) Get(ctx context.Context, session, url string) (*Solution, error) {
	return c.GetWithCookies(ctx, session, url, nil)
}

// GetWithCookies is Get with cookies set in the browser before loading url.
func (c *Client) GetWithCookies(ctx context.Context, session, url string,
	cookies map[string]string) (*Solution, error) {
	p := map[string]any{"cmd": "request.get", "url": url, "maxTimeout": 60000}
	if session != "" {
		p["session"] = session
	}
	if len(cookies) > 0 {
		list := make([]map[string]string, 0, len(cookies))
		for k, v := range cookies {
			list = append(list, map[string]string{"name": k, "value": v})
		}
		p["cookies"] = list
	}
	r, err := c.call(ctx, p)
	if err != nil {
		return nil, err
	}
	return &r.Solution, nil
}

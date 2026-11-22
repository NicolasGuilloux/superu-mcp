package client

import (
	"context"
	"io"
	"net/url"
	"strings"
	"sync"

	fhttp "github.com/bogdanfinn/fhttp"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
)

const (
	BaseURL          = "https://www.coursesu.com"
	sfccBase         = BaseURL + "/on/demandware.store/Sites-DigitalU-Site/fr_FR"
	defaultUserAgent = "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:148.0) " +
		"Gecko/20100101 Firefox/148.0"
)

// userAgent is the User-Agent sent with every request. Cloudflare binds its
// clearance cookie to the browser that solved the challenge, so it should
// match the browser the session cookies came from.
var userAgent = defaultUserAgent

// SetUserAgent overrides the User-Agent and the matching TLS fingerprint.
// An empty value keeps the current one. A change takes effect on the next
// request, so a long-running process can follow a new login.
func SetUserAgent(ua string) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if ua != "" && ua != userAgent {
		userAgent = ua
		sharedClient = nil
	}
}

func currentUserAgent() string {
	clientMu.Lock()
	defer clientMu.Unlock()
	return userAgent
}

// tlsProfile picks the TLS fingerprint that matches the User-Agent's browser
// family, so the handshake and the header tell the same story.
func tlsProfile(ua string) profiles.ClientProfile {
	switch {
	case strings.Contains(ua, "Firefox/"):
		return profiles.Firefox_147
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "Chromium/"):
		return profiles.Chrome_146
	case strings.Contains(ua, "Safari/"):
		return profiles.Safari_IOS_26_0
	default:
		return profiles.Firefox_147
	}
}

type SessionExpiredError struct{ Location string }

func (e *SessionExpiredError) Error() string {
	return "session expired — re-run: coursesu login"
}

type Response struct {
	StatusCode int
	Body       string
	SetCookies map[string]string
	Location   string
}

var (
	sharedClient tls_client.HttpClient
	clientMu     sync.Mutex
)

func httpClient() (tls_client.HttpClient, error) {
	clientMu.Lock()
	defer clientMu.Unlock()
	if sharedClient != nil {
		return sharedClient, nil
	}
	c, err := tls_client.NewHttpClient(tls_client.NewNoopLogger(),
		tls_client.WithTimeoutSeconds(30),
		tls_client.WithClientProfile(tlsProfile(userAgent)),
		tls_client.WithNotFollowRedirects(),
	)
	if err != nil {
		return nil, err
	}
	sharedClient = c
	return c, nil
}

func raiseIfExpired(resp *fhttp.Response) error {
	if resp.StatusCode != 302 {
		return nil
	}
	loc := resp.Header.Get("Location")
	if loc != "" && strings.Contains(loc, "/connexion") {
		return &SessionExpiredError{Location: loc}
	}
	return nil
}

func cookieHeader(cookies map[string]string) string {
	parts := make([]string, 0, len(cookies))
	for k, v := range cookies {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, "; ")
}

func do(req *fhttp.Request, cookies map[string]string) (*Response, error) {
	hc, err := httpClient()
	if err != nil {
		return nil, err
	}
	if len(cookies) > 0 {
		req.Header.Set("Cookie", cookieHeader(cookies))
	}
	resp, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if len(cookies) > 0 {
		if err := raiseIfExpired(resp); err != nil {
			return nil, err
		}
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	set := map[string]string{}
	for _, c := range resp.Cookies() {
		set[c.Name] = c.Value
	}
	return &Response{StatusCode: resp.StatusCode, Body: string(body), SetCookies: set,
		Location: resp.Header.Get("Location")}, nil
}

func resolve(base, path string) string {
	if strings.HasPrefix(path, "http") {
		return path
	}
	return base + path
}

const acceptHTML = "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8"

func newRequest(ctx context.Context, method, rawURL, accept, referer string, body io.Reader) (*fhttp.Request, error) {
	req, err := fhttp.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header = fhttp.Header{
		"User-Agent":      {currentUserAgent()},
		"Accept-Language": {"en-US,en;q=0.9"},
		"Accept":          {accept},
	}
	if referer != "" {
		req.Header.Set("Referer", referer)
	}
	return req, nil
}

func Get(ctx context.Context, path, referer string, cookies map[string]string) (*Response, error) {
	req, err := newRequest(ctx, "GET", resolve(BaseURL, path), acceptHTML, referer, nil)
	if err != nil {
		return nil, err
	}
	return do(req, cookies)
}

func GetAjax(ctx context.Context, path, referer string, cookies map[string]string) (*Response, error) {
	req, err := newRequest(ctx, "GET", resolve(sfccBase, path), "*/*", referer, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	return do(req, cookies)
}

func PostForm(ctx context.Context, path string, data map[string]string, referer string,
	cookies map[string]string) (*Response, error) {
	form := make([]string, 0, len(data))
	for k, v := range data {
		form = append(form, k+"="+url.QueryEscape(v))
	}
	req, err := newRequest(ctx, "POST", resolve(sfccBase, path), "*/*", referer,
		strings.NewReader(strings.Join(form, "&")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Origin", BaseURL)
	return do(req, cookies)
}

// PostJSON posts a JSON body to an absolute URL or a path on BaseURL.
func PostJSON(ctx context.Context, rawURL, body, referer string,
	cookies map[string]string) (*Response, error) {
	req, err := newRequest(ctx, "POST", resolve(BaseURL, rawURL), "*/*", referer,
		strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", BaseURL)
	return do(req, cookies)
}

// PostURLEncoded posts a form to an absolute URL.
func PostURLEncoded(ctx context.Context, rawURL string, form url.Values) (*Response, error) {
	req, err := newRequest(ctx, "POST", rawURL, "application/json", "",
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req, nil)
}

func MergeCookies(jar, set map[string]string) map[string]string {
	for k, v := range set {
		jar[k] = v
	}
	return jar
}

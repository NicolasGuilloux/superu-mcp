package session

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

var ErrNotLoggedIn = errors.New("not logged in — run: coursesu login")

var (
	ConfigDir   = defaultConfigDir()
	SessionFile = filepath.Join(defaultConfigDir(), "session.json")
)

func defaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "coursesu")
}

type stored struct {
	Cookies    map[string]string `json:"cookies"`
	AcquiredAt int64             `json:"acquired_at"`
	UserAgent  string            `json:"user_agent,omitempty"`
	SSOCookie  string            `json:"sso_cookie,omitempty"`
}

func load() stored {
	var s stored
	b, err := os.ReadFile(SessionFile)
	if err != nil {
		return s
	}
	_ = json.Unmarshal(b, &s)
	return s
}

func Save(cookies map[string]string, acquiredAt int64) error {
	return SaveWithUserAgent(cookies, acquiredAt, load().UserAgent)
}

// SaveWithUserAgent saves the session along with the User-Agent of the
// browser the cookies came from.
func SaveWithUserAgent(cookies map[string]string, acquiredAt int64, userAgent string) error {
	if acquiredAt == 0 {
		acquiredAt = time.Now().Unix()
	}
	if err := os.MkdirAll(ConfigDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(stored{Cookies: cookies, AcquiredAt: acquiredAt,
		UserAgent: userAgent, SSOCookie: load().SSOCookie}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SessionFile, b, 0o600)
}

func LoadCookies() map[string]string {
	c := load().Cookies
	if c == nil {
		return map[string]string{}
	}
	return c
}

func RequireCookies() (map[string]string, error) {
	c := LoadCookies()
	if len(c) == 0 {
		return nil, ErrNotLoggedIn
	}
	return c, nil
}

func AcquiredAt() int64 { return load().AcquiredAt }

// UserAgent returns the User-Agent saved with the session, or "".
func UserAgent() string { return load().UserAgent }

func Persist(jar, set map[string]string) {
	for k, v := range set {
		jar[k] = v
	}
	_ = Save(jar, AcquiredAt())
}

// SSOCookie returns the Magasins U SSO session cookie saved for silent
// renewal, or "".
func SSOCookie() string { return load().SSOCookie }

// SetSSOCookie saves the SSO session cookie next to the session.
func SetSSOCookie(v string) error {
	s := load()
	s.SSOCookie = v
	if err := os.MkdirAll(ConfigDir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(SessionFile, b, 0o600)
}

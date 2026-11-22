package browser

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/pierrec/lz4/v4"
)

const LoginURL = "https://www.coursesu.com/connexion"

var RequiredCookies = []string{"dwsid", "sid"}

const FirefoxUserJS = `user_pref("browser.sessionstore.interval", 2000);
user_pref("browser.sessionstore.resume_from_crash", true);
user_pref("browser.startup.page", 3);
user_pref("browser.shell.checkDefaultBrowser", false);
user_pref("browser.rights.3.shown", true);
user_pref("browser.startup.homepage_override.mstone", "ignore");
user_pref("browser.aboutwelcome.enabled", false);
user_pref("datareporting.policy.firstRunURL", "");
user_pref("datareporting.policy.dataSubmissionPolicyBypassNotification", true);
user_pref("toolkit.telemetry.reportingpolicy.firstRun", false);
`

var (
	cookieHeaderRe = regexp.MustCompile(`(?i)-H\s+\$?['"]?[Cc]ookie:\s*([^'"\n]+)`)
	// Chrome's "Copy as cURL" passes cookies with -b / --cookie instead.
	cookieFlagRe    = regexp.MustCompile(`(?:^|\s)(?:-b|--cookie)\s+\$?['"]([^'"\n]+)`)
	userAgentRe     = regexp.MustCompile(`(?i)-H\s+\$?['"]?User-Agent:\s*([^'"\n]+)`)
	userAgentFlagRe = regexp.MustCompile(`(?:^|\s)(?:-A|--user-agent)\s+\$?['"]([^'"\n]+)`)
)

// ParseUserAgentFromCurl returns the User-Agent of a copied cURL, or "".
func ParseUserAgentFromCurl(curlText string) string {
	for _, re := range []*regexp.Regexp{userAgentRe, userAgentFlagRe} {
		if m := re.FindStringSubmatch(curlText); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func nativeProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "coursesu", "firefox-profile")
}

func snapProfileDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "snap", "firefox", "common",
		".mozilla", "firefox", "coursesu-spike")
}

func FindFirefoxBinary() string {
	for _, name := range []string{"firefox", "firefox-esr", "firefox-developer-edition"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func isSnapFirefox(binary string) bool {
	resolved, err := filepath.EvalSymlinks(binary)
	if err == nil && strings.Contains(resolved, "/snap/") {
		return true
	}
	_, err = os.Stat("/snap/bin/firefox")
	return err == nil
}

func ProfileDir(binary string) string {
	if isSnapFirefox(binary) {
		return snapProfileDir()
	}
	return nativeProfileDir()
}

func EnsureProfile(binary string) (string, error) {
	profile := ProfileDir(binary)
	if err := os.MkdirAll(profile, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(profile, "user.js"),
		[]byte(FirefoxUserJS), 0o644); err != nil {
		return "", err
	}
	return profile, nil
}

func FindSessionstoreFile(profile string) string {
	candidates := []string{
		filepath.Join(profile, "sessionstore-backups", "recovery.jsonlz4"),
		filepath.Join(profile, "sessionstore.jsonlz4"),
		filepath.Join(profile, "sessionstore-backups", "previous.jsonlz4"),
	}
	type cand struct {
		path string
		mod  int64
	}
	var existing []cand
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil {
			existing = append(existing, cand{c, fi.ModTime().UnixNano()})
		}
	}
	if len(existing) == 0 {
		return ""
	}
	sort.Slice(existing, func(i, j int) bool { return existing[i].mod > existing[j].mod })
	return existing[0].path
}

func ReadMozLz4(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 12 || string(data[:8]) != "mozLz40\x00" {
		return nil, &os.PathError{Op: "mozlz4", Path: path,
			Err: os.ErrInvalid}
	}
	size := binary.LittleEndian.Uint32(data[8:12])
	out := make([]byte, size)
	if _, err := lz4.UncompressBlock(data[12:], out); err != nil {
		return nil, err
	}
	return out, nil
}

func ExtractCoursesuCookies(state any) map[string]string {
	out := map[string]string{}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			host, hOK := x["host"].(string)
			name, nOK := x["name"].(string)
			val, vOK := x["value"].(string)
			if hOK && nOK && vOK {
				h := strings.TrimPrefix(host, ".")
				if h == "coursesu.com" || strings.HasSuffix(h, ".coursesu.com") {
					out[name] = val
				}
			}
			for _, v2 := range x {
				walk(v2)
			}
		case []any:
			for _, v2 := range x {
				walk(v2)
			}
		}
	}
	walk(state)
	return out
}

func ParseCookiesFromCurl(curlText string) map[string]string {
	out := map[string]string{}
	m := cookieHeaderRe.FindStringSubmatch(curlText)
	if m == nil {
		m = cookieFlagRe.FindStringSubmatch(curlText)
	}
	if m == nil {
		return out
	}
	cookieStr := strings.TrimRight(strings.TrimSpace(m[1]), `\`)
	for _, part := range strings.Split(cookieStr, "; ") {
		if k, v, ok := strings.Cut(part, "="); ok {
			out[strings.TrimSpace(k)] = strings.TrimSpace(v)
		}
	}
	return out
}

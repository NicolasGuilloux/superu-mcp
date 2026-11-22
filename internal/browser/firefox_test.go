package browser

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func writeMozLz4(t *testing.T, path string, payload []byte) {
	t.Helper()
	comp := make([]byte, lz4.CompressBlockBound(len(payload)))
	var c lz4.Compressor
	n, err := c.CompressBlock(payload, comp)
	if err != nil {
		t.Fatalf("compress: %v", err)
	}
	buf := []byte("mozLz40\x00")
	size := make([]byte, 4)
	binary.LittleEndian.PutUint32(size, uint32(len(payload)))
	buf = append(buf, size...)
	buf = append(buf, comp[:n]...)
	if err := os.WriteFile(path, buf, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestReadMozLz4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recovery.jsonlz4")
	payload := []byte(`{"hello":"world"}`)
	writeMozLz4(t, path, payload)
	got, err := ReadMozLz4(path)
	if err != nil {
		t.Fatalf("ReadMozLz4: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q, want %q", got, payload)
	}
}

func TestParseCookiesFromCurl(t *testing.T) {
	curl := `curl 'https://x' -H 'Cookie: sid=ABC; dwsid=DEF; other=1' --compressed`
	got := ParseCookiesFromCurl(curl)
	if got["sid"] != "ABC" || got["dwsid"] != "DEF" || got["other"] != "1" {
		t.Fatalf("parsed cookies wrong: %v", got)
	}
}

func TestExtractCoursesuCookies(t *testing.T) {
	state := map[string]any{
		"windows": []any{map[string]any{
			"cookies": []any{
				map[string]any{"host": ".coursesu.com", "name": "sid", "value": "S"},
				map[string]any{"host": "other.com", "name": "x", "value": "Y"},
			},
		}},
	}
	got := ExtractCoursesuCookies(state)
	if got["sid"] != "S" || len(got) != 1 {
		t.Fatalf("extracted cookies wrong: %v", got)
	}
}

func TestParseCookiesFromCurlChromeFlag(t *testing.T) {
	curl := "curl 'https://www.coursesu.com/' \\\n  -H 'accept: text/html' \\\n  -b 'dwsid=abc; sid=def; cf_clearance=x=y' \\\n  -H 'user-agent: Mozilla/5.0 (Macintosh) Chrome/146.0 Safari/537.36'"
	got := ParseCookiesFromCurl(curl)
	if got["dwsid"] != "abc" || got["sid"] != "def" || got["cf_clearance"] != "x=y" {
		t.Fatalf("unexpected cookies: %v", got)
	}
	if ua := ParseUserAgentFromCurl(curl); ua != "Mozilla/5.0 (Macintosh) Chrome/146.0 Safari/537.36" {
		t.Fatalf("unexpected user agent: %q", ua)
	}
}

func TestParseUserAgentFromCurlMissing(t *testing.T) {
	if ua := ParseUserAgentFromCurl("curl 'https://www.coursesu.com/'"); ua != "" {
		t.Fatalf("expected empty user agent, got %q", ua)
	}
}

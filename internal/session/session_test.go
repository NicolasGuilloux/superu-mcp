package session

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestSaveAndRequire(t *testing.T) {
	dir := t.TempDir()
	ConfigDir = filepath.Join(dir, "coursesu")
	SessionFile = filepath.Join(ConfigDir, "session.json")

	if _, err := RequireCookies(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("want ErrNotLoggedIn, got %v", err)
	}
	if err := Save(map[string]string{"sid": "X", "dwsid": "Y"}, 0); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := RequireCookies()
	if err != nil {
		t.Fatalf("RequireCookies after Save: %v", err)
	}
	if got["sid"] != "X" || got["dwsid"] != "Y" {
		t.Fatalf("cookies wrong: %v", got)
	}
	if AcquiredAt() == 0 {
		t.Fatal("AcquiredAt should be set after Save")
	}
}

func TestLoadCookiesEmpty(t *testing.T) {
	dir := t.TempDir()
	ConfigDir = filepath.Join(dir, "coursesu")
	SessionFile = filepath.Join(ConfigDir, "session.json")
	if got := LoadCookies(); len(got) != 0 {
		t.Fatalf("want empty, got %v", got)
	}
}

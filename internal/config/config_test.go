package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileGivesDefaults(t *testing.T) {
	s, err := Load(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != Defaults() {
		t.Fatalf("got %+v, want defaults", s)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "settings.json")
	want := Defaults()
	want.Language = "de"
	want.FlashOnRed = false
	want.Sources.ClaudeDesktopToasts = false
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	// No temp files left behind.
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("expected only settings.json, found %d entries", len(entries))
	}
}

func TestMissingKeysKeepDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"language":"es","sources":{"claudeCode":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if s.Language != "es" || s.Sources.ClaudeCode {
		t.Fatalf("file values not applied: %+v", s)
	}
	if !s.FlashOnRed || s.PollIntervalMs != 1000 || !s.Sources.ClaudeDesktopToasts {
		t.Fatalf("defaults lost for missing keys: %+v", s)
	}
}

func TestCorruptFileIsMovedAside(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err == nil {
		t.Fatal("expected an informational error for a corrupt file")
	}
	if s != Defaults() {
		t.Fatalf("got %+v, want defaults", s)
	}
	if _, err := os.Stat(path + ".bad"); err != nil {
		t.Fatalf("corrupt file not moved aside: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("corrupt file still in place")
	}
}

func TestNormalization(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, []byte(`{"language":"xx","pollIntervalMs":5}`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, _ := Load(path)
	if s.Language != "auto" {
		t.Errorf("unsupported language kept: %q", s.Language)
	}
	if s.PollIntervalMs != MinPollIntervalMs {
		t.Errorf("poll interval not clamped: %d", s.PollIntervalMs)
	}
	s.PollIntervalMs = 10_000_000
	if n := s.normalized().PollIntervalMs; n != MaxPollIntervalMs {
		t.Errorf("poll interval not clamped to max: %d", n)
	}
}

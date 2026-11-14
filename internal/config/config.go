// Package config loads and saves the user's settings file.
//
// The file is small JSON in the per-user config directory:
//
//	Windows  %APPDATA%\AgentSemaphore\settings.json
//	Linux    $XDG_CONFIG_HOME/AgentSemaphore/settings.json (~/.config/…)
//	macOS    ~/Library/Application Support/AgentSemaphore/settings.json
//
// Loading never fails hard: a missing file gives the defaults, a corrupt file
// is moved aside to settings.json.bad and the defaults are used.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	AppDirName = "AgentSemaphore"
	FileName   = "settings.json"

	MinPollIntervalMs = 250
	MaxPollIntervalMs = 60_000
)

// SupportedLanguages are the UI languages with a string catalog.
var SupportedLanguages = []string{"en", "es", "de"}

// Settings is everything the user can change. New fields must have a safe
// default in Defaults(), because older files will not contain them.
type Settings struct {
	// Language is "auto" (follow the OS) or one of SupportedLanguages.
	Language string `json:"language"`
	// PollIntervalMs is how often the sources are read.
	PollIntervalMs int `json:"pollIntervalMs"`
	// FlashOnRed flashes the agent's window in the taskbar on a change to red.
	FlashOnRed bool `json:"flashOnRed"`
	// Sources switches individual detection sources on or off.
	Sources Sources `json:"sources"`
}

// Sources holds one switch per detection source.
type Sources struct {
	ClaudeCode          bool `json:"claudeCode"`
	ClaudeDesktopToasts bool `json:"claudeDesktopToasts"`
}

// Defaults returns the settings used when there is no file or a key is missing.
func Defaults() Settings {
	return Settings{
		Language:       "auto",
		PollIntervalMs: 1000,
		FlashOnRed:     true,
		Sources: Sources{
			ClaudeCode:          true,
			ClaudeDesktopToasts: true,
		},
	}
}

// DefaultPath returns the settings file path for the current user.
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("no user config directory: %w", err)
	}
	return filepath.Join(dir, AppDirName, FileName), nil
}

// Load reads the settings at path. It always returns usable settings.
// The error is informational (for the log); it is non-nil when the file
// existed but could not be used, in which case it was renamed to path+".bad".
func Load(path string) (Settings, error) {
	s := Defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("read %s: %w", path, err)
	}
	// Unmarshal over the defaults: keys missing from the file keep their default.
	if err := json.Unmarshal(data, &s); err != nil {
		bad := path + ".bad"
		_ = os.Remove(bad)
		if rerr := os.Rename(path, bad); rerr != nil {
			return Defaults(), fmt.Errorf("corrupt %s (%v); could not move it aside: %w", path, err, rerr)
		}
		return Defaults(), fmt.Errorf("corrupt %s moved to %s: %w", path, bad, err)
	}
	return s.normalized(), nil
}

// Save writes the settings atomically (temp file + rename) and creates the
// directory if needed.
func Save(path string, s Settings) error {
	s = s.normalized()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), FileName+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// normalized clamps values that would make the app misbehave.
func (s Settings) normalized() Settings {
	if !isSupported(s.Language) {
		s.Language = "auto"
	}
	if s.PollIntervalMs < MinPollIntervalMs {
		s.PollIntervalMs = MinPollIntervalMs
	}
	if s.PollIntervalMs > MaxPollIntervalMs {
		s.PollIntervalMs = MaxPollIntervalMs
	}
	return s
}

func isSupported(lang string) bool {
	if lang == "auto" {
		return true
	}
	for _, l := range SupportedLanguages {
		if l == lang {
			return true
		}
	}
	return false
}

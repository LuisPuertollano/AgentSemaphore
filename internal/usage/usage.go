// Package usage keeps the plan usage that Claude Code reports to its status
// line (rate_limits.five_hour / seven_day: used_percentage, resets_at; see
// docs/USAGE.md) and turns it into menu lines. Only these numbers are stored;
// nothing else from the status line input is kept.
package usage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/config"
	"github.com/LuisPuertollano/AgentSemaphore/internal/i18n"
)

// Window is one rate-limit window.
type Window struct {
	UsedPercentage float64 `json:"usedPercentage"`
	ResetsAt       int64   `json:"resetsAt"` // Unix seconds
}

// Claude is what is stored for Claude Code.
type Claude struct {
	FiveHour *Window   `json:"fiveHour,omitempty"`
	SevenDay *Window   `json:"sevenDay,omitempty"`
	SeenAt   time.Time `json:"seenAt"`
}

// Dir is where usage files live (next to the settings).
func Dir() (string, error) {
	p, err := config.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "usage"), nil
}

// statusInput is the part of Claude Code's status line JSON we read.
type statusInput struct {
	RateLimits *struct {
		FiveHour *rawWindow `json:"five_hour"`
		SevenDay *rawWindow `json:"seven_day"`
	} `json:"rate_limits"`
}

type rawWindow struct {
	UsedPercentage *float64 `json:"used_percentage"`
	ResetsAt       *float64 `json:"resets_at"`
}

func (w *rawWindow) window() *Window {
	if w == nil || w.UsedPercentage == nil || w.ResetsAt == nil {
		return nil
	}
	p := *w.UsedPercentage
	if math.IsNaN(p) || p < 0 || p > 1000 {
		return nil
	}
	return &Window{UsedPercentage: p, ResetsAt: int64(*w.ResetsAt)}
}

// Parse reads Claude Code's status line JSON. ok is false when it has no
// usable rate-limit window (not a subscriber, or before the first response).
func Parse(data []byte, now time.Time) (Claude, bool) {
	var in statusInput
	if json.Unmarshal(data, &in) != nil || in.RateLimits == nil {
		return Claude{}, false
	}
	c := Claude{FiveHour: in.RateLimits.FiveHour.window(), SevenDay: in.RateLimits.SevenDay.window(), SeenAt: now.UTC()}
	return c, c.FiveHour != nil || c.SevenDay != nil
}

func claudeFile(dir string) string { return filepath.Join(dir, "claude.json") }

// Save writes c atomically.
func Save(dir string, c Claude) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, _ := json.Marshal(c)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), claudeFile(dir))
}

// Load returns the stored usage; ok is false when there is none.
func Load(dir string) (Claude, bool) {
	b, err := os.ReadFile(claudeFile(dir))
	if errors.Is(err, fs.ErrNotExist) || err != nil {
		return Claude{}, false
	}
	var c Claude
	if json.Unmarshal(b, &c) != nil {
		return Claude{}, false
	}
	return c, c.FiveHour != nil || c.SevenDay != nil
}

// MenuLines is the "Plan usage" group: a title and one line, or nothing when
// no usage was ever recorded. A window whose reset time has passed is not
// shown as a number: no session has reported since, so it is unknown.
func MenuLines(dir, lang string, now time.Time) []string {
	c, ok := Load(dir)
	if !ok {
		return nil
	}
	var parts []string
	for _, w := range []struct {
		win *Window
		key string
	}{{c.FiveHour, "usage.window5h"}, {c.SevenDay, "usage.window7d"}} {
		if w.win == nil {
			continue
		}
		reset := time.Unix(w.win.ResetsAt, 0)
		if !reset.After(now) {
			continue
		}
		parts = append(parts, i18n.T(lang, w.key,
			"pct", fmt.Sprintf("%.0f", w.win.UsedPercentage),
			"in", Until(reset.Sub(now))))
	}
	line := ""
	if len(parts) == 0 {
		line = i18n.T(lang, "usage.stale")
	} else {
		line = parts[0]
		for _, p := range parts[1:] {
			line += " · " + p
		}
	}
	return []string{i18n.T(lang, "usage.title"), "    Claude: " + line}
}

// Until formats a duration for "resets in …": minutes, hours and minutes, or days.
func Until(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", int(d.Minutes()+0.5))
	case d < 48*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) - 60*h
		if m == 0 {
			return fmt.Sprintf("%d h", h)
		}
		return fmt.Sprintf("%d h %d min", h, m)
	}
	return fmt.Sprintf("%d d", int(d.Hours()/24+0.5))
}

// StatusText is what the status line shows when the user had none of their own.
func StatusText(c Claude, now time.Time) string {
	s := ""
	if c.FiveHour != nil && time.Unix(c.FiveHour.ResetsAt, 0).After(now) {
		s = fmt.Sprintf("5h %.0f%%", c.FiveHour.UsedPercentage)
	}
	if c.SevenDay != nil && time.Unix(c.SevenDay.ResetsAt, 0).After(now) {
		if s != "" {
			s += " · "
		}
		s += fmt.Sprintf("7d %.0f%%", c.SevenDay.UsedPercentage)
	}
	return s
}

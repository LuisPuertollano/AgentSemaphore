package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Claude Code hooks for "usage limit reached" (docs/USAGE.md). Two matcher
// groups are added next to the user's own hooks in Claude Code's user
// settings; uninstall removes exactly those two and nothing else.
//
//   - StopFailure, matcher "rate_limit": a turn ended on a rate limit.
//   - Notification, matcher "quota_auto_resume_fired|quota_auto_resume_stale":
//     the wait for the limit ended because the limit reset (Claude Code
//     v2.1.234 or later).
//
// The hook runs "<exe> usage-event claude" in exec form ("args"): no shell
// is involved, so paths with spaces work on every system.

// LimitEventArg is the first argument of our hook command.
const LimitEventArg = "usage-event"

var limitHooks = []struct{ event, matcher string }{
	{"StopFailure", "rate_limit"},
	{"Notification", "quota_auto_resume_fired|quota_auto_resume_stale"},
}

type hooksBackup struct {
	HooksExisted  bool            `json:"hooksExisted"`  // the "hooks" key was there before
	EventsExisted map[string]bool `json:"eventsExisted"` // per event we add to: it was there before
}

func hooksBackupFile(dir string) string { return filepath.Join(dir, "claude-hooks-backup.json") }

// handler is one hook handler as Claude Code reads it.
type handler struct {
	Type    string   `json:"type"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Timeout int      `json:"timeout,omitempty"`
}

type matcherGroup struct {
	Matcher string    `json:"matcher,omitempty"`
	Hooks   []handler `json:"hooks"`
}

// isOurGroup recognises a matcher group we added, whatever the program path.
func isOurGroup(raw json.RawMessage) bool {
	var g matcherGroup
	if json.Unmarshal(raw, &g) != nil || len(g.Hooks) != 1 {
		return false
	}
	h := g.Hooks[0]
	return len(h.Args) >= 1 && h.Args[0] == LimitEventArg &&
		strings.Contains(strings.ToLower(h.Command), "agentsemaphore")
}

func parseObject(raw json.RawMessage) (*ordered, error) {
	o := &ordered{vals: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(raw)) == 0 {
		return o, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New(`"hooks" is not a JSON object, so it is left alone`)
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		o.set(k, v)
	}
	return o, nil
}

func (o *ordered) raw() json.RawMessage {
	b := o.bytes()
	return json.RawMessage(bytes.TrimSpace(b))
}

func marshalNoEscape(v any) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
	return json.RawMessage(bytes.TrimSpace(b.Bytes()))
}

// InstallClaudeLimitHooks adds our two matcher groups to Claude Code's user
// settings. Running it again changes nothing but the program path.
func InstallClaudeLimitHooks(dir, exe, home string) (string, error) {
	path := ClaudeSettings(home)
	o, _, err := readOrdered(path)
	if err != nil {
		return path, err
	}
	hooksRaw, hooksExisted := o.vals["hooks"]
	hooks, err := parseObject(hooksRaw)
	if err != nil {
		return path, err
	}
	bk := hooksBackup{HooksExisted: hooksExisted, EventsExisted: map[string]bool{}}
	if old, err := os.ReadFile(hooksBackupFile(dir)); err == nil {
		_ = json.Unmarshal(old, &bk) // already installed once: keep the first backup
	} else {
		for _, h := range limitHooks {
			_, bk.EventsExisted[h.event] = hooks.vals[h.event]
		}
	}
	for _, h := range limitHooks {
		var groups []json.RawMessage
		if raw, ok := hooks.vals[h.event]; ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return path, fmt.Errorf("hooks.%s is not a list, so it is left alone", h.event)
			}
		}
		kept := groups[:0]
		for _, g := range groups {
			if !isOurGroup(g) {
				kept = append(kept, g)
			}
		}
		ours := marshalNoEscape(matcherGroup{Matcher: h.matcher, Hooks: []handler{{
			Type: "command", Command: exe, Args: []string{LimitEventArg, "claude"}, Timeout: 10,
		}}})
		hooks.set(h.event, marshalNoEscape(append(kept, ours)))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return path, err
	}
	if err := os.WriteFile(hooksBackupFile(dir), marshalNoEscape(bk), 0o644); err != nil {
		return path, err
	}
	o.set("hooks", hooks.raw())
	return path, writeAtomic(path, o.bytes())
}

// UninstallClaudeLimitHooks removes our matcher groups, and the keys we
// created for them, and nothing else.
func UninstallClaudeLimitHooks(dir, home string) (string, error) {
	path := ClaudeSettings(home)
	o, existed, err := readOrdered(path)
	if err != nil || !existed {
		os.Remove(hooksBackupFile(dir))
		return path, err
	}
	bk := hooksBackup{HooksExisted: true, EventsExisted: map[string]bool{}}
	if b, err := os.ReadFile(hooksBackupFile(dir)); err == nil {
		_ = json.Unmarshal(b, &bk)
	} else if errors.Is(err, fs.ErrNotExist) {
		for _, h := range limitHooks { // no backup: keep whatever is left
			bk.EventsExisted[h.event] = true
		}
	}
	hooksRaw, ok := o.vals["hooks"]
	if !ok {
		os.Remove(hooksBackupFile(dir))
		return path, nil
	}
	hooks, err := parseObject(hooksRaw)
	if err != nil {
		return path, err
	}
	changed := false
	for _, h := range limitHooks {
		raw, ok := hooks.vals[h.event]
		if !ok {
			continue
		}
		var groups []json.RawMessage
		if json.Unmarshal(raw, &groups) != nil {
			continue
		}
		kept := groups[:0]
		for _, g := range groups {
			if isOurGroup(g) {
				changed = true
			} else {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 && !bk.EventsExisted[h.event] {
			hooks.del(h.event)
		} else {
			hooks.set(h.event, marshalNoEscape(kept))
		}
	}
	if len(hooks.keys) == 0 && !bk.HooksExisted {
		o.del("hooks")
	} else {
		o.set("hooks", hooks.raw())
	}
	os.Remove(hooksBackupFile(dir))
	if !changed {
		return path, nil
	}
	return path, writeAtomic(path, o.bytes())
}

// ClaudeLimitHooksInstalled reports whether our groups are in the settings.
func ClaudeLimitHooksInstalled(home string) bool {
	o, _, err := readOrdered(ClaudeSettings(home))
	if err != nil {
		return false
	}
	hooks, err := parseObject(o.vals["hooks"])
	if err != nil {
		return false
	}
	n := 0
	for _, h := range limitHooks {
		var groups []json.RawMessage
		_ = json.Unmarshal(hooks.vals[h.event], &groups)
		for _, g := range groups {
			if isOurGroup(g) {
				n++
				break
			}
		}
	}
	return n == len(limitHooks)
}

// Limit is the "usage limit reached" note, kept until the limit is over.
type Limit struct {
	Since   time.Time `json:"since"`
	Message string    `json:"message,omitempty"`
}

func limitFile(dir string) string { return filepath.Join(dir, "claude-limit.json") }

// HandleLimitEvent takes the JSON Claude Code passes to our hook: a
// StopFailure with error "rate_limit" records the limit; the notifications
// that say the limit reset clear it. Anything else is ignored.
func HandleLimitEvent(dir string, data []byte, now time.Time) error {
	var in struct {
		Event   string `json:"hook_event_name"`
		Error   string `json:"error"`
		Type    string `json:"notification_type"`
		Message string `json:"last_assistant_message"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	switch {
	case in.Event == "StopFailure" && in.Error == "rate_limit":
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		msg := in.Message
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return os.WriteFile(limitFile(dir), marshalNoEscape(Limit{Since: now.UTC(), Message: msg}), 0o644)
	case in.Event == "Notification" && (in.Type == "quota_auto_resume_fired" || in.Type == "quota_auto_resume_stale"):
		err := os.Remove(limitFile(dir))
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return nil
}

// activeLimit returns the recorded limit if it still holds: no later status
// line report (Claude Code answered again, so the limit is over) and not
// older than a week (the longest window).
func activeLimit(dir string, c Claude, haveUsage bool, now time.Time) (Limit, bool) {
	b, err := os.ReadFile(limitFile(dir))
	if err != nil {
		return Limit{}, false
	}
	var l Limit
	if json.Unmarshal(b, &l) != nil || l.Since.IsZero() {
		return Limit{}, false
	}
	if now.Sub(l.Since) > 7*24*time.Hour {
		return Limit{}, false
	}
	if haveUsage && c.SeenAt.After(l.Since) {
		return Limit{}, false
	}
	return l, true
}

// limitReset guesses when the limit ends: the reset of a window that was full
// (or nearly) at the last report, the five-hour one first.
func limitReset(c Claude, now time.Time) (time.Time, bool) {
	for _, w := range []*Window{c.FiveHour, c.SevenDay} {
		if w != nil && w.UsedPercentage >= 95 {
			if t := time.Unix(w.ResetsAt, 0); t.After(now) {
				return t, true
			}
		}
	}
	return time.Time{}, false
}

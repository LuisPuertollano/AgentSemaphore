package usage

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLimitHooksSitNextToTheUsersOwnAndLeaveCleanly(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	p := settings(t, home)
	orig := `{"model": "opus", "hooks": {"PreToolUse": [{"matcher": "Bash", "hooks": [{"type": "command", "command": "mine.sh"}]}],` +
		` "StopFailure": [{"matcher": "server_error", "hooks": [{"type": "command", "command": "alert & log"}]}]}, "theme": "dark"}`
	os.WriteFile(p, []byte(orig), 0o644)

	for i := 0; i < 2; i++ { // twice: no duplicates, first backup kept
		if _, err := InstallClaudeLimitHooks(dir, "/opt/My Apps/agentsemaphore", home); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if strings.Count(s, `"usage-event"`) != 2 {
		t.Errorf("one StopFailure and one Notification group of ours: %s", s)
	}
	if !strings.Contains(s, `"mine.sh"`) || !strings.Contains(s, `"alert & log"`) || !strings.Contains(s, `"server_error"`) {
		t.Errorf("the user's own hooks stay, unescaped: %s", s)
	}
	if !(strings.Index(s, `"model"`) < strings.Index(s, `"hooks"`) && strings.Index(s, `"hooks"`) < strings.Index(s, `"theme"`)) {
		t.Errorf("key order kept: %s", s)
	}
	if !strings.Contains(s, `"command": "/opt/My Apps/agentsemaphore"`) || !strings.Contains(s, `"rate_limit"`) ||
		!strings.Contains(s, `"quota_auto_resume_fired|quota_auto_resume_stale"`) {
		t.Errorf("exec form with the path as is, and both matchers: %s", s)
	}
	var parsed struct {
		Hooks map[string][]matcherGroup `json:"hooks"`
	}
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatal(err)
	}
	sf := parsed.Hooks["StopFailure"]
	if len(sf) != 2 || sf[0].Matcher != "server_error" || sf[1].Hooks[0].Args[0] != LimitEventArg {
		t.Errorf("ours is appended after the user's group: %+v", sf)
	}
	if !ClaudeLimitHooksInstalled(home) {
		t.Error("installed")
	}

	if _, err := UninstallClaudeLimitHooks(dir, home); err != nil {
		t.Fatal(err)
	}
	if ClaudeLimitHooksInstalled(home) {
		t.Error("uninstalled")
	}
	var got, want map[string]any
	b, _ = os.ReadFile(p)
	json.Unmarshal(b, &got)
	json.Unmarshal([]byte(orig), &want)
	gb, _ := json.Marshal(got)
	wb, _ := json.Marshal(want)
	if string(gb) != string(wb) {
		t.Errorf("not restored:\n%s\n%s", gb, wb)
	}
}

func TestLimitHooksWithoutAnyHooksBefore(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	p := settings(t, home)
	os.WriteFile(p, []byte(`{"model": "opus"}`), 0o644)
	if _, err := InstallClaudeLimitHooks(dir, "/x/agentsemaphore", home); err != nil {
		t.Fatal(err)
	}
	if _, err := UninstallClaudeLimitHooks(dir, home); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "hooks") || !strings.Contains(string(b), `"opus"`) {
		t.Errorf(`the "hooks" key we created is gone again: %s`, b)
	}
	// Not plain JSON: left alone.
	os.WriteFile(p, []byte(`{"hooks": [1]}`), 0o644)
	if _, err := InstallClaudeLimitHooks(dir, "/x/agentsemaphore", home); err == nil {
		t.Error(`"hooks" that is not an object must be left alone`)
	}
}

func limitEvent(t *testing.T, dir, js string, at time.Time) {
	t.Helper()
	if err := HandleLimitEvent(dir, []byte(js), at); err != nil {
		t.Fatal(err)
	}
}

func TestLimitReachedInTheMenuUntilItIsOver(t *testing.T) {
	dir := t.TempDir()
	if MenuLines(dir, "en", now) != nil {
		t.Fatal("nothing recorded: no group")
	}
	// Another API error is not a usage limit.
	limitEvent(t, dir, `{"hook_event_name":"StopFailure","error":"server_error"}`, now)
	if MenuLines(dir, "en", now) != nil {
		t.Fatal("server_error is not a limit")
	}
	// The limit, with usage data from before it: the full 5-hour window gives the reset.
	c, _ := Parse([]byte(`{"rate_limits":{"five_hour":{"used_percentage":100,"resets_at":`+
		itoa(now.Add(90*time.Minute).Unix())+`},"seven_day":{"used_percentage":60,"resets_at":`+
		itoa(now.Add(72*time.Hour).Unix())+`}}}`), now.Add(-time.Minute))
	Save(dir, c)
	limitEvent(t, dir, `{"hook_event_name":"StopFailure","error":"rate_limit","last_assistant_message":"API Error: Rate limit reached"}`, now)
	lines := MenuLines(dir, "en", now.Add(time.Minute))
	if len(lines) != 3 || !strings.Contains(lines[2], "limit reached at") || !strings.Contains(lines[2], "resets in 1 h 29 min") {
		t.Fatalf("limit line with the 5-hour reset: %q", lines)
	}
	// The wait ended because the limit reset: the note goes.
	limitEvent(t, dir, `{"hook_event_name":"Notification","notification_type":"quota_auto_resume_fired"}`, now.Add(2*time.Hour))
	if lines := MenuLines(dir, "en", now.Add(2*time.Hour)); len(lines) != 2 {
		t.Fatalf("no limit line after the reset: %q", lines)
	}
	// A limit without usage data, then a newer status line report ends it.
	os.Remove(claudeFile(dir))
	limitEvent(t, dir, `{"hook_event_name":"StopFailure","error":"rate_limit"}`, now)
	if lines := MenuLines(dir, "es", now); len(lines) != 2 || !strings.Contains(lines[1], "límite alcanzado") {
		t.Fatalf("limit only, in Spanish: %q", lines)
	}
	c, _ = Parse(sample(now.Add(time.Hour).Unix(), now.Add(48*time.Hour).Unix()), now.Add(10*time.Minute))
	Save(dir, c)
	if lines := MenuLines(dir, "en", now.Add(11*time.Minute)); len(lines) != 2 || strings.Contains(strings.Join(lines, ""), "limit") {
		t.Fatalf("Claude Code answered again, so the limit is over: %q", lines)
	}
	// Garbage in: an error, no panic, nothing recorded.
	if HandleLimitEvent(dir, []byte("not json"), now) == nil {
		t.Error("bad input is reported")
	}
}

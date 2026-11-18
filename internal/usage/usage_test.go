package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

// sample follows the example in Claude Code's status line documentation.
func sample(five, seven int64) []byte {
	return []byte(`{"model":{"display_name":"Opus"},"cost":{"total_cost_usd":0.12},
	  "rate_limits":{"five_hour":{"used_percentage":23.5,"resets_at":` + itoa(five) + `},
	                 "seven_day":{"used_percentage":41.2,"resets_at":` + itoa(seven) + `}}}`)
}

func itoa(v int64) string { b, _ := json.Marshal(v); return string(b) }

func TestParseKeepsOnlyTheNumbers(t *testing.T) {
	c, ok := Parse(sample(now.Add(2*time.Hour).Unix(), now.Add(72*time.Hour).Unix()), now)
	if !ok || c.FiveHour.UsedPercentage != 23.5 || c.SevenDay.UsedPercentage != 41.2 {
		t.Fatalf("%+v %v", c, ok)
	}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "Opus") || strings.Contains(string(b), "cost") {
		t.Fatalf("only rate-limit numbers may be stored: %s", b)
	}
	if _, ok := Parse([]byte(`{"model":{}}`), now); ok {
		t.Error("no rate_limits (not a subscriber, or before the first reply): nothing to store")
	}
	if _, ok := Parse([]byte(`not json`), now); ok {
		t.Error("garbage")
	}
	if c, ok := Parse([]byte(`{"rate_limits":{"seven_day":{"used_percentage":5,"resets_at":1}}}`), now); !ok || c.FiveHour != nil {
		t.Errorf("one window may be missing: %+v", c)
	}
}

func TestMenuLines(t *testing.T) {
	dir := t.TempDir()
	if MenuLines(dir, "en", now) != nil {
		t.Fatal("no data: no menu group")
	}
	c, _ := Parse(sample(now.Add(2*time.Hour+30*time.Minute).Unix(), now.Add(72*time.Hour).Unix()), now)
	if err := Save(dir, c); err != nil {
		t.Fatal(err)
	}
	got := MenuLines(dir, "en", now)
	want := []string{"Plan usage", "    Claude: 5 h 24 % (resets in 2 h 30 min) · week 41 % (resets in 3 d)"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %q", got)
	}
	// After the 5-hour reset with no newer reply, that number is unknown.
	if got := MenuLines(dir, "en", now.Add(3*time.Hour)); !strings.Contains(got[1], "week 41 %") || strings.Contains(got[1], "5 h") {
		t.Errorf("past 5-hour reset: %q", got)
	}
	if got := MenuLines(dir, "en", now.Add(100*time.Hour)); !strings.Contains(got[1], "unknown") {
		t.Errorf("past both resets: %q", got)
	}
	if s := StatusText(c, now); s != "5h 24% · 7d 41%" {
		t.Errorf("status text %q", s)
	}
}

func TestUntil(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Minute: "10 min", 2 * time.Hour: "2 h", 90 * time.Minute: "1 h 30 min", 72 * time.Hour: "3 d",
	} {
		if got := Until(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}

func settings(t *testing.T, home string) string {
	t.Helper()
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	p := ClaudeSettings(home)
	os.MkdirAll(filepath.Dir(p), 0o755)
	return p
}

func TestInstallKeepsTheUsersStatusLineAndRestoresIt(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	p := settings(t, home)
	orig := `{"model": "opus", "statusLine": {"type": "command", "command": "~/bin/my-line.sh", "padding": 1}, "theme": "dark"}`
	os.WriteFile(p, []byte(orig), 0o644)

	for i := 0; i < 2; i++ { // twice: the backup must stay the user's original
		if _, err := InstallClaude(dir, "/opt/agentsemaphore", home); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(p)
	s := string(b)
	if !(strings.Index(s, `"model"`) < strings.Index(s, `"statusLine"`) && strings.Index(s, `"statusLine"`) < strings.Index(s, `"theme"`)) {
		t.Errorf("key order must be kept: %s", s)
	}
	if !strings.Contains(s, `\"/opt/agentsemaphore\" statusline`) || !strings.Contains(s, `"padding": 1`) {
		t.Errorf("our command, the user's padding kept: %s", s)
	}
	if !ClaudeInstalled(home) || PreviousCommand(dir) != "~/bin/my-line.sh" {
		t.Errorf("installed=%v previous=%q", ClaudeInstalled(home), PreviousCommand(dir))
	}
	if _, what, err := UninstallClaude(dir, home); err != nil || what != "restored" {
		t.Fatalf("%q %v", what, err)
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

func TestInstallWithoutStatusLineAndUserChangesSince(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	p := settings(t, home)
	if _, err := InstallClaude(dir, "/x/agentsemaphore", home); err != nil { // no settings.json yet
		t.Fatal(err)
	}
	if PreviousCommand(dir) != "" {
		t.Error("no previous status line")
	}
	if _, what, _ := UninstallClaude(dir, home); what != "restored" {
		t.Fatal(what)
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "statusLine") {
		t.Errorf("statusLine must be gone again: %s", b)
	}
	// The user replaced our status line by hand: uninstall leaves theirs alone.
	InstallClaude(dir, "/x/agentsemaphore", home)
	os.WriteFile(p, []byte(`{"statusLine": {"type": "command", "command": "theirs"}}`), 0o644)
	if _, what, _ := UninstallClaude(dir, home); !strings.Contains(what, "left as it is") {
		t.Fatal(what)
	}
	b, _ = os.ReadFile(p)
	if !strings.Contains(string(b), "theirs") {
		t.Error("the user's own status line must stay")
	}
}

func TestInstallRefusesWhatItCannotParse(t *testing.T) {
	home, dir := t.TempDir(), t.TempDir()
	p := settings(t, home)
	os.WriteFile(p, []byte("{ // comment\n}"), 0o644)
	if _, err := InstallClaude(dir, "/x/agentsemaphore", home); err == nil {
		t.Error("not plain JSON: left alone")
	}
	os.WriteFile(p, []byte(`{"a":1}`), 0o644)
	if _, err := InstallClaude(dir, `/x/a"b/agentsemaphore`, home); err == nil {
		t.Error("a quote in the path cannot go into a command")
	}
}

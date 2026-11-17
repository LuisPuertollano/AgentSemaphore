package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

var t0 = time.Date(2026, 11, 17, 22, 0, 0, 0, time.UTC)

func TestMap(t *testing.T) {
	cases := []struct {
		agent, event string
		p            payload
		want         state.State
		remove       bool
		ignore       bool
	}{
		{agent: "copilot", event: "userPromptSubmitted", want: state.Orange},
		{agent: "copilot", event: "notification", p: payload{NotificationType: "permission_prompt"}, want: state.Red},
		{agent: "copilot", event: "notification", p: payload{NotificationType: "elicitation_dialog"}, want: state.Red},
		{agent: "copilot", event: "notification", p: payload{NotificationType: "shell_completed"}, ignore: true},
		{agent: "copilot", event: "permissionRequest", ignore: true}, // fires even when no prompt is shown
		{agent: "copilot", event: "preToolUse", ignore: true},
		{agent: "copilot", event: "agentStop", want: state.Green},
		{agent: "copilot", event: "sessionEnd", remove: true},
		{agent: "codex", event: "UserPromptSubmit", want: state.Orange},
		{agent: "codex", event: "PermissionRequest", want: state.Red},
		{agent: "codex", event: "PostToolUse", want: state.Orange},
		{agent: "codex", event: "Stop", want: state.Green},
		{agent: "codex", event: "Interrupt", want: state.Green},
		{agent: "codex", event: "SessionEnd", remove: true},
		{agent: "gemini", event: "BeforeAgent", want: state.Orange},
		{agent: "gemini", event: "Notification", p: payload{NotificationType: "ToolPermission"}, want: state.Red},
		{agent: "gemini", event: "BeforeTool", p: payload{ToolName: "ask_user"}, want: state.Red},
		{agent: "gemini", event: "BeforeTool", p: payload{ToolName: "read_file"}, want: state.Orange},
		{agent: "gemini", event: "AfterAgent", want: state.Green},
		{agent: "gemini", event: "SessionEnd", remove: true},
		{agent: "other", event: "Stop", ignore: true},
	}
	for _, c := range cases {
		a := Map(c.agent, c.event, c.p)
		if a.ignore != c.ignore || a.remove != c.remove || (!a.ignore && !a.remove && a.state != c.want) {
			t.Errorf("%s %s %+v: got %+v", c.agent, c.event, c.p, a)
		}
	}
}

func handle(t *testing.T, dir, agent, event, input string) {
	t.Helper()
	if err := Handle(dir, agent, event, strings.NewReader(input), 0, t0); err != nil {
		t.Fatalf("%s %s: %v", agent, event, err)
	}
}

func sessions(t *testing.T, dir string, alive bool) map[string]source.Session {
	t.Helper()
	s := &Source{Dir: dir, Alive: func(int, string) bool { return alive }, Now: func() time.Time { return t0.Add(time.Minute) }}
	r := s.Poll()
	if r.Health != source.OK {
		t.Fatalf("health %v %s", r.Health, r.Detail)
	}
	m := map[string]source.Session{}
	for _, x := range r.Sessions {
		m[x.Name] = x
	}
	return m
}

func TestHandleAndPoll(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "hooks")
	if r := (&Source{Dir: dir}).Poll(); r.Health != source.Absent {
		t.Fatalf("no folder must be absent, got %v", r.Health)
	}
	handle(t, dir, "copilot", "userPromptSubmitted", `{"sessionId":"abc","cwd":"/work/site","timestamp":1}`)
	handle(t, dir, "codex", "PermissionRequest", `{"session_id":"x1","cwd":"/work/api","tool_name":"Bash"}`)
	got := sessions(t, dir, true)
	if got["Copilot · site"].State != state.Orange {
		t.Errorf("copilot: %+v", got)
	}
	if s := got["Codex · api"]; s.State != state.Red || s.Detail != "permission prompt" {
		t.Errorf("codex: %+v", got)
	}

	handle(t, dir, "copilot", "notification", `{"sessionId":"abc","cwd":"/work/site","notification_type":"elicitation_dialog"}`)
	if s := sessions(t, dir, true)["Copilot · site"]; s.State != state.Red || s.Detail != "input needed" {
		t.Errorf("copilot question: %+v", s)
	}
	handle(t, dir, "copilot", "notification", `{"sessionId":"abc","notification_type":"shell_completed"}`) // ignored
	if s := sessions(t, dir, true)["Copilot · site"]; s.State != state.Red {
		t.Errorf("an ignored event must not change the state: %+v", s)
	}
	handle(t, dir, "copilot", "agentStop", `{"sessionId":"abc","cwd":"/work/site"}`)
	if s := sessions(t, dir, true)["Copilot · site"]; s.State != state.Green {
		t.Errorf("copilot stop: %+v", s)
	}
	handle(t, dir, "copilot", "sessionEnd", `{"sessionId":"abc"}`)
	if _, ok := sessions(t, dir, true)["Copilot · site"]; ok {
		t.Error("session end must remove the session")
	}
}

func TestDeadAgentIsDropped(t *testing.T) {
	dir := t.TempDir()
	r := Record{Agent: "codex", Session: "s", Cwd: "/w/x", State: "red", Updated: t0, Pid: 4242, PidName: "codex"}
	b, _ := json.Marshal(r)
	os.WriteFile(filepath.Join(dir, fileName("codex", "s")), b, 0o644)
	if _, ok := sessions(t, dir, true)["Codex · x"]; !ok {
		t.Fatal("live agent must be shown")
	}
	if _, ok := sessions(t, dir, false)["Codex · x"]; ok {
		t.Fatal("exited agent must not be shown")
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatal("the file of an exited agent is deleted")
	}
}

func TestUnknownStateIsGreyAndNoPidGoesStale(t *testing.T) {
	dir := t.TempDir()
	b, _ := json.Marshal(Record{Agent: "gemini", Session: "g", State: "purple", Updated: t0})
	os.WriteFile(filepath.Join(dir, "gemini-1.json"), b, 0o644)
	os.WriteFile(filepath.Join(dir, "broken.json"), []byte("{"), 0o644)
	got := sessions(t, dir, true)
	if s := got["Gemini · g"]; s.State != state.Grey {
		t.Errorf("unknown state must be grey: %+v", got)
	}
	if s := got["broken.json"]; s.State != state.Grey {
		t.Errorf("unreadable file must be grey: %+v", got)
	}
	s := &Source{Dir: dir, Now: func() time.Time { return t0.Add(13 * time.Hour) }}
	for _, x := range s.Poll().Sessions {
		if strings.HasPrefix(x.Name, "Gemini") {
			t.Error("a session without a known agent process is dropped after 12 h")
		}
	}
}

func TestHandleRejectsBadInput(t *testing.T) {
	dir := t.TempDir()
	if Handle(dir, "nobody", "Stop", strings.NewReader("{}"), 0, t0) == nil {
		t.Error("unknown agent")
	}
	if Handle(dir, "codex", "Stop", strings.NewReader("not json"), 0, t0) == nil {
		t.Error("non-JSON input")
	}
	if Handle(dir, "codex", "Stop", strings.NewReader(`{"cwd":"/x"}`), 0, t0) == nil {
		t.Error("missing session id")
	}
	if err := Handle(dir, "codex", "PreModelRequest", strings.NewReader(`{}`), 0, t0); err != nil {
		t.Errorf("an event we do not use is ignored quietly: %v", err)
	}
}

func TestInstallCopilot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("COPILOT_HOME", "")
	path, err := Install("copilot", "/opt/agentsemaphore/agentsemaphore", home)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var doc struct {
		Version int                         `json:"version"`
		Hooks   map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil || doc.Version != 1 {
		t.Fatalf("%v %s", err, b)
	}
	if _, ok := doc.Hooks["preToolUse"]; ok {
		t.Error("preToolUse fails closed and must not be registered")
	}
	n := doc.Hooks["notification"][0]
	if n["bash"] != `"/opt/agentsemaphore/agentsemaphore" hook copilot notification` || !strings.HasPrefix(n["powershell"].(string), "& ") {
		t.Errorf("notification hook: %v", n)
	}
	if !Installed("copilot", home) {
		t.Error("Installed")
	}
	if _, removed, err := Uninstall("copilot", home); err != nil || !removed || Installed("copilot", home) {
		t.Errorf("uninstall: %v %v", removed, err)
	}
}

func TestInstallCodexKeepsTheUsersHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	path := CodexFile(home)
	os.MkdirAll(filepath.Dir(path), 0o755)
	user := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"say done"}]}]},"other":true}`
	os.WriteFile(path, []byte(user), 0o644)

	for i := 0; i < 2; i++ { // twice: no duplicates
		if _, err := Install("codex", "/usr/local/bin/agentsemaphore", home); err != nil {
			t.Fatal(err)
		}
	}
	var doc map[string]any
	b, _ := os.ReadFile(path)
	json.Unmarshal(b, &doc)
	stop := doc["hooks"].(map[string]any)["Stop"].([]any)
	if len(stop) != 2 || doc["other"] != true {
		t.Fatalf("user's Stop hook and other keys must stay, ours added once: %s", b)
	}
	if !Installed("codex", home) {
		t.Error("Installed")
	}
	if _, removed, err := Uninstall("codex", home); err != nil || !removed {
		t.Fatalf("uninstall: %v %v", removed, err)
	}
	b, _ = os.ReadFile(path)
	json.Unmarshal(b, &doc)
	hooks := doc["hooks"].(map[string]any)
	if len(hooks) != 1 || !strings.Contains(string(b), "say done") {
		t.Errorf("only the user's hook must remain: %s", b)
	}
}

func TestInstallRefusesWhatItCannotParse(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	path := CodexFile(home)
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("{ // a comment\n}"), 0o644)
	if _, err := Install("codex", "/x/agentsemaphore", home); err == nil {
		t.Error("a file that is not plain JSON must be left alone")
	}
	if _, err := Install("gemini", "/x/agentsemaphore", home); err == nil {
		t.Error("gemini settings are never edited automatically")
	}
	if _, err := Install("copilot", `/x/a"b/agentsemaphore`, home); err == nil {
		t.Error("a path with a quote cannot go into a command")
	}
	if s, err := Snippet("gemini", "/x/agentsemaphore"); err != nil || !strings.Contains(s, "hook gemini BeforeTool") {
		t.Errorf("snippet: %v %s", err, s)
	}
}

// Package hooks shows the state of agents that report it through their own
// hook systems (GitHub Copilot CLI, OpenAI Codex CLI, Gemini CLI). The agent
// runs "agentsemaphore hook <agent> <event>" with the event's JSON on stdin;
// Handle turns that into one small file per session in Dir, and Source reads
// those files.
//
// Rules:
//   - one file per agent session; a session-end event removes it;
//   - the agent's process id is recorded (the first parent of the hook that is
//     not a shell); when that process is gone, the file is ignored and deleted;
//   - a state this version does not know is grey, never green;
//   - no hooks folder means no hooks are installed: the source is absent.
//
// Event meanings come from each vendor's documentation; see docs/AGENTS.md
// for sources and what is verified.
package hooks

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/config"
	"github.com/LuisPuertollano/AgentSemaphore/internal/proc"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

const ID = "agentHooks"

// Agents that Handle understands, with the name shown in the menu.
var Agents = map[string]string{"copilot": "Copilot", "codex": "Codex", "gemini": "Gemini"}

// Record is one session's last known state, as written to disk.
type Record struct {
	Agent   string    `json:"agent"`
	Session string    `json:"session"`
	Cwd     string    `json:"cwd,omitempty"`
	State   string    `json:"state"` // red, orange, green
	Detail  string    `json:"detail,omitempty"`
	Event   string    `json:"event"`
	Updated time.Time `json:"updated"`
	Pid     int       `json:"pid,omitempty"`     // the agent process
	PidName string    `json:"pidName,omitempty"` // its executable name when recorded
}

// Dir is where the session files live.
func Dir() (string, error) {
	p, err := config.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(p), "hooks"), nil
}

// action is what an event does to a session.
type action struct {
	remove bool
	state  state.State
	detail string
	ignore bool
}

// payload holds the fields we read from any agent's hook input.
type payload struct {
	SessionID        string `json:"session_id"`
	SessionIDCamel   string `json:"sessionId"`
	Cwd              string `json:"cwd"`
	NotificationType string `json:"notification_type"`
	ToolName         string `json:"tool_name"`
	ToolNameCamel    string `json:"toolName"`
}

func (p payload) session() string {
	if p.SessionID != "" {
		return p.SessionID
	}
	return p.SessionIDCamel
}

func (p payload) tool() string {
	if p.ToolName != "" {
		return p.ToolName
	}
	return p.ToolNameCamel
}

// Map tells what an agent's event means. Unknown agents or events are ignored.
func Map(agent, event string, p payload) action {
	e := strings.ToLower(event)
	switch agent {
	case "copilot":
		// docs.github.com/en/copilot/reference/hooks-reference. permissionRequest is not
		// used: it fires before the rules engine, so also when no prompt is shown.
		switch e {
		case "sessionstart", "agentstop":
			return action{state: state.Green}
		case "userpromptsubmitted", "posttooluse", "posttoolusefailure", "subagentstart":
			return action{state: state.Orange}
		case "notification":
			switch p.NotificationType {
			case "permission_prompt":
				return action{state: state.Red, detail: "permission prompt"}
			case "elicitation_dialog":
				return action{state: state.Red, detail: "input needed"}
			}
		case "sessionend":
			return action{remove: true}
		}
	case "codex":
		// learn.chatgpt.com/docs/hooks. PermissionRequest fires when Codex is about to ask;
		// another hook may still approve it, and the next PostToolUse turns it orange again.
		switch e {
		case "sessionstart", "stop", "interrupt":
			return action{state: state.Green}
		case "userpromptsubmit", "posttooluse", "subagentstart":
			return action{state: state.Orange}
		case "permissionrequest":
			return action{state: state.Red, detail: "permission prompt"}
		case "sessionend":
			return action{remove: true}
		}
	case "gemini":
		// github.com/google-gemini/gemini-cli/blob/main/docs/hooks/reference.md
		switch e {
		case "sessionstart", "afteragent":
			return action{state: state.Green}
		case "beforeagent", "aftertool":
			return action{state: state.Orange}
		case "beforetool":
			if p.tool() == "ask_user" { // built-in question tool; not confirmed to reach hooks
				return action{state: state.Red, detail: "input needed"}
			}
			return action{state: state.Orange}
		case "notification":
			if p.NotificationType == "ToolPermission" {
				return action{state: state.Red, detail: "permission prompt"}
			}
		case "sessionend":
			return action{remove: true}
		}
	}
	return action{ignore: true}
}

// shells are skipped when looking for the agent process above the hook.
var shells = map[string]bool{
	"sh": true, "bash": true, "dash": true, "zsh": true, "fish": true, "ksh": true,
	"pwsh": true, "powershell": true, "cmd": true, "conhost": true, "env": true,
}

// agentProcess walks up from the hook's parent past shells (at most 4 levels).
func agentProcess(start int) (int, string) {
	pid := start
	for i := 0; i < 4 && pid > 1; i++ {
		n, ok := proc.Name(pid)
		if !ok {
			return 0, ""
		}
		if !shells[n] {
			return pid, n
		}
		pp, ok := proc.Parent(pid)
		if !ok {
			return 0, ""
		}
		pid = pp
	}
	return 0, ""
}

// fileName is stable per agent and session, and safe on every file system.
func fileName(agent, session string) string {
	h := sha256.Sum256([]byte(session))
	return agent + "-" + hex.EncodeToString(h[:8]) + ".json"
}

// Handle processes one hook call. It never returns an error the agent would
// see as a failure: callers log it at most. parentPid is os.Getppid() in
// production.
func Handle(dir, agent, event string, in io.Reader, parentPid int, now time.Time) error {
	if _, ok := Agents[agent]; !ok {
		return fmt.Errorf("unknown agent %q", agent)
	}
	data, err := io.ReadAll(io.LimitReader(in, 1<<20))
	if err != nil {
		return err
	}
	var p payload
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("hook input is not JSON: %w", err)
		}
	}
	a := Map(agent, event, p)
	if a.ignore {
		return nil
	}
	session := p.session()
	if session == "" {
		return errors.New("hook input has no session id")
	}
	path := filepath.Join(dir, fileName(agent, session))
	if a.remove {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	pid, name := agentProcess(parentPid)
	r := Record{Agent: agent, Session: session, Cwd: p.Cwd, State: a.state.String(), Detail: a.detail,
		Event: event, Updated: now.UTC(), Pid: pid, PidName: name}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	out, _ := json.Marshal(r)
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	tmp.Close()
	return os.Rename(tmp.Name(), path)
}

// staleWithoutPid is how long a session whose agent process could not be
// identified is shown after its last event.
const staleWithoutPid = 12 * time.Hour

// Source reads the session files in Dir.
type Source struct {
	Dir string
	// Alive reports whether the agent process is still the one recorded.
	// Defaults to comparing proc.Name; tests replace it.
	Alive func(pid int, name string) bool
	// Now defaults to time.Now; tests replace it.
	Now func() time.Time
}

// New returns a source for the default hooks folder.
func New() *Source {
	d, _ := Dir()
	return &Source{Dir: d}
}

func (s *Source) ID() string { return ID }

func alive(pid int, name string) bool {
	n, ok := proc.Name(pid)
	return ok && n == name
}

// parseState maps a stored state; anything unknown is grey.
func parseState(v string) state.State {
	switch v {
	case "red":
		return state.Red
	case "orange":
		return state.Orange
	case "green":
		return state.Green
	}
	return state.Grey
}

func (s *Source) Poll() source.Result {
	res := source.Result{Source: ID, Health: source.OK}
	if s.Dir == "" {
		res.Health = source.Absent
		return res
	}
	isAlive := s.Alive
	if isAlive == nil {
		isAlive = alive
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, fs.ErrNotExist) {
		res.Health = source.Absent
		return res
	}
	if err != nil {
		res.Health, res.Detail = source.Error, err.Error()
		return res
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(s.Dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // being replaced right now; next poll
		}
		var r Record
		if err := json.Unmarshal(data, &r); err != nil || r.Agent == "" {
			res.Sessions = append(res.Sessions, source.Session{Name: e.Name(), State: state.Grey, Detail: "unreadable hook file"})
			continue
		}
		if r.Pid > 0 {
			if !isAlive(r.Pid, r.PidName) {
				os.Remove(path) // the agent has exited; this folder is ours
				continue
			}
		} else if now().Sub(r.Updated) > staleWithoutPid {
			continue
		}
		st := parseState(r.State)
		detail := r.Detail
		if st == state.Grey {
			detail = fmt.Sprintf("unknown state %q", r.State)
		}
		res.Sessions = append(res.Sessions, source.Session{Name: displayName(r), State: st, Detail: detail})
	}
	sort.Slice(res.Sessions, func(i, j int) bool { return res.Sessions[i].Name < res.Sessions[j].Name })
	return res
}

func displayName(r Record) string {
	label := Agents[r.Agent]
	if label == "" {
		label = r.Agent
	}
	if r.Cwd != "" {
		return label + " · " + filepath.Base(r.Cwd)
	}
	s := r.Session
	if len(s) > 8 {
		s = s[:8]
	}
	return label + " · " + s
}

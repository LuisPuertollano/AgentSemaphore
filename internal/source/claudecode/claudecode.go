// Package claudecode reads the live session state that Claude Code itself
// writes to ~/.claude/sessions/<pid>.json (undocumented; see docs/FINDINGS.md).
//
// Rules (same as the Windows prototype):
//   - status waiting → red, busy/shell → orange, idle → green, anything else → grey;
//   - a file whose pid is not a running Claude Code process is ignored, and on
//     Linux also one whose procStart shows the pid now belongs to a newer process;
//   - a file without a pid, or unreadable 3 polls in a row, is grey;
//   - a missing sessions folder means Claude Code is absent, not an error.
package claudecode

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/proc"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

const ID = "claudeCode"

// failuresBeforeGrey is how many consecutive unreadable polls of one file are
// tolerated (a file can be caught half-written) before it shows grey.
const failuresBeforeGrey = 3

// StartingGrace is how long a session may have an empty status. Claude Desktop
// creates short-lived session files with no status while a session starts
// (seen on Windows, < 1 s); they must not flash grey.
const StartingGrace = 10 * time.Second

// Source reads one sessions directory.
type Source struct {
	Dir string
	// IsAgent reports whether pid is a running Claude Code process. Defaults
	// to IsClaudeProcess; tests replace it.
	IsAgent func(pid int) bool
	// Now defaults to time.Now; tests replace it.
	Now func() time.Time
	// StartOf and LocalDomain default to proc.StartTicks and proc.Domain;
	// tests replace them.
	StartOf     func(pid int) (uint64, bool)
	LocalDomain *string

	failures  map[string]int
	lastGood  map[string]file
	firstSeen map[string]time.Time
}

type file struct {
	Pid        int    `json:"pid"`
	Name       string `json:"name"`
	Cwd        string `json:"cwd"`
	Status     string `json:"status"`
	WaitingFor string `json:"waitingFor"`
	ProcStart  string `json:"procStart"`
	PidDomain  string `json:"pidDomain"`
}

// New returns a source for the current user's ~/.claude/sessions.
func New() *Source {
	home, _ := os.UserHomeDir()
	return &Source{Dir: filepath.Join(home, ".claude", "sessions")}
}

// IsClaudeProcess accepts the native Claude Code binary ("claude") and the
// runtimes an npm or bun install runs under.
func IsClaudeProcess(pid int) bool {
	n, ok := proc.Name(pid)
	if !ok {
		return false
	}
	return strings.HasPrefix(n, "claude") || n == "node" || n == "bun"
}

func (s *Source) ID() string { return ID }

// StateOf maps a session status to a colour. Unknown values are grey, never green.
func StateOf(status string) state.State {
	switch status {
	case "waiting":
		return state.Red
	case "busy", "shell":
		return state.Orange
	case "idle":
		return state.Green
	}
	return state.Grey
}

func (s *Source) Poll() source.Result {
	res := source.Result{Source: ID, Health: source.OK}
	if s.failures == nil {
		s.failures = map[string]int{}
		s.lastGood = map[string]file{}
		s.firstSeen = map[string]time.Time{}
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	isAgent := s.IsAgent
	if isAgent == nil {
		isAgent = IsClaudeProcess
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
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		key := e.Name()
		seen[key] = true
		var f file
		data, err := os.ReadFile(filepath.Join(s.Dir, key))
		if err == nil {
			err = json.Unmarshal(data, &f)
		}
		if err != nil {
			s.failures[key]++
			f = s.lastGood[key]
		} else {
			s.failures[key] = 0
			s.lastGood[key] = f
		}
		switch {
		case s.failures[key] >= failuresBeforeGrey:
			res.Sessions = append(res.Sessions, source.Session{Name: key, State: state.Grey, Detail: "unreadable session file"})
			continue
		case f == (file{}):
			continue // unreadable and never read fine: wait for the next poll
		case f.Pid <= 0:
			res.Sessions = append(res.Sessions, source.Session{Name: key, State: state.Grey, Detail: "session file without pid (new format?)"})
			continue
		case !isAgent(f.Pid):
			continue // closed session or reused pid
		case s.pidReused(f):
			continue // the pid now belongs to a newer process (Claude Code restarted, or another program)
		}
		if f.Status == "" {
			if _, ok := s.firstSeen[key]; !ok {
				s.firstSeen[key] = now()
			}
			if now().Sub(s.firstSeen[key]) < StartingGrace {
				continue // starting up
			}
		}
		st := StateOf(f.Status)
		detail := f.WaitingFor
		if st == state.Grey {
			detail = fmt.Sprintf("unknown status %q", f.Status)
		}
		res.Sessions = append(res.Sessions, source.Session{Name: displayName(f), State: st, Detail: detail})
	}
	for k := range s.failures {
		if !seen[k] {
			delete(s.failures, k)
			delete(s.lastGood, k)
		}
	}
	for k := range s.firstSeen {
		if !seen[k] {
			delete(s.firstSeen, k)
		}
	}
	sort.Slice(res.Sessions, func(i, j int) bool { return res.Sessions[i].Name < res.Sessions[j].Name })
	return res
}

// pidReused uses procStart, the process start time Claude Code records, when
// its meaning is known: Linux, same machine and PID namespace (pidDomain).
// Anywhere else it never decides; the process-name check still applies.
func (s *Source) pidReused(f file) bool {
	local := proc.Domain()
	if s.LocalDomain != nil {
		local = *s.LocalDomain
	}
	if f.ProcStart == "" || local == "" || !strings.HasPrefix(local, "linux:") || f.PidDomain != local {
		return false
	}
	want, err := strconv.ParseUint(f.ProcStart, 10, 64)
	if err != nil {
		return false
	}
	startOf := proc.StartTicks
	if s.StartOf != nil {
		startOf = s.StartOf
	}
	got, ok := startOf(f.Pid)
	return ok && got != want
}

func displayName(f file) string {
	if f.Name != "" {
		return f.Name
	}
	if f.Cwd != "" {
		return filepath.Base(f.Cwd)
	}
	return fmt.Sprintf("pid %d", f.Pid)
}

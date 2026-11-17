package claudecode

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newSource(dir string) *Source {
	return &Source{Dir: dir, IsAgent: func(pid int) bool { return pid >= 100 && pid < 200 }}
}

func byName(r source.Result) map[string]source.Session {
	m := map[string]source.Session{}
	for _, s := range r.Sessions {
		m[s.Name] = s
	}
	return m
}

func TestStates(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "101.json", `{"pid":101,"name":"a","status":"waiting","waitingFor":"permission prompt"}`)
	write(t, dir, "102.json", `{"pid":102,"cwd":"/work/b","status":"busy"}`)
	write(t, dir, "103.json", `{"pid":103,"name":"c","status":"idle"}`)
	write(t, dir, "104.json", `{"pid":104,"name":"d","status":"dreaming"}`)
	write(t, dir, "999.json", `{"pid":999,"name":"dead","status":"waiting"}`)
	write(t, dir, "101.abc.key", `not json, ignored`)
	r := newSource(dir).Poll()
	if r.Health != source.OK {
		t.Fatalf("health %v", r.Health)
	}
	m := byName(r)
	if len(m) != 4 {
		t.Fatalf("want 4 live sessions, got %v", m)
	}
	if m["a"].State != state.Red || m["a"].Detail != "permission prompt" {
		t.Errorf("a: %+v", m["a"])
	}
	if m["b"].State != state.Orange {
		t.Errorf("b (name from cwd): %+v", m["b"])
	}
	if m["c"].State != state.Green {
		t.Errorf("c: %+v", m["c"])
	}
	if m["d"].State != state.Grey {
		t.Errorf("unknown status must be grey: %+v", m["d"])
	}
}

func TestTornWritesAndBrokenFiles(t *testing.T) {
	dir := t.TempDir()
	s := newSource(dir)
	write(t, dir, "101.json", `{"pid":101,"name":"a","status":"waiting"}`)
	s.Poll()
	write(t, dir, "101.json", `{"pid":101,"na`)
	if got := byName(s.Poll())["a"].State; got != state.Red {
		t.Fatalf("torn write should keep last good reading, got %v", got)
	}
	s.Poll()
	if got := byName(s.Poll())["101.json"].State; got != state.Grey {
		t.Fatalf("3rd failed read should be grey, got %v", got)
	}
	write(t, dir, "105.json", `{"sessionId":"x","status":"idle"}`)
	if got := byName(s.Poll())["105.json"].State; got != state.Grey {
		t.Fatalf("file without pid should be grey, got %v", got)
	}
}

func TestNewBrokenFileNeedsThreeFailures(t *testing.T) {
	dir := t.TempDir()
	s := newSource(dir)
	write(t, dir, "106.json", `{broken`)
	for i := 1; i <= 2; i++ {
		if _, ok := byName(s.Poll())["106.json"]; ok {
			t.Fatalf("grey after only %d failure(s)", i)
		}
	}
	if got := byName(s.Poll())["106.json"].State; got != state.Grey {
		t.Fatalf("want grey after 3 failures, got %v", got)
	}
}

func TestMissingDirIsAbsent(t *testing.T) {
	r := newSource(filepath.Join(t.TempDir(), "nope")).Poll()
	if r.Health != source.Absent || len(r.Sessions) != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestStateOf(t *testing.T) {
	for in, want := range map[string]state.State{"waiting": state.Red, "busy": state.Orange, "shell": state.Orange, "idle": state.Green, "": state.Grey, "new": state.Grey} {
		if got := StateOf(in); got != want {
			t.Errorf("StateOf(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestEmptyStatusWhileStartingDoesNotFlashGrey(t *testing.T) {
	dir := t.TempDir()
	clock := time.Date(2026, 11, 14, 16, 41, 1, 0, time.UTC)
	s := newSource(dir)
	s.Now = func() time.Time { return clock }
	write(t, dir, "120.json", `{"pid":120,"name":"user-6f","status":""}`)
	if _, ok := byName(s.Poll())["user-6f"]; ok {
		t.Fatal("starting session must be ignored, not grey")
	}
	clock = clock.Add(StartingGrace - time.Second)
	if _, ok := byName(s.Poll())["user-6f"]; ok {
		t.Fatal("still within the grace period")
	}
	clock = clock.Add(2 * time.Second)
	if got := byName(s.Poll())["user-6f"].State; got != state.Grey {
		t.Fatalf("empty status after the grace period should be grey, got %v", got)
	}
	write(t, dir, "120.json", `{"pid":120,"name":"user-6f","status":"idle"}`)
	if got := byName(s.Poll())["user-6f"].State; got != state.Green {
		t.Fatalf("got %v", got)
	}
}

func TestProcStartCatchesReusedPid(t *testing.T) {
	dir := t.TempDir()
	const dom = "linux:abc123:pid:[4026531836]"
	write(t, dir, "101.json", `{"pid":101,"name":"same","status":"waiting","procStart":"205","pidDomain":"`+dom+`"}`)
	write(t, dir, "102.json", `{"pid":102,"name":"reused","status":"waiting","procStart":"205","pidDomain":"`+dom+`"}`)
	write(t, dir, "103.json", `{"pid":103,"name":"other-namespace","status":"busy","procStart":"205","pidDomain":"linux:abc123:pid:[999]"}`)
	write(t, dir, "104.json", `{"pid":104,"name":"no-procstart","status":"idle","pidDomain":"`+dom+`"}`)
	write(t, dir, "105.json", `{"pid":105,"name":"odd-procstart","status":"idle","procStart":"2026-11-17T10:00:00Z","pidDomain":"`+dom+`"}`)
	src := newSource(dir)
	local := dom
	src.LocalDomain = &local
	src.StartOf = func(pid int) (uint64, bool) {
		if pid == 102 {
			return 9999, true // a newer process got pid 102
		}
		return 205, true
	}
	got := byName(src.Poll())
	for _, name := range []string{"same", "other-namespace", "no-procstart", "odd-procstart"} {
		if _, ok := got[name]; !ok {
			t.Errorf("%s: must still be shown", name)
		}
	}
	if _, ok := got["reused"]; ok {
		t.Error("reused pid: the old session must be ignored")
	}

	// Outside Linux (or unknown domain) procStart never decides.
	empty := ""
	src.LocalDomain = &empty
	if _, ok := byName(src.Poll())["reused"]; !ok {
		t.Error("without a known local domain the session must be shown")
	}
}

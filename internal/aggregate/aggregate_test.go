package aggregate

import (
	"strings"
	"testing"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

func TestEmptyIsGreen(t *testing.T) {
	v := Build(nil, "en")
	if v.State != state.Green || !strings.Contains(v.Tooltip, "All quiet") {
		t.Fatalf("%+v", v)
	}
}

func TestWorstWinsAndOrder(t *testing.T) {
	v := Build([]source.Result{{Source: "claudeCode", Sessions: []source.Session{
		{Name: "b", State: state.Green}, {Name: "a", State: state.Orange}, {Name: "z", State: state.Red},
	}}}, "en")
	if v.State != state.Red {
		t.Fatalf("state %v", v.State)
	}
	if v.Sessions[0].Name != "z" || v.Sessions[1].Name != "a" || v.Sessions[2].Name != "b" {
		t.Fatalf("order %+v", v.Sessions)
	}
	if !strings.HasPrefix(v.Tooltip, "1 waiting · 1 working · 1 idle\nz") {
		t.Fatalf("tooltip %q", v.Tooltip)
	}
}

func TestBrokenSourceIsGreyAndBeatsOrange(t *testing.T) {
	v := Build([]source.Result{
		{Source: "claudeCode", Sessions: []source.Session{{Name: "a", State: state.Orange}}},
		{Source: "claudeDesktopToasts", Health: source.Error, Detail: "no such table"},
	}, "es")
	if v.State != state.Grey {
		t.Fatalf("state %v", v.State)
	}
	if !strings.Contains(v.Tooltip, "No se puede leer: Avisos de Claude Desktop") {
		t.Fatalf("tooltip %q", v.Tooltip)
	}
}

func TestAbsentSourceIsNotAnError(t *testing.T) {
	v := Build([]source.Result{{Source: "claudeCode", Health: source.Absent}}, "en")
	if v.State != state.Green {
		t.Fatalf("absent source must not turn grey: %v", v.State)
	}
}

func TestGroupsSayWhatIsWatched(t *testing.T) {
	v := Build([]source.Result{
		{Source: "claudeCode", Sessions: []source.Session{{Name: "b", State: state.Green}, {Name: "a", State: state.Red}}},
		{Source: "claudeDesktopToasts", Health: source.Absent},
	}, "en")
	if len(v.Groups) != 2 {
		t.Fatalf("groups %+v", v.Groups)
	}
	if v.Groups[0].Title != "Claude Code sessions" || v.Groups[0].Sessions[0].Name != "a" || v.Groups[0].Note != "" {
		t.Errorf("claude code group %+v", v.Groups[0])
	}
	if v.Groups[1].Title != "Claude Desktop notifications (shows only what needs you)" || v.Groups[1].Note != "not found on this system" {
		t.Errorf("toasts group %+v", v.Groups[1])
	}
	if !strings.HasPrefix(v.NotVisible, "Not visible:") {
		t.Errorf("not-visible line %q", v.NotVisible)
	}
	empty := Build([]source.Result{{Source: "claudeCode"}}, "es")
	if empty.Groups[0].Note != "nada pendiente" {
		t.Errorf("empty source note %q", empty.Groups[0].Note)
	}
	broken := Build([]source.Result{{Source: "claudeDesktopToasts", Health: source.Error, Detail: "no such table"}}, "en")
	if g := broken.Groups[0]; len(g.Sessions) != 1 || g.Sessions[0].State != state.Grey || !strings.Contains(g.Note, "no such table") {
		t.Errorf("broken source group %+v", g)
	}
}

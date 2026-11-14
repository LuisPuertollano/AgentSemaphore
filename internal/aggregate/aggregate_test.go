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

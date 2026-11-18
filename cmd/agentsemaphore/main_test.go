package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LuisPuertollano/AgentSemaphore/internal/aggregate"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

type panicky struct{}

func (panicky) ID() string          { return "claudeCode" }
func (panicky) Poll() source.Result { panic("boom") }

func TestPanickingSourceIsGreyNotACrash(t *testing.T) {
	v := poll([]source.Source{panicky{}}, "en")
	if v.State != state.Grey {
		t.Fatalf("state %v", v.State)
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("a", 200)
	if got := truncate(long, 127); len([]rune(got)) != 127 || !strings.HasSuffix(got, "…") {
		t.Fatalf("len %d", len([]rune(got)))
	}
	if got := truncate("short", 127); got != "short" {
		t.Fatalf("%q", got)
	}
	if got := truncate(strings.Repeat("🔴", 100), 127); !strings.HasSuffix(got, "…") || len([]rune(got)) > 64 {
		t.Fatalf("surrogate pairs not counted: %d runes", len([]rune(got)))
	}
}

func TestASCIIJSONRoundTrips(t *testing.T) {
	in := map[string]string{"tooltip": "0 te esperan · 1 trabajando", "name": "Grüße 🔴 ñ"}
	data, _ := json.Marshal(in)
	out := asciiJSON(data)
	for _, c := range out {
		if c >= 0x80 {
			t.Fatalf("non-ASCII byte in %s", out)
		}
	}
	var back map[string]string
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back["tooltip"] != in["tooltip"] || back["name"] != in["name"] {
		t.Fatalf("round trip changed text: %v", back)
	}
}

func TestMenuRowsShowEachSourceAndWhatIsNotVisible(t *testing.T) {
	v := aggregate.Build([]source.Result{
		{Source: "claudeCode", Sessions: []source.Session{{Name: "api", State: state.Red, Detail: "permission prompt"}}},
		{Source: "claudeDesktopToasts"},
	}, "en")
	got := strings.Join(menuRows(v), "\n")
	for _, want := range []string{
		"■ Claude Code sessions",
		"🔴  api  (permission prompt)",
		"■ Claude Desktop notifications (shows only what needs you)",
		"    nothing pending",
		"ⓘ Not visible:",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("menu misses %q:\n%s", want, got)
		}
	}
}

func TestResendIcon(t *testing.T) {
	for tick := 1; tick <= 10; tick++ {
		if !resendIcon(tick) {
			t.Fatalf("tick %d: start-up ticks must re-send the icon", tick)
		}
	}
	if resendIcon(11) || resendIcon(29) {
		t.Fatal("after start-up, only every 30th tick re-sends")
	}
	if !resendIcon(30) || !resendIcon(60) {
		t.Fatal("every 30th tick re-sends")
	}
}

func TestReportURL(t *testing.T) {
	u := reportURL("v0.1.0-alpha.5")
	for _, want := range []string{"/issues/new?", "template=test-report.yml", "version=v0.1.0-alpha.5", "title=Test+report%3A+"} {
		if !strings.Contains(u, want) {
			t.Errorf("%s: missing %q", u, want)
		}
	}
}

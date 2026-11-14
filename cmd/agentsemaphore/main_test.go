package main

import (
	"encoding/json"
	"strings"
	"testing"

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

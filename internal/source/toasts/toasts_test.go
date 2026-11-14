package toasts

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

func toastXML(header string, silent bool, texts ...string) string {
	s := "<toast>" + header + `<visual><binding template="ToastGeneric">`
	for _, t := range texts {
		s += "<text>" + t + "</text>"
	}
	s += "</binding></visual>"
	if silent {
		s += `<audio silent="true"/>`
	}
	return s + "</toast>"
}

func TestPhrasesMatchThePrototype(t *testing.T) {
	proto, err := os.ReadFile(filepath.Join("..", "..", "..", "prototype", "windows", "avisos-textos.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(proto, phrasesJSON) {
		t.Fatal("internal/source/toasts/phrases.json differs from prototype/windows/avisos-textos.json; copy it over")
	}
}

func TestExamplesClassifyAsDocumented(t *testing.T) {
	var f struct {
		Languages map[string]struct {
			Examples struct {
				Needs  []string `json:"pide"`
				Ignore []string `json:"ignorar"`
			} `json:"ejemplos"`
		} `json:"idiomas"`
	}
	if err := json.Unmarshal(phrasesJSON, &f); err != nil {
		t.Fatal(err)
	}
	for lang, l := range f.Languages {
		for _, txt := range l.Examples.Needs {
			if k := Classify(toastXML("", false, "Chat", txt)).Kind; k != NeedsYou {
				t.Errorf("%s %q: got %s, want needs", lang, txt, k)
			}
		}
		for _, txt := range l.Examples.Ignore {
			if k := Classify(toastXML("", false, "Chat", txt)).Kind; k != Ignored {
				t.Errorf("%s %q: got %s, want ignored", lang, txt, k)
			}
		}
	}
}

func TestOriginSilenceAndName(t *testing.T) {
	i := Classify(toastXML(`<header id="session-session_01ABC" title="Cloud" arguments=""/>`, false, "Cloud", "please confirm"))
	if i.Kind != Unknown || i.Origin != OriginCloud || i.Silent || i.Name != "Cloud" {
		t.Errorf("cloud: %+v", i)
	}
	i = Classify(toastXML(`<header id="session-local_5ad8" title="Local" arguments=""/>`, true, "Local", "done, nothing to do"))
	if i.Origin != OriginLocal || !i.Silent || i.Kind != Progress {
		t.Errorf("silent Code-session summary should be progress: %+v", i)
	}
	if k := Classify(toastXML("", true, "App", "something new")).Kind; k != Unknown {
		t.Errorf("silent toast without a session header must stay unknown (red), got %s", k)
	}
	if k := Classify(toastXML(`<header id="session-local_1"/>`, true, "S", "Claude needs your input to continue")).Kind; k != NeedsYou {
		t.Errorf("a known needs-you phrase wins over silence, got %s", k)
	}
	i = Classify(toastXML("", false, "My chat &amp; co", "Claude necesita tu respuesta para continuar"))
	if i.Kind != NeedsYou || i.Name != "My chat & co" || i.Origin != OriginOther {
		t.Errorf("chat: %+v", i)
	}
}

// --- source logic with a fake platform ---------------------------------------

type fakePlatform struct {
	toasts  []Stored
	err     error
	focus   bool
	changed bool
}

func (f *fakePlatform) Available() bool { return true }
func (f *fakePlatform) Changed() bool   { c := f.changed; f.changed = false; return c }
func (f *fakePlatform) Read(time.Time) ([]Stored, error) {
	return f.toasts, f.err
}
func (f *fakePlatform) ClaudeInForeground() bool { return f.focus }

func TestSourceRedUntilSeen(t *testing.T) {
	clock := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	p := &fakePlatform{changed: true}
	var unknown []string
	s := &Source{P: p, Now: func() time.Time { return clock }, OnUnknown: func(_ Stored, i Info) { unknown = append(unknown, i.Name) }}
	if r := s.Poll(); len(r.Sessions) != 0 || r.Health != source.OK {
		t.Fatalf("start: %+v", r)
	}
	clock = clock.Add(time.Minute)
	p.toasts = []Stored{
		{Toast{1, toastXML("", false, "Chat A", "Claude needs your input to continue")}, clock.Add(-10 * time.Second)},
		{Toast{2, toastXML("", false, "Scheduled task completed", "Daily")}, clock.Add(-5 * time.Second)},
		{Toast{3, toastXML(`<header id="session-session_x"/>`, false, "Cloud", "confirm step 4")}, clock.Add(-2 * time.Second)},
		{Toast{5, toastXML(`<header id="session-session_x"/>`, true, "Cloud", "PR merged")}, clock.Add(-time.Second)}, // progress: not red
		{Toast{4, toastXML("", false, "Old", "Claude needs your input to continue")}, clock.Add(-2 * time.Hour)},      // before start
	}
	p.changed = true
	r := s.Poll()
	if len(r.Sessions) != 2 || r.Sessions[0].State != state.Red {
		t.Fatalf("want 2 red (Chat A, Cloud), got %+v", r.Sessions)
	}
	if len(unknown) != 2 || unknown[0] != "Cloud" || unknown[1] != "Cloud" {
		t.Fatalf("unknown and progress toasts should both be reported: %v", unknown)
	}
	s.Poll() // reported once only
	if len(unknown) != 2 {
		t.Fatalf("unknown reported twice: %v", unknown)
	}

	// Claude in the foreground for less than 2 s: still red.
	p.focus = true
	s.Poll()
	clock = clock.Add(time.Second)
	if r := s.Poll(); len(r.Sessions) != 2 {
		t.Fatalf("seen too early: %+v", r.Sessions)
	}
	clock = clock.Add(2 * time.Second)
	if r := s.Poll(); len(r.Sessions) != 0 {
		t.Fatalf("should be seen after 2 s in front: %+v", r.Sessions)
	}
}

func TestSourceErrorIsGreyThenRecovers(t *testing.T) {
	p := &fakePlatform{changed: true, err: errors.New("no such table: Notification")}
	s := &Source{P: p}
	if r := s.Poll(); r.Health != source.Error {
		t.Fatalf("want error, got %+v", r)
	}
	p.err = nil // fixed, and no WAL change: must still retry after an error
	if r := s.Poll(); r.Health != source.OK {
		t.Fatalf("want recovery, got %+v", r)
	}
}

// --- database reading ----------------------------------------------------------

func TestReadDB(t *testing.T) {
	src, tmp := t.TempDir(), t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(src, "wpndatabase.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 14, 18, 0, 0, 0, time.UTC)
	for _, q := range []string{
		`create table NotificationHandler (RecordId integer primary key, PrimaryId text)`,
		`create table Notification (Id integer primary key, HandlerId integer, Type text, ArrivalTime integer, Payload blob)`,
		`insert into NotificationHandler values (1, 'Claude_pzs8sxrjxfjjc!Claude'), (2, 'Other!App')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	ins := `insert into Notification values (?, ?, ?, ?, ?)`
	mustExec := func(args ...any) {
		if _, err := db.Exec(ins, args...); err != nil {
			t.Fatal(err)
		}
	}
	mustExec(1, 1, "toast", ToFiletime(now.Add(-time.Minute)), []byte(toastXML("", false, "A", "x")))
	mustExec(2, 2, "toast", ToFiletime(now.Add(-time.Minute)), []byte("other app"))
	mustExec(3, 1, "tile", ToFiletime(now.Add(-time.Minute)), []byte("tile"))
	mustExec(4, 1, "toast", ToFiletime(now.Add(-13*time.Hour)), []byte("old"))
	db.Close()

	got, err := ReadDB(src, tmp, now.Add(-MaxAge))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 1 || !got[0].Arrival.Equal(now.Add(-time.Minute)) {
		t.Fatalf("got %+v", got)
	}

	// A schema change is an error, never an empty list.
	db, _ = sql.Open("sqlite", filepath.Join(src, "wpndatabase.db"))
	if _, err := db.Exec(`alter table Notification rename to Notification2`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	if _, err := ReadDB(src, tmp, now.Add(-MaxAge)); err == nil {
		t.Fatal("want an error after a schema change")
	}
}

package toasts

import (
	"fmt"
	"sync"
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

const ID = "claudeDesktopToasts"

// SeenAfter: once Claude Desktop has been in the foreground this long, the
// toasts that arrived before are considered seen.
const SeenAfter = 2 * time.Second

// Platform hides what only exists on Windows, so the logic can be tested anywhere.
type Platform interface {
	// Available reports whether the notification database exists here.
	Available() bool
	// Changed reports whether the database changed since the last call (WAL mtime).
	Changed() bool
	// Read returns the Claude toasts that arrived after since.
	Read(since time.Time) ([]Stored, error)
	// ClaudeInForeground reports whether a Claude Desktop window has the focus.
	ClaudeInForeground() bool
}

// Source turns Claude Desktop toasts into red sessions.
type Source struct {
	P   Platform
	Now func() time.Time
	// OnUnknown is called once per unrecognised toast, so the caller can log it.
	OnUnknown func(Stored, Info)

	mu         sync.Mutex
	started    bool
	lastSeen   time.Time // toasts that arrived before this are seen
	focusSince time.Time // zero when Claude is not in the foreground
	cached     []Stored
	lastErr    error
	reported   map[int64]bool
}

func (s *Source) ID() string { return ID }

func (s *Source) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Source) Poll() source.Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := source.Result{Source: ID, Health: source.OK}
	now := s.now()
	if !s.started {
		s.started = true
		s.lastSeen = now // toasts from before the app started do not count
		s.reported = map[int64]bool{}
	}
	if s.P == nil || !s.P.Available() {
		res.Health = source.Absent
		return res
	}

	// "Seen" rule: Claude in the foreground for SeenAfter.
	if s.P.ClaudeInForeground() {
		if s.focusSince.IsZero() {
			s.focusSince = now
		}
		if now.Sub(s.focusSince) >= SeenAfter {
			s.lastSeen = now
		}
	} else {
		s.focusSince = time.Time{}
	}

	if s.lastErr != nil || s.P.Changed() {
		toasts, err := s.P.Read(now.Add(-MaxAge))
		if err != nil {
			s.lastErr = err
			res.Health, res.Detail = source.Error, err.Error()
			return res
		}
		s.lastErr = nil
		s.cached = toasts
	}

	for _, t := range s.cached {
		info := Classify(t.Payload)
		if info.Kind == Unknown && !s.reported[t.ID] {
			s.reported[t.ID] = true
			if s.OnUnknown != nil {
				s.OnUnknown(t, info)
			}
		}
		if info.Kind == Ignored || !t.Arrival.After(s.lastSeen) {
			continue
		}
		detail := "chat/Cowork"
		switch {
		case info.Origin == OriginCloud:
			detail = "Claude Code (cloud)"
		case info.Origin == OriginLocal:
			detail = "Claude Code (Desktop)"
		case info.Kind == Unknown:
			detail = "unrecognised notification"
		}
		res.Sessions = append(res.Sessions, source.Session{Name: info.Name, State: state.Red, Detail: detail})
	}
	return res
}

// String is for logs.
func (i Info) String() string {
	return fmt.Sprintf("origin=%q silent=%v texts=%q", i.Origin, i.Silent, i.Texts)
}

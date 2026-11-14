// Package source defines what a detection source reports. Each source (Claude
// Code session files, Claude Desktop notifications, other agents...) lives in
// its own sub-package and knows nothing about the tray.
package source

import "github.com/LuisPuertollano/AgentSemaphore/internal/state"

// Session is one agent session (or one pending notification) a source found.
type Session struct {
	Name   string      // what the user recognises: session name, folder, chat title
	State  state.State // Red, Orange, Green or Grey
	Detail string      // e.g. "permission prompt", or why the state is unknown
}

// Health says whether the source could be read at all.
type Health int

const (
	// OK: read fine (it may still have found no sessions).
	OK Health = iota
	// Absent: the agent is not installed or has never run here. Not an error.
	Absent
	// Error: the source exists but could not be read, or its format changed.
	// The tray shows grey: a red could be hidden.
	Error
)

// Result is what one poll of a source returns.
type Result struct {
	Source   string // stable id, also the i18n key suffix: "source.<id>"
	Health   Health
	Detail   string // error detail when Health == Error
	Sessions []Session
}

// Source is implemented by every detection source. Poll must not block for
// long and must never panic on unexpected data.
type Source interface {
	ID() string
	Poll() Result
}

// Package state defines the indicator colours and how they combine.
package state

// State is the colour shown in the tray. The numeric order is the priority:
// when several sessions or sources disagree, the highest value wins.
//
//	Red    an agent is waiting for the user
//	Grey   a source could not be read, so a red may be hidden ("unknown")
//	Orange an agent is working and does not need the user
//	Green  nothing pending
//
// Grey outranks orange on purpose: a blind source must never be masked by
// "working", and must never be shown as green.
type State int

const (
	Green State = iota
	Orange
	Grey
	Red
)

// String returns the stable identifier used in logs, settings and i18n keys.
func (s State) String() string {
	switch s {
	case Green:
		return "green"
	case Orange:
		return "orange"
	case Grey:
		return "grey"
	case Red:
		return "red"
	}
	return "grey"
}

// Worst returns the highest-priority state. With no input it returns Green
// (nothing is running, nothing is pending).
func Worst(states ...State) State {
	w := Green
	for _, s := range states {
		if s < Green || s > Red {
			s = Grey // out-of-range values are treated as unknown, never as green
		}
		if s > w {
			w = s
		}
	}
	return w
}

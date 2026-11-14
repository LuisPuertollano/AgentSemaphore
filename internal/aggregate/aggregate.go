// Package aggregate combines the results of all sources into what the tray
// shows: one colour, a tooltip and the list of sessions. Worst state wins:
// red > grey > orange > green.
package aggregate

import (
	"sort"
	"strconv"
	"strings"

	"github.com/LuisPuertollano/AgentSemaphore/internal/i18n"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

// View is everything the tray needs for one refresh.
type View struct {
	State    state.State
	Tooltip  string
	Sessions []source.Session // sorted: worst state first, then by name
}

// Build turns source results into a View in the given language.
func Build(results []source.Result, lang string) View {
	var sessions []source.Session
	var blind []string
	for _, r := range results {
		sessions = append(sessions, r.Sessions...)
		if r.Health == source.Error {
			name := i18n.T(lang, "source."+r.Source)
			blind = append(blind, name)
			sessions = append(sessions, source.Session{Name: name, State: state.Grey, Detail: r.Detail})
		}
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].State != sessions[j].State {
			return sessions[i].State > sessions[j].State
		}
		return sessions[i].Name < sessions[j].Name
	})
	states := make([]state.State, len(sessions))
	count := map[state.State]int{}
	for i, s := range sessions {
		states[i] = s.State
		count[s.State]++
	}
	v := View{State: state.Worst(states...), Sessions: sessions}

	var b strings.Builder
	if len(sessions) == 0 {
		b.WriteString(i18n.T(lang, "tooltip.allQuiet"))
	} else {
		b.WriteString(i18n.T(lang, "tooltip.summary",
			"waiting", strconv.Itoa(count[state.Red]),
			"working", strconv.Itoa(count[state.Orange]),
			"idle", strconv.Itoa(count[state.Green])))
		var waiting []string
		for _, s := range sessions {
			if s.State == state.Red {
				waiting = append(waiting, s.Name)
			}
		}
		if len(waiting) > 0 {
			b.WriteString("\n" + strings.Join(waiting, ", "))
		}
	}
	if len(blind) > 0 {
		b.WriteString("\n" + i18n.T(lang, "tooltip.unknown", "sources", strings.Join(blind, ", ")))
	}
	v.Tooltip = b.String()
	return v
}

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
	// Groups is the menu: one group per source, so it is clear what is watched.
	Groups []Group
	// NotVisible says what no source can see (shown at the end of the menu).
	NotVisible string
}

// Group is one source in the menu.
type Group struct {
	Title    string           // translated source name
	Note     string           // e.g. "nothing pending", "not found on this system"
	Sessions []source.Session // worst state first
}

// hiddenWhenAbsent: sources that only matter once the user has set them up
// (no menu group while there is nothing to read).
var hiddenWhenAbsent = map[string]bool{"agentHooks": true}

// notes per source shown next to its title (only red is detected from notifications).
var titleNotes = map[string]string{"claudeDesktopToasts": "menu.toastsRedOnly"}

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
	v := View{State: state.Worst(states...), Sessions: sessions, NotVisible: i18n.T(lang, "menu.notVisible")}
	for _, r := range results {
		if r.Health == source.Absent && hiddenWhenAbsent[r.Source] {
			continue
		}
		g := Group{Title: i18n.T(lang, "source."+r.Source)}
		if k, ok := titleNotes[r.Source]; ok {
			g.Title += " (" + i18n.T(lang, k) + ")"
		}
		switch r.Health {
		case source.Absent:
			g.Note = i18n.T(lang, "menu.sourceAbsent")
		case source.Error:
			g.Note = i18n.T(lang, "source.error", "source", i18n.T(lang, "source."+r.Source), "detail", r.Detail)
			g.Sessions = []source.Session{{Name: i18n.T(lang, "source."+r.Source), State: state.Grey, Detail: r.Detail}}
		default:
			g.Sessions = append([]source.Session(nil), r.Sessions...)
			sort.SliceStable(g.Sessions, func(i, j int) bool {
				if g.Sessions[i].State != g.Sessions[j].State {
					return g.Sessions[i].State > g.Sessions[j].State
				}
				return g.Sessions[i].Name < g.Sessions[j].Name
			})
			if len(g.Sessions) == 0 {
				g.Note = i18n.T(lang, "menu.sourceNothing")
			}
		}
		v.Groups = append(v.Groups, g)
	}

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

//go:build linux

// Command snwatcher is a minimal StatusNotifierWatcher for end-to-end tests on
// Linux: it lets the tray register on a private session bus (no desktop
// needed) and reads back what a real panel would show — the dot's colour, the
// tooltip and the menu labels. It writes the latest snapshot as JSON to -out
// every half second until it is stopped.
//
//	dbus-run-session -- sh -c 'snwatcher -out snap.json & agentsemaphore'
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"github.com/LuisPuertollano/AgentSemaphore/internal/icon"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

const (
	watcherName  = "org.kde.StatusNotifierWatcher"
	watcherPath  = "/StatusNotifierWatcher"
	itemIface    = "org.kde.StatusNotifierItem"
	dbusmenuCall = "com.canonical.dbusmenu.GetLayout"
)

// Snapshot is what the test script reads.
type Snapshot struct {
	Registered bool     `json:"registered"`
	Item       string   `json:"item,omitempty"`
	State      string   `json:"state,omitempty"` // red/orange/grey/green, or "unknown-colour"
	Tooltip    string   `json:"tooltip,omitempty"`
	Menu       []string `json:"menu,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type watcher struct {
	mu    sync.Mutex
	items []string // "busname/path"
	props *prop.Properties
}

// RegisterStatusNotifierItem follows the KDE spec: the argument is either a
// bus name or an object path on the caller's connection (what fyne systray sends).
func (w *watcher) RegisterStatusNotifierItem(sender dbus.Sender, service string) *dbus.Error {
	item := service
	if strings.HasPrefix(service, "/") {
		item = string(sender) + service
	} else {
		item = service + "/StatusNotifierItem"
	}
	w.mu.Lock()
	w.items = append(w.items, item)
	list := append([]string(nil), w.items...)
	w.mu.Unlock()
	w.props.SetMust(watcherName, "RegisteredStatusNotifierItems", list)
	log.Printf("registered %s", item)
	return nil
}

func (w *watcher) RegisterStatusNotifierHost(string) *dbus.Error { return nil }

func (w *watcher) latest() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.items) == 0 {
		return ""
	}
	return w.items[len(w.items)-1]
}

func main() {
	out := flag.String("out", "snwatcher.json", "file that receives the latest snapshot")
	flag.Parse()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		log.Fatalf("session bus: %v", err)
	}
	w := &watcher{}
	if err := conn.Export(w, watcherPath, watcherName); err != nil {
		log.Fatal(err)
	}
	w.props, err = prop.Export(conn, watcherPath, prop.Map{watcherName: {
		"RegisteredStatusNotifierItems":  {Value: []string{}, Emit: prop.EmitTrue},
		"IsStatusNotifierHostRegistered": {Value: true, Emit: prop.EmitTrue},
		"ProtocolVersion":                {Value: int32(0), Emit: prop.EmitTrue},
	}})
	if err != nil {
		log.Fatal(err)
	}
	reply, err := conn.RequestName(watcherName, dbus.NameFlagDoNotQueue)
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		log.Fatalf("could not own %s (reply %v): %v", watcherName, reply, err)
	}
	log.Printf("watching")

	for {
		writeJSON(*out, snapshot(conn, w.latest()))
		time.Sleep(500 * time.Millisecond)
	}
}

func snapshot(conn *dbus.Conn, item string) Snapshot {
	if item == "" {
		return Snapshot{}
	}
	s := Snapshot{Registered: true, Item: item}
	i := strings.Index(item, "/")
	obj := conn.Object(item[:i], dbus.ObjectPath(item[i:]))

	var errs []string
	if v, err := obj.GetProperty(itemIface + ".IconPixmap"); err != nil {
		errs = append(errs, "icon: "+err.Error())
	} else {
		var px []struct {
			W, H int32
			Pix  []byte
		}
		if err := v.Store(&px); err != nil || len(px) == 0 {
			errs = append(errs, fmt.Sprintf("icon: %v (%d pixmaps)", err, len(px)))
		} else {
			s.State = stateOf(px[0].W, px[0].H, px[0].Pix)
		}
	}
	if v, err := obj.GetProperty(itemIface + ".ToolTip"); err != nil {
		errs = append(errs, "tooltip: "+err.Error())
	} else {
		var tt struct {
			Name  string
			Icons []struct {
				W, H int32
				Pix  []byte
			}
			Title, Description string
		}
		if err := v.Store(&tt); err != nil {
			errs = append(errs, "tooltip: "+err.Error())
		} else {
			s.Tooltip = tt.Title
		}
	}
	if v, err := obj.GetProperty(itemIface + ".Menu"); err != nil {
		errs = append(errs, "menu path: "+err.Error())
	} else if p, ok := v.Value().(dbus.ObjectPath); ok {
		call := conn.Object(item[:i], p).Call(dbusmenuCall, 0, int32(0), int32(-1), []string{})
		if call.Err != nil {
			errs = append(errs, "menu: "+call.Err.Error())
		} else if len(call.Body) >= 2 {
			s.Menu = labels(call.Body[1])
		}
	}
	s.Error = strings.Join(errs, "; ")
	return s
}

// stateOf reads the centre pixel (ARGB32, network byte order per the spec).
func stateOf(w, h int32, pix []byte) string {
	o := int((h/2)*w+w/2) * 4
	if o+4 > len(pix) {
		return "unknown-colour"
	}
	a, r, g, b := pix[o], pix[o+1], pix[o+2], pix[o+3]
	for _, st := range []state.State{state.Red, state.Orange, state.Grey, state.Green} {
		c := icon.Colour(st)
		if a == 255 && r == c.R && g == c.G && b == c.B {
			return st.String()
		}
	}
	return fmt.Sprintf("unknown-colour #%02x%02x%02x/%d", r, g, b, a)
}

// labels walks a dbusmenu layout (id, props, children) and returns the
// visible, non-empty labels in order.
func labels(node any) []string {
	var out []string
	var walk func(any)
	walk = func(n any) {
		if v, ok := n.(dbus.Variant); ok {
			n = v.Value()
		}
		parts, ok := n.([]any)
		if !ok || len(parts) < 3 {
			return
		}
		props, _ := parts[1].(map[string]dbus.Variant)
		visible := true
		if v, ok := props["visible"]; ok {
			if b, ok := v.Value().(bool); ok {
				visible = b
			}
		}
		if l, ok := props["label"]; ok && visible {
			if s, ok := l.Value().(string); ok && s != "" {
				out = append(out, s)
			}
		}
		children, _ := parts[2].([]dbus.Variant)
		for _, c := range children {
			walk(c)
		}
	}
	walk(node)
	return out
}

func writeJSON(path string, s Snapshot) {
	data, _ := json.MarshalIndent(s, "", "  ")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err == nil {
		_ = os.Rename(tmp, path)
	}
}

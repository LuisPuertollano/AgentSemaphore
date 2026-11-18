package main

import (
	"fmt"
	"log"
	"runtime"
	"sync/atomic"
	"time"

	"fyne.io/systray"

	"github.com/LuisPuertollano/AgentSemaphore/internal/aggregate"
	"github.com/LuisPuertollano/AgentSemaphore/internal/autostart"
	"github.com/LuisPuertollano/AgentSemaphore/internal/config"
	"github.com/LuisPuertollano/AgentSemaphore/internal/desktop"
	"github.com/LuisPuertollano/AgentSemaphore/internal/i18n"
	"github.com/LuisPuertollano/AgentSemaphore/internal/icon"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/state"
)

// maxMenuRows is how many lines the status part of the menu can show
// (group titles, sessions, notes); the rest are summarised as "+N".
const maxMenuRows = 24

// menuRows turns the view into the status lines of the menu: one block per
// source (title, then its sessions or a note), and finally what no source sees.
func menuRows(v aggregate.View) []string {
	var rows []string
	for _, g := range v.Groups {
		rows = append(rows, "■ "+g.Title)
		for _, s := range g.Sessions {
			label := fmt.Sprintf("    %s  %s", dots[s.State], s.Name)
			if s.Detail != "" && (s.State == state.Red || s.State == state.Grey) {
				label += "  (" + s.Detail + ")"
			}
			rows = append(rows, label)
		}
		if g.Note != "" && len(g.Sessions) == 0 {
			rows = append(rows, "    "+g.Note)
		}
	}
	if len(v.Usage) > 0 {
		rows = append(rows, "■ "+v.Usage[0])
		rows = append(rows, v.Usage[1:]...)
	}
	if v.NotVisible != "" {
		rows = append(rows, "ⓘ "+v.NotVisible)
	}
	return rows
}

type trayConfig struct {
	sources      []source.Source
	lang         string
	interval     time.Duration
	log          *log.Logger
	settings     config.Settings
	settingsPath string
}

var dots = map[state.State]string{state.Red: "🔴", state.Orange: "🟠", state.Green: "🟢", state.Grey: "⚪"}

func runTray(cfg trayConfig) {
	systray.Run(func() { onReady(cfg) }, func() { cfg.log.Println("exit") })
}

// resendIcon reports whether refresh number tick re-sends an unchanged icon:
// every tick for the first 10, then every 30th.
func resendIcon(tick int) bool { return tick <= 10 || tick%30 == 0 }

func iconBytes(s state.State) []byte {
	p := icon.PNG(s)
	if runtime.GOOS == "windows" {
		return icon.ICO(p)
	}
	return p
}

func onReady(cfg trayConfig) {
	T := func(key string, args ...string) string { return i18n.T(cfg.lang, key, args...) }
	systray.SetIcon(iconBytes(state.Grey))
	systray.SetTooltip("AgentSemaphore: " + T("tooltip.starting"))

	items := make([]*systray.MenuItem, maxMenuRows)
	for i := range items {
		items[i] = systray.AddMenuItem("", "")
		items[i].Disable()
		items[i].Hide()
	}
	more := systray.AddMenuItem("", "")
	more.Disable()
	more.Hide()
	systray.AddSeparator()

	openClaude := func() {
		if err := desktop.OpenClaude(); err != nil {
			cfg.log.Printf("open Claude: %v", err)
		}
	}
	if desktop.CanOpenClaude() {
		open := systray.AddMenuItem(T("menu.openClaude"), "")
		go func() {
			for range open.ClickedCh {
				openClaude()
			}
		}()
		if leftClickOpensClaude(runtime.GOOS) {
			systray.SetOnTapped(openClaude) // left click opens Claude, right click shows the menu
		}
	}
	var flashOn atomic.Bool // read by the refresh loop, written by the menu
	flashOn.Store(cfg.settings.FlashOnRed)
	if desktop.CanFlash() {
		flash := systray.AddMenuItemCheckbox(T("menu.flashOnRed"), "", flashOn.Load())
		go func() {
			for range flash.ClickedCh {
				on := !flashOn.Load()
				flashOn.Store(on)
				if on {
					flash.Check()
				} else {
					flash.Uncheck()
				}
				cfg.settings.FlashOnRed = on
				if err := config.Save(cfg.settingsPath, cfg.settings); err != nil {
					cfg.log.Printf("save settings: %v", err)
				}
			}
		}()
	}
	auto := systray.AddMenuItemCheckbox(T("menu.startWithSystem"), "", autostart.Enabled())
	go func() {
		for range auto.ClickedCh {
			var err error
			if autostart.Enabled() {
				err = autostart.Disable()
			} else {
				err = autostart.Enable("")
			}
			if err != nil {
				cfg.log.Printf("autostart: %v", err)
			}
			if autostart.Enabled() {
				auto.Check()
			} else {
				auto.Uncheck()
			}
		}
	}()
	systray.AddSeparator()
	report := systray.AddMenuItem(T("menu.testReport"), "")
	go func() {
		for range report.ClickedCh {
			if err := desktop.OpenURL(reportURL(version)); err != nil {
				cfg.log.Printf("open test report form: %v", err)
			}
		}
	}()
	ver := systray.AddMenuItem(T("menu.version", "v", version), "")
	ver.Disable()
	systray.AddSeparator()
	quit := systray.AddMenuItem(T("menu.quit"), "")
	go func() {
		<-quit.ClickedCh
		systray.Quit()
	}()

	cfg.log.Printf("start, %d source(s), language %s, every %v", len(cfg.sources), cfg.lang, cfg.interval)
	prev := state.State(-1)
	tick := 0
	refresh := func() {
		v := poll(cfg.sources, cfg.lang)
		tick++
		// fyne.io/systray on Linux keeps, but never publishes, an icon set while it
		// is still exporting its D-Bus objects. Seen in CI on Ubuntu 26.04: the
		// tooltip said "1 waiting" while the dot stayed grey. So the icon is also
		// re-sent during the first ticks and then now and then, not only on change.
		if v.State != prev || resendIcon(tick) {
			systray.SetIcon(iconBytes(v.State))
		}
		if v.State != prev {
			cfg.log.Printf("%-6s %s", v.State, summary(v))
			if v.State == state.Red && flashOn.Load() && desktop.CanFlash() {
				desktop.FlashClaude()
			}
			prev = v.State
		}
		systray.SetTooltip(truncate("AgentSemaphore: "+v.Tooltip, 127))
		rows := menuRows(v)
		for i, it := range items {
			if i < len(rows) {
				it.SetTitle(rows[i])
				it.Show()
			} else {
				it.Hide()
			}
		}
		if extra := len(rows) - maxMenuRows; extra > 0 {
			more.SetTitle(fmt.Sprintf("+%d …", extra))
			more.Show()
		} else {
			more.Hide()
		}
	}
	go func() {
		refresh()
		t := time.NewTicker(cfg.interval)
		defer t.Stop()
		for range t.C {
			refresh()
		}
	}()
}

func summary(v aggregate.View) string {
	s := ""
	for _, x := range v.Sessions {
		if x.State == v.State {
			if s != "" {
				s += "; "
			}
			s += x.Name
			if x.Detail != "" {
				s += " [" + x.Detail + "]"
			}
		}
	}
	return s
}

// truncate keeps the tooltip within the Windows limit (127 UTF-16 units),
// cutting on a rune boundary.
func truncate(s string, max int) string {
	units := 0
	for i, r := range s {
		n := 1
		if r > 0xFFFF {
			n = 2
		}
		if units+n > max-1 {
			return s[:i] + "…"
		}
		units += n
	}
	return s
}

// leftClickOpensClaude: on macOS fyne.io/systray (v1.12.2, systray_darwin.m)
// sends every click on the menu bar item, right and Control clicks included,
// to the left-click handler, so with one set the menu could never be opened.
// There a click shows the menu, as menu bar items usually do, and Claude is
// opened from its "Open Claude" item.
func leftClickOpensClaude(goos string) bool { return goos != "darwin" }

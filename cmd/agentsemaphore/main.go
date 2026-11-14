// Command agentsemaphore shows a traffic-light dot in the system tray / menu
// bar with the worst state of your AI agent sessions.
//
//	agentsemaphore            run the tray app
//	agentsemaphore -once      read every source once, print the result as JSON and exit
//	agentsemaphore -version   print the version and exit
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/LuisPuertollano/AgentSemaphore/internal/aggregate"
	"github.com/LuisPuertollano/AgentSemaphore/internal/config"
	"github.com/LuisPuertollano/AgentSemaphore/internal/i18n"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source/claudecode"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source/toasts"
)

// version is set at build time: -ldflags "-X main.version=..."
var version = "dev"

func main() {
	once := flag.Bool("once", false, "read all sources once, print JSON and exit")
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("agentsemaphore", version)
		return
	}

	settingsPath, err := config.DefaultPath()
	if err != nil {
		log.Fatal(err)
	}
	settings, loadErr := config.Load(settingsPath)
	lang := i18n.Resolve(settings.Language)

	if *once {
		// Note: toasts that arrived before start count as seen, so -once never
		// shows notification reds; it is meant for checking sessions and health.
		v := poll(buildSources(settings, nil), lang)
		out := struct {
			Version  string          `json:"version"`
			Language string          `json:"language"`
			State    string          `json:"state"`
			Tooltip  string          `json:"tooltip"`
			Sessions []sessionJSON   `json:"sessions"`
			Settings config.Settings `json:"settings"`
			Warning  string          `json:"warning,omitempty"`
		}{Version: version, Language: lang, State: v.State.String(), Tooltip: v.Tooltip, Settings: settings, Sessions: []sessionJSON{}}
		for _, s := range v.Sessions {
			out.Sessions = append(out.Sessions, sessionJSON{s.Name, s.State.String(), s.Detail})
		}
		if loadErr != nil {
			out.Warning = loadErr.Error()
		}
		data, _ := json.MarshalIndent(out, "", "  ")
		// ASCII-only output: Windows consoles decode a child's stdout with the
		// OEM code page, which garbled "·" as "┬À" (seen on Windows 11, 2026-11-14).
		os.Stdout.Write(append(asciiJSON(data), '\n'))
		return
	}

	logger := openLog(filepath.Dir(settingsPath))
	if loadErr != nil {
		logger.Printf("settings: %v", loadErr)
	}
	runTray(trayConfig{
		sources:  buildSources(settings, logger),
		lang:     lang,
		interval: time.Duration(settings.PollIntervalMs) * time.Millisecond,
		log:      logger,
	})
}

// asciiJSON escapes every non-ASCII character of valid JSON as \uXXXX (with
// surrogate pairs above U+FFFF), so the output survives any console code page.
func asciiJSON(data []byte) []byte {
	var b strings.Builder
	for _, r := range string(data) {
		switch {
		case r < 0x80:
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			r1, r2 := utf16.EncodeRune(r)
			fmt.Fprintf(&b, "\\u%04x\\u%04x", r1, r2)
		}
	}
	return []byte(b.String())
}

type sessionJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

func buildSources(s config.Settings, logger *log.Logger) []source.Source {
	var out []source.Source
	if s.Sources.ClaudeCode {
		out = append(out, claudecode.New())
	}
	if s.Sources.ClaudeDesktopToasts {
		ts := &toasts.Source{P: toasts.NewPlatform()}
		if logger != nil {
			ts.OnUnknown = func(t toasts.Stored, i toasts.Info) {
				if i.Kind == toasts.Progress {
					logger.Printf("silent Code-session summary (not red): %s", i)
					return
				}
				logger.Printf("unrecognised Claude notification (counts as red): %s", i)
			}
		}
		out = append(out, ts) // absent outside Windows
	}
	return out
}

func poll(sources []source.Source, lang string) aggregate.View {
	results := make([]source.Result, 0, len(sources))
	for _, s := range sources {
		results = append(results, safePoll(s))
	}
	return aggregate.Build(results, lang)
}

// safePoll turns a panicking source into an Error result (grey), never a crash.
func safePoll(s source.Source) (r source.Result) {
	defer func() {
		if p := recover(); p != nil {
			r = source.Result{Source: s.ID(), Health: source.Error, Detail: fmt.Sprint("internal error: ", p)}
		}
	}()
	return s.Poll()
}

// openLog appends to agentsemaphore.log next to the settings file; falls back to stderr.
func openLog(dir string) *log.Logger {
	if err := os.MkdirAll(dir, 0o755); err == nil {
		if f, err := os.OpenFile(filepath.Join(dir, "agentsemaphore.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			return log.New(f, "", log.LstdFlags)
		}
	}
	return log.New(os.Stderr, "", log.LstdFlags)
}

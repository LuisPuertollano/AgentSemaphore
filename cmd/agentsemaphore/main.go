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
	"time"

	"github.com/LuisPuertollano/AgentSemaphore/internal/aggregate"
	"github.com/LuisPuertollano/AgentSemaphore/internal/config"
	"github.com/LuisPuertollano/AgentSemaphore/internal/i18n"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source"
	"github.com/LuisPuertollano/AgentSemaphore/internal/source/claudecode"
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
	sources := buildSources(settings)

	if *once {
		v := poll(sources, lang)
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
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(out)
		return
	}

	logger := openLog(filepath.Dir(settingsPath))
	if loadErr != nil {
		logger.Printf("settings: %v", loadErr)
	}
	runTray(trayConfig{
		sources:  sources,
		lang:     lang,
		interval: time.Duration(settings.PollIntervalMs) * time.Millisecond,
		log:      logger,
	})
}

type sessionJSON struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

func buildSources(s config.Settings) []source.Source {
	var out []source.Source
	if s.Sources.ClaudeCode {
		out = append(out, claudecode.New())
	}
	// Claude Desktop notifications (Windows) arrive with issue #16.
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

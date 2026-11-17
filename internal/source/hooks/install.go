package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Events each agent is registered for. Copilot's preToolUse is deliberately
// absent: it fails closed (a hook error denies the tool call).
var installEvents = map[string][]string{
	"copilot": {"sessionStart", "userPromptSubmitted", "postToolUse", "postToolUseFailure", "notification", "agentStop", "sessionEnd"},
	"codex":   {"SessionStart", "UserPromptSubmit", "PermissionRequest", "PostToolUse", "Stop", "Interrupt", "SessionEnd"},
	"gemini":  {"SessionStart", "BeforeAgent", "BeforeTool", "AfterTool", "AfterAgent", "Notification", "SessionEnd"},
}

// hookCommand is the shell command an agent runs for one event. The path is
// in double quotes as is (no escaping: Windows paths keep single backslashes;
// a path with a double quote in it is refused by Install).
func hookCommand(exe, agent, event string) string {
	return `"` + exe + `" hook ` + agent + " " + event
}

// isOurs recognises a command this package wrote, whatever the exe path.
func isOurs(cmd, agent string) bool {
	return strings.Contains(strings.ToLower(cmd), "agentsemaphore") && strings.Contains(cmd, " hook "+agent+" ")
}

func homeOr(env, home, sub string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	return filepath.Join(home, sub)
}

// CopilotFile is our own hook file for GitHub Copilot CLI
// ($COPILOT_HOME/hooks or ~/.copilot/hooks; every *.json there is loaded).
func CopilotFile(home string) string {
	return filepath.Join(homeOr("COPILOT_HOME", home, ".copilot"), "hooks", "agentsemaphore.json")
}

// CodexFile is the user-level hooks file of OpenAI Codex CLI ($CODEX_HOME/hooks.json).
func CodexFile(home string) string {
	return filepath.Join(homeOr("CODEX_HOME", home, ".codex"), "hooks.json")
}

func copilotConfig(exe string) map[string]any {
	hooks := map[string]any{}
	for _, e := range installEvents["copilot"] {
		cmd := hookCommand(exe, "copilot", e)
		hooks[e] = []any{map[string]any{
			"type": "command", "bash": cmd, "powershell": "& " + cmd, "timeoutSec": 10,
		}}
	}
	return map[string]any{"version": 1, "hooks": hooks}
}

func codexGroup(exe, event string) map[string]any {
	cmd := hookCommand(exe, "codex", event)
	return map[string]any{"hooks": []any{map[string]any{
		"type": "command", "command": cmd, "commandWindows": cmd, "timeout": 10,
	}}}
}

func geminiGroup(exe, event string) map[string]any {
	return map[string]any{"hooks": []any{map[string]any{
		"type": "command", "name": "agentsemaphore", "command": hookCommand(exe, "gemini", event), "timeout": 10000,
	}}}
}

// Snippet is the configuration for agent, to show or to paste by hand.
func Snippet(agent, exe string) (string, error) {
	var v any
	switch agent {
	case "copilot":
		v = copilotConfig(exe)
	case "codex", "gemini":
		hooks := map[string]any{}
		for _, e := range installEvents[agent] {
			if agent == "codex" {
				hooks[e] = []any{codexGroup(exe, e)}
			} else {
				hooks[e] = []any{geminiGroup(exe, e)}
			}
		}
		v = map[string]any{"hooks": hooks}
	default:
		return "", fmt.Errorf("unknown agent %q (copilot, codex, gemini)", agent)
	}
	return string(marshal(v)), nil
}

// Install registers our hooks for agent and returns the file it wrote.
// Copilot gets its own file. Codex: our entries are added to hooks.json next
// to the user's, which stay untouched. Gemini keeps hooks in the user's main
// settings.json, so it is not edited automatically: print the Snippet instead.
func Install(agent, exe, home string) (string, error) {
	if strings.ContainsAny(exe, `"`+"\n") {
		return "", fmt.Errorf("the program path %q cannot be used in a hook command", exe)
	}
	switch agent {
	case "copilot":
		path := CopilotFile(home)
		return path, writeFile(path, marshal(copilotConfig(exe)))
	case "codex":
		path := CodexFile(home)
		doc, err := readJSON(path)
		if err != nil {
			return path, err
		}
		removeOurs(doc, "codex")
		hooks, _ := doc["hooks"].(map[string]any)
		if hooks == nil {
			hooks = map[string]any{}
			doc["hooks"] = hooks
		}
		for _, e := range installEvents["codex"] {
			list, _ := hooks[e].([]any)
			hooks[e] = append(list, codexGroup(exe, e))
		}
		return path, writeFile(path, marshal(doc))
	case "gemini":
		return "", errors.New("Gemini CLI keeps hooks in your main settings.json, which is not edited automatically: run 'agentsemaphore hooks show gemini' and add the \"hooks\" part by hand")
	}
	return "", fmt.Errorf("unknown agent %q (copilot, codex, gemini)", agent)
}

// Uninstall removes only what Install added. It reports whether anything was removed.
func Uninstall(agent, home string) (string, bool, error) {
	switch agent {
	case "copilot":
		path := CopilotFile(home)
		err := os.Remove(path)
		if errors.Is(err, fs.ErrNotExist) {
			return path, false, nil
		}
		return path, err == nil, err
	case "codex":
		path := CodexFile(home)
		if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
			return path, false, nil
		}
		doc, err := readJSON(path)
		if err != nil {
			return path, false, err
		}
		if !removeOurs(doc, "codex") {
			return path, false, nil
		}
		return path, true, writeFile(path, marshal(doc))
	}
	return "", false, fmt.Errorf("unknown agent %q (copilot, codex)", agent)
}

// Installed reports whether our hooks are registered for agent.
func Installed(agent, home string) bool {
	switch agent {
	case "copilot":
		_, err := os.Stat(CopilotFile(home))
		return err == nil
	case "codex":
		doc, err := readJSON(CodexFile(home))
		if err != nil {
			return false
		}
		return removeOurs(doc, "codex") // works on a copy read just for this
	}
	return false
}

// readJSON loads a JSON object; a missing file is an empty object. Anything
// that is not plain JSON is refused rather than rewritten.
func readJSON(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if len(strings.TrimSpace(string(b))) == 0 {
		return doc, nil
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("%s is not plain JSON, so it is left alone (%v)", path, err)
	}
	return doc, nil
}

// removeOurs drops our handlers from doc["hooks"], and groups or events left
// empty by that. It reports whether it removed anything.
func removeOurs(doc map[string]any, agent string) bool {
	hooks, _ := doc["hooks"].(map[string]any)
	removed := false
	for event, v := range hooks {
		groups, _ := v.([]any)
		var keep []any
		for _, g := range groups {
			gm, ok := g.(map[string]any)
			if !ok {
				keep = append(keep, g)
				continue
			}
			handlers, _ := gm["hooks"].([]any)
			var left []any
			for _, h := range handlers {
				hm, _ := h.(map[string]any)
				cmd, _ := hm["command"].(string)
				if hm != nil && isOurs(cmd, agent) {
					removed = true
					continue
				}
				left = append(left, h)
			}
			if len(left) == 0 && len(handlers) > 0 {
				continue
			}
			if len(left) != len(handlers) {
				gm["hooks"] = left
			}
			keep = append(keep, gm)
		}
		if len(keep) == 0 {
			delete(hooks, event)
		} else {
			hooks[event] = keep
		}
	}
	return removed
}

// marshal is indented JSON without HTML escaping, so "&" stays readable in
// the PowerShell command.
func marshal(v any) []byte {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
	return b.Bytes()
}

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".agentsemaphore-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path) // rename does not replace on every Windows file system
	}
	return os.Rename(tmp, path)
}

package usage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// ClaudeSettings is Claude Code's user settings file ($CLAUDE_CONFIG_DIR or ~/.claude).
func ClaudeSettings(home string) string {
	if d := os.Getenv("CLAUDE_CONFIG_DIR"); d != "" {
		return filepath.Join(d, "settings.json")
	}
	return filepath.Join(home, ".claude", "settings.json")
}

// backup records what statusLine was before Install, so Uninstall can put it back.
type backup struct {
	Settings string          `json:"settings"`
	Existed  bool            `json:"existed"`  // statusLine was set before
	Previous json.RawMessage `json:"previous"` // its exact value then
}

func backupFile(dir string) string { return filepath.Join(dir, "claude-statusline-backup.json") }

// statusCommand is what Claude Code runs as its status line.
func statusCommand(exe string) string { return `"` + exe + `" statusline` }

// IsOurs recognises our status line command, whatever the program path.
func IsOurs(cmd string) bool {
	return strings.Contains(strings.ToLower(cmd), "agentsemaphore") && strings.HasSuffix(strings.TrimSpace(cmd), " statusline")
}

// ordered is a top-level JSON object that keeps its key order on rewrite.
type ordered struct {
	keys []string
	vals map[string]json.RawMessage
}

func readOrdered(path string) (*ordered, bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &ordered{vals: map[string]json.RawMessage{}}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	o := &ordered{vals: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(b)) == 0 {
		return o, true, nil
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, true, fmt.Errorf("%s is not a plain JSON object, so it is left alone", path)
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, true, fmt.Errorf("%s is not plain JSON, so it is left alone (%v)", path, err)
		}
		k, _ := t.(string)
		var v json.RawMessage
		if err := dec.Decode(&v); err != nil {
			return nil, true, fmt.Errorf("%s is not plain JSON, so it is left alone (%v)", path, err)
		}
		if _, dup := o.vals[k]; !dup {
			o.keys = append(o.keys, k)
		}
		o.vals[k] = v
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return nil, true, fmt.Errorf("%s is not plain JSON, so it is left alone", path)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, true, fmt.Errorf("%s has data after its JSON object, so it is left alone", path)
	}
	return o, true, nil
}

func (o *ordered) set(k string, v json.RawMessage) {
	if _, ok := o.vals[k]; !ok {
		o.keys = append(o.keys, k)
	}
	o.vals[k] = v
}

func (o *ordered) del(k string) {
	delete(o.vals, k)
	for i, x := range o.keys {
		if x == k {
			o.keys = append(o.keys[:i], o.keys[i+1:]...)
			return
		}
	}
}

func (o *ordered) bytes() []byte {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range o.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		b.Write(kb)
		b.WriteByte(':')
		b.Write(o.vals[k])
	}
	b.WriteByte('}')
	var out bytes.Buffer
	if json.Indent(&out, b.Bytes(), "", "  ") != nil {
		return append(b.Bytes(), '\n')
	}
	return append(out.Bytes(), '\n')
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".agentsemaphore-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	_ = os.Remove(path) // Windows: rename does not replace everywhere
	return os.Rename(tmp, path)
}

// currentCommand returns statusLine.command, if any.
func currentCommand(raw json.RawMessage) string {
	var sl struct {
		Command string `json:"command"`
	}
	_ = json.Unmarshal(raw, &sl)
	return sl.Command
}

// PreviousCommand is the user's own status line command that the wrapper
// passes the input to ("" when there was none, or it was not a command).
func PreviousCommand(dir string) string {
	b, err := os.ReadFile(backupFile(dir))
	if err != nil {
		return ""
	}
	var bk backup
	if json.Unmarshal(b, &bk) != nil || !bk.Existed {
		return ""
	}
	cmd := currentCommand(bk.Previous)
	if IsOurs(cmd) {
		return ""
	}
	return cmd
}

// InstallClaude points Claude Code's status line at "exe statusline" and
// remembers the previous value. Other settings keep their keys and order.
func InstallClaude(dir, exe, home string) (string, error) {
	if strings.ContainsAny(exe, "\"\n") {
		return "", fmt.Errorf("the program path %q cannot be used in a command", exe)
	}
	path := ClaudeSettings(home)
	o, _, err := readOrdered(path)
	if err != nil {
		return path, err
	}
	prev, existed := o.vals["statusLine"]
	if existed && IsOurs(currentCommand(prev)) {
		// Already ours: keep the original backup, only refresh the command.
		existed = false
	} else {
		bk, _ := json.Marshal(backup{Settings: path, Existed: existed, Previous: prev})
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return path, err
		}
		if err := os.WriteFile(backupFile(dir), bk, 0o644); err != nil {
			return path, err
		}
	}
	sl := map[string]any{}
	if len(prev) > 0 {
		_ = json.Unmarshal(prev, &sl) // keep padding, refreshInterval…
	}
	sl["type"] = "command"
	sl["command"] = statusCommand(exe)
	v, _ := json.Marshal(sl)
	o.set("statusLine", v)
	return path, writeAtomic(path, o.bytes())
}

// UninstallClaude restores the status line from before InstallClaude. If the
// user changed it since, it is left as it is. It reports what it did.
func UninstallClaude(dir, home string) (string, string, error) {
	path := ClaudeSettings(home)
	b, err := os.ReadFile(backupFile(dir))
	if errors.Is(err, fs.ErrNotExist) {
		return path, "not installed", nil
	}
	if err != nil {
		return path, "", err
	}
	var bk backup
	if err := json.Unmarshal(b, &bk); err != nil {
		return path, "", err
	}
	o, _, err := readOrdered(path)
	if err != nil {
		return path, "", err
	}
	cur, ok := o.vals["statusLine"]
	if !ok || !IsOurs(currentCommand(cur)) {
		os.Remove(backupFile(dir))
		return path, "status line was changed since; left as it is", nil
	}
	if bk.Existed {
		o.set("statusLine", bk.Previous)
	} else {
		o.del("statusLine")
	}
	if err := writeAtomic(path, o.bytes()); err != nil {
		return path, "", err
	}
	os.Remove(backupFile(dir))
	return path, "restored", nil
}

// ClaudeInstalled reports whether Claude Code's status line is ours.
func ClaudeInstalled(home string) bool {
	o, _, err := readOrdered(ClaudeSettings(home))
	if err != nil {
		return false
	}
	return IsOurs(currentCommand(o.vals["statusLine"]))
}

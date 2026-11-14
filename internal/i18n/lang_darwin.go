//go:build darwin

package i18n

import (
	"os/exec"
	"strings"
)

// systemLanguage prefers the environment (set in terminals) and otherwise
// asks macOS for the user's preferred languages, because apps started from
// Finder or at login have no LANG.
func systemLanguage() string {
	if l := fromEnv(); l != "" {
		return l
	}
	out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output()
	if err != nil {
		return ""
	}
	// Output looks like: (\n    "de-DE",\n    "en-US"\n)
	for _, f := range strings.FieldsFunc(string(out), func(r rune) bool {
		return r == '(' || r == ')' || r == ',' || r == '"' || r == '\n' || r == ' '
	}) {
		if normalize(f) != "" {
			return f
		}
	}
	return ""
}

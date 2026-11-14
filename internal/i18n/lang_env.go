package i18n

import (
	"os"
	"strings"
)

// fromEnv returns the first language found in LANGUAGE, LC_ALL, LC_MESSAGES
// or LANG. LANGUAGE may hold a colon-separated priority list.
func fromEnv() string {
	for _, v := range []string{"LANGUAGE", "LC_ALL", "LC_MESSAGES", "LANG"} {
		for _, part := range strings.Split(os.Getenv(v), ":") {
			if normalize(part) != "" {
				return part
			}
		}
	}
	return ""
}

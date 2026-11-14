// Package i18n holds the UI strings in English, Spanish and German and picks
// the language: the user's setting, else the OS UI language, else English.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

// Fallback is used when neither the setting nor the OS gives a supported language.
const Fallback = "en"

//go:embed locales/*.json
var files embed.FS

// catalogs maps language code → key → text. Filled once at init; a broken
// embedded catalog is a build bug and is caught by the tests.
var catalogs = mustLoad()

func mustLoad() map[string]map[string]string {
	out := map[string]map[string]string{}
	entries, err := files.ReadDir("locales")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		lang := strings.TrimSuffix(e.Name(), ".json")
		data, err := files.ReadFile("locales/" + e.Name())
		if err != nil {
			panic(err)
		}
		m := map[string]string{}
		if err := json.Unmarshal(data, &m); err != nil {
			panic(fmt.Sprintf("locale %s: %v", lang, err))
		}
		out[lang] = m
	}
	return out
}

// Languages returns the language codes that have a catalog.
func Languages() []string {
	out := make([]string, 0, len(catalogs))
	for l := range catalogs {
		out = append(out, l)
	}
	return out
}

// Resolve turns a setting ("auto", "en", "es", "de", …) into a language that
// has a catalog. "auto" or anything unknown follows the OS, then Fallback.
func Resolve(setting string) string {
	if l := normalize(setting); l != "" {
		return l
	}
	if l := normalize(systemLanguage()); l != "" {
		return l
	}
	return Fallback
}

// normalize maps "de_DE.UTF-8", "es-ES", "EN" … to a supported code, or "".
func normalize(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" || tag == "auto" || tag == "c" || tag == "posix" {
		return ""
	}
	if i := strings.IndexAny(tag, "_-.@"); i > 0 {
		tag = tag[:i]
	}
	if _, ok := catalogs[tag]; ok {
		return tag
	}
	return ""
}

// T returns the text for key in lang, with {name} placeholders replaced by
// args given as name/value pairs. Missing keys fall back to English and then
// to the key itself, so the UI never shows an empty string.
func T(lang, key string, args ...string) string {
	s, ok := catalogs[lang][key]
	if !ok {
		s, ok = catalogs[Fallback][key]
	}
	if !ok {
		s = key
	}
	for i := 0; i+1 < len(args); i += 2 {
		s = strings.ReplaceAll(s, "{"+args[i]+"}", args[i+1])
	}
	return s
}

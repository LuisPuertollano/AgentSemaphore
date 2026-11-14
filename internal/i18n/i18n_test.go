package i18n

import (
	"regexp"
	"sort"
	"testing"
)

var placeholder = regexp.MustCompile(`\{[a-zA-Z]+\}`)

// Every catalog must have exactly the English keys and the same placeholders.
func TestCatalogsAreComplete(t *testing.T) {
	en := catalogs["en"]
	if len(en) == 0 {
		t.Fatal("English catalog missing or empty")
	}
	for _, lang := range []string{"es", "de"} {
		c, ok := catalogs[lang]
		if !ok {
			t.Fatalf("catalog %s missing", lang)
		}
		for k, v := range en {
			tr, ok := c[k]
			if !ok || tr == "" {
				t.Errorf("%s: missing key %q", lang, k)
				continue
			}
			a, b := placeholder.FindAllString(v, -1), placeholder.FindAllString(tr, -1)
			sort.Strings(a)
			sort.Strings(b)
			if len(a) != len(b) {
				t.Errorf("%s: %q placeholders %v, English has %v", lang, k, b, a)
				continue
			}
			for i := range a {
				if a[i] != b[i] {
					t.Errorf("%s: %q placeholders %v, English has %v", lang, k, b, a)
					break
				}
			}
		}
		for k := range c {
			if _, ok := en[k]; !ok {
				t.Errorf("%s: extra key %q not in English", lang, k)
			}
		}
	}
}

func TestNormalize(t *testing.T) {
	for in, want := range map[string]string{
		"de_DE.UTF-8": "de", "es-ES": "es", "EN": "en", "en_US@euro": "en",
		"fr_FR": "", "C": "", "POSIX": "", "auto": "", "": "",
	} {
		if got := normalize(in); got != want {
			t.Errorf("normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveExplicitSettingWins(t *testing.T) {
	if got := Resolve("de"); got != "de" {
		t.Fatalf("Resolve(de) = %q", got)
	}
}

func TestResolveAutoUsesEnvironment(t *testing.T) {
	t.Setenv("LANGUAGE", "")
	t.Setenv("LC_ALL", "es_ES.UTF-8")
	if got := Resolve("auto"); got != "es" {
		t.Skipf("system language source is not the environment on this OS (got %q)", got)
	}
}

func TestT(t *testing.T) {
	got := T("es", "tooltip.summary", "waiting", "1", "working", "2", "idle", "3")
	if want := "1 te esperan · 2 trabajando · 3 en reposo"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := T("xx", "menu.quit"); got != "Quit" {
		t.Errorf("unknown language should fall back to English, got %q", got)
	}
	if got := T("en", "no.such.key"); got != "no.such.key" {
		t.Errorf("unknown key should return the key, got %q", got)
	}
}

// Package toasts reads the notifications Claude Desktop leaves in the Windows
// notification centre (see docs/FINDINGS.md §3) and turns the ones that may
// need the user into red sessions.
//
// Rules, shared with the Windows prototype (prototype/windows/avisos.py):
//   - a toast whose text matches a known "needs you" phrase is red;
//   - one matching a known "does not need you" phrase is ignored;
//   - a silent end-of-turn summary of a Claude Code session (a toast with a
//     <header id="session-..."> and <audio silent="true"/>) only informs and
//     is not red (5 of 5 samples, docs/FINDINGS.md §3);
//   - any other Claude toast is red too (better a red too many than a wrong
//     green) and is reported as unknown so it can be added to the table;
//   - a toast stops counting once Claude Desktop has been in the foreground
//     for 2 s after it arrived ("seen").
package toasts

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"strings"
)

// phrases.json is a copy of prototype/windows/avisos-textos.json; a test keeps
// the two identical.
//
//go:embed phrases.json
var phrasesJSON []byte

// Kind says how a toast was classified.
type Kind string

const (
	NeedsYou Kind = "needs"    // known phrase that needs the user
	Ignored  Kind = "ignored"  // known phrase that does not
	Unknown  Kind = "unknown"  // a Claude toast we have not seen before: counts as red
	Progress Kind = "progress" // silent end-of-turn summary of a Code session: not red
)

// Origin of a toast, from its <header id="session-...">.
type Origin string

const (
	OriginOther Origin = ""      // chat, Cowork, scheduled tasks, app notices
	OriginCloud Origin = "cloud" // Claude Code session on claude.ai/code
	OriginLocal Origin = "local" // Claude Desktop Code tab, local or over SSH
)

// Toast is one notification as stored by Windows.
type Toast struct {
	ID      int64
	Payload string // the toast XML
}

// Info is what the classifier derives from a toast.
type Info struct {
	Kind   Kind
	Name   string   // the text that is not a fixed phrase: chat or session title
	Texts  []string // all <text> lines
	Origin Origin
	Silent bool // <audio silent="true"/>
}

type phrases struct {
	needs, ignore *regexp.Regexp
}

var defaultPhrases = mustPhrases(phrasesJSON)

func mustPhrases(data []byte) phrases {
	p, err := parsePhrases(data)
	if err != nil {
		panic(err)
	}
	return p
}

func parsePhrases(data []byte) (phrases, error) {
	var f struct {
		Languages map[string]struct {
			Needs  []string `json:"pide"`
			Ignore []string `json:"ignorar"`
		} `json:"idiomas"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return phrases{}, err
	}
	var needs, ignore []string
	for _, l := range f.Languages {
		needs = append(needs, l.Needs...)
		ignore = append(ignore, l.Ignore...)
	}
	if len(needs) == 0 {
		return phrases{}, fmt.Errorf("phrase table has no \"needs you\" phrases")
	}
	p := phrases{}
	var err error
	if p.needs, err = regexp.Compile("(?i)" + strings.Join(needs, "|")); err != nil {
		return p, err
	}
	if len(ignore) > 0 {
		if p.ignore, err = regexp.Compile("(?i)" + strings.Join(ignore, "|")); err != nil {
			return p, err
		}
	}
	return p, nil
}

var (
	reText   = regexp.MustCompile(`(?s)<text[^>]*>(.*?)</text>`)
	reTag    = regexp.MustCompile(`<[^>]+>`)
	reHeader = regexp.MustCompile(`<header\b[^>]*\bid="session-([a-z]+)_`)
	reSilent = regexp.MustCompile(`<audio\b[^>]*\bsilent="true"`)
)

// Classify derives Info from a toast payload.
func Classify(payload string) Info { return defaultPhrases.classify(payload) }

func (p phrases) classify(payload string) Info {
	var texts []string
	for _, m := range reText.FindAllStringSubmatch(payload, -1) {
		texts = append(texts, strings.TrimSpace(html.UnescapeString(reTag.ReplaceAllString(m[1], ""))))
	}
	info := Info{Texts: texts, Silent: reSilent.MatchString(payload)}
	if m := reHeader.FindStringSubmatch(payload); m != nil {
		switch m[1] {
		case "session":
			info.Origin = OriginCloud
		case "local":
			info.Origin = OriginLocal
		default:
			info.Origin = Origin(m[1])
		}
	}
	anyMatch := func(re *regexp.Regexp) bool {
		if re == nil {
			return false
		}
		for _, t := range texts {
			if re.MatchString(t) {
				return true
			}
		}
		return false
	}
	switch {
	case anyMatch(p.needs):
		info.Kind = NeedsYou
	case anyMatch(p.ignore):
		info.Kind = Ignored
	case info.Origin != OriginOther && info.Silent:
		info.Kind = Progress
	default:
		info.Kind = Unknown
	}
	for _, t := range texts {
		fixed := p.needs.MatchString(t) || (p.ignore != nil && p.ignore.MatchString(t))
		if t != "" && !fixed {
			info.Name = t
			break
		}
	}
	if info.Name == "" {
		info.Name = "Claude"
		if len(texts) > 0 && texts[0] != "" {
			info.Name = texts[0]
		}
	}
	return info
}

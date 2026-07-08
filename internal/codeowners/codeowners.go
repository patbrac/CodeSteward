// Package codeowners discovers, parses, matches, and validates CODEOWNERS
// files for the GitHub and GitLab dialects. It is deterministic: identical
// input files always yield identical rules, warnings, and match results.
package codeowners

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Dialect selects CODEOWNERS parsing/matching semantics.
type Dialect string

// Supported dialects.
const (
	DialectGitHub Dialect = "github"
	DialectGitLab Dialect = "gitlab"
	DialectAuto   Dialect = "auto"
)

// Rule is one parsed CODEOWNERS entry.
type Rule struct {
	Pattern string
	Owners  []string
	Line    int
	Section string // "" for github
}

// ParseWarning is a non-fatal problem found while parsing.
type ParseWarning struct {
	Line int
	Text string

	// kind classifies the warning for Validate's severity mapping. It is an
	// internal implementation detail and not part of the exported surface.
	kind warnKind
}

// warnKind classifies a ParseWarning so Validate can map it to an
// "error:"/"warning:" severity for the CLI.
type warnKind int

const (
	kindUnsupported     warnKind = iota // unsupported syntax (warning)
	kindMalformedOwner                  // owner token is not @user/@org/team/email (warning)
	kindSectionInGithub                 // [Section] header under the github dialect (warning)
	kindInvalidPattern                  // pattern is not a valid glob (error)
	kindInvalidLine                     // line could not be interpreted as a rule (error)
)

// File is a parsed CODEOWNERS file.
type File struct {
	Path     string
	Dialect  Dialect
	Rules    []Rule
	Warnings []ParseWarning

	// optionalSections records GitLab "^[Section]" optional-section names
	// (lower-cased). Optional sections are excluded from ownership matching
	// per the v0 scoring contract. Internal only.
	optionalSections map[string]bool
}

// sectionPolicy controls how "[Section]" headers are handled while parsing.
type sectionPolicy int

const (
	secReject     sectionPolicy = iota // github: warn and ignore section headers
	secAccept                          // gitlab: parse sections silently
	secAcceptWarn                      // auto/lenient: parse sections but warn
)

// Discover locates the CODEOWNERS file for the given dialect. It returns the
// filesystem path of the first existing candidate, or "" (with a nil error)
// when none exists.
//
// Search order — github: .github/CODEOWNERS, CODEOWNERS, docs/CODEOWNERS.
// gitlab: CODEOWNERS, docs/CODEOWNERS, .gitlab/CODEOWNERS. auto: the union in
// order .github/CODEOWNERS, CODEOWNERS, docs/CODEOWNERS, .gitlab/CODEOWNERS.
func Discover(root string, dialect Dialect) (string, error) {
	var order []string
	switch dialect {
	case DialectGitHub:
		order = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}
	case DialectGitLab:
		order = []string{"CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}
	case DialectAuto, "":
		order = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS", ".gitlab/CODEOWNERS"}
	default:
		return "", fmt.Errorf("codesteward: unknown codeowners dialect %q", dialect)
	}
	for _, rel := range order {
		full := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(full)
		if err == nil && !info.IsDir() {
			return full, nil
		}
	}
	return "", nil
}

// ParseFile reads and parses the CODEOWNERS file at path.
func ParseFile(pathToFile string, dialect Dialect) (*File, error) {
	b, err := os.ReadFile(pathToFile)
	if err != nil {
		return nil, fmt.Errorf("codesteward: read CODEOWNERS %s: %w", pathToFile, err)
	}
	return Parse(b, pathToFile, dialect), nil
}

// Parse parses CODEOWNERS content. The "auto" dialect is resolved to a
// concrete dialect using sourcePath: files under .github/ use github
// semantics, files under .gitlab/ use gitlab semantics, and anything else uses
// github semantics but still parses GitLab "[Section]" headers leniently
// (with a warning). Parse never returns nil.
func Parse(content []byte, sourcePath string, dialect Dialect) *File {
	resolved, policy := resolveDialect(sourcePath, dialect)
	f := &File{
		Path:             sourcePath,
		Dialect:          resolved,
		optionalSections: map[string]bool{},
	}

	warn := func(line int, kind warnKind, text string) {
		f.Warnings = append(f.Warnings, ParseWarning{Line: line, Text: text, kind: kind})
	}

	currentSection := ""
	sectionDefaults := map[string][]string{}

	lines := strings.Split(string(content), "\n")
	for idx, raw := range lines {
		lineNo := idx + 1
		line := strings.TrimRight(raw, "\r")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Section headers ("[Section]" or "^[Section]").
		if ok, name, optional, rest := parseSectionHeader(trimmed); ok {
			switch policy {
			case secReject:
				warn(lineNo, kindSectionInGithub,
					"section headers are not supported in the github dialect and were ignored")
				continue
			case secAcceptWarn:
				warn(lineNo, kindUnsupported,
					"GitLab section header parsed leniently under auto/github semantics")
			}
			currentSection = name
			key := strings.ToLower(name)
			if optional {
				f.optionalSections[key] = true
			}
			if def := parseSectionDefaults(rest, lineNo, warn); len(def) > 0 {
				sectionDefaults[key] = def
			}
			continue
		}

		tokens := tokenizeFields(line)
		if len(tokens) == 0 {
			continue
		}
		pat := tokens[0]
		owners := tokens[1:]

		if strings.HasPrefix(pat, "!") {
			warn(lineNo, kindUnsupported,
				`negation patterns ("!") are not supported and were ignored`)
			continue
		}
		if !validPattern(pat) {
			warn(lineNo, kindInvalidPattern, fmt.Sprintf("invalid pattern %q", pat))
			continue
		}

		cleanOwners := make([]string, 0, len(owners))
		for _, o := range owners {
			if !validOwner(o) {
				warn(lineNo, kindMalformedOwner,
					fmt.Sprintf("invalid owner %q (expected @user, @org/team, or an email address)", o))
			}
			cleanOwners = append(cleanOwners, o)
		}
		if len(cleanOwners) == 0 && currentSection != "" {
			if def, ok := sectionDefaults[strings.ToLower(currentSection)]; ok && len(def) > 0 {
				cleanOwners = append(cleanOwners, def...)
			}
		}

		f.Rules = append(f.Rules, Rule{
			Pattern: pat,
			Owners:  cleanOwners,
			Line:    lineNo,
			Section: currentSection,
		})
	}

	return f
}

// resolveDialect maps the requested dialect (possibly "auto") to a concrete
// dialect and a section-handling policy.
func resolveDialect(sourcePath string, dialect Dialect) (Dialect, sectionPolicy) {
	switch dialect {
	case DialectGitLab:
		return DialectGitLab, secAccept
	case DialectGitHub:
		return DialectGitHub, secReject
	default: // auto or unknown
		sp := filepath.ToSlash(sourcePath)
		switch {
		case strings.Contains(sp, ".github/"):
			return DialectGitHub, secReject
		case strings.Contains(sp, ".gitlab/"):
			return DialectGitLab, secAccept
		default:
			return DialectGitHub, secAcceptWarn
		}
	}
}

// parseSectionHeader reports whether s is a GitLab section header and, if so,
// returns the section name, whether it is optional ("^" prefix), and the
// remaining text (default owners and/or an approval-count token).
func parseSectionHeader(s string) (ok bool, name string, optional bool, rest string) {
	if strings.HasPrefix(s, "^") {
		optional = true
		s = s[1:]
	}
	if !strings.HasPrefix(s, "[") {
		return false, "", false, ""
	}
	end := strings.IndexByte(s, ']')
	if end < 2 { // need at least one character between the brackets
		return false, "", false, ""
	}
	name = strings.TrimSpace(s[1:end])
	if name == "" {
		return false, "", false, ""
	}
	rest = s[end+1:]
	if rest != "" {
		// A section header is followed by nothing, whitespace (default
		// owners), or "[" (an approval-count token). Otherwise the line is a
		// glob pattern such as "[abc].js" and not a section header.
		if c := rest[0]; c != ' ' && c != '\t' && c != '[' {
			return false, "", false, ""
		}
	}
	return true, name, optional, rest
}

// parseSectionDefaults extracts default-owner tokens from a section header's
// trailing text, warning on an unsupported approval-count token and on
// malformed owners.
func parseSectionDefaults(rest string, lineNo int, warn func(int, warnKind, string)) []string {
	r := strings.TrimSpace(rest)
	if strings.HasPrefix(r, "[") {
		if e := strings.IndexByte(r, ']'); e >= 0 {
			warn(lineNo, kindUnsupported,
				"approval-count in a section header is not supported and was ignored")
			r = strings.TrimSpace(r[e+1:])
		}
	}
	toks := tokenizeFields(r)
	out := make([]string, 0, len(toks))
	for _, o := range toks {
		if !validOwner(o) {
			warn(lineNo, kindMalformedOwner,
				fmt.Sprintf("invalid default owner %q (expected @user, @org/team, or an email address)", o))
		}
		out = append(out, o)
	}
	return out
}

// tokenizeFields splits a CODEOWNERS line into whitespace-separated tokens.
// A backslash escapes the following character (so "a\ b" is a single token
// "a b"). A "#" begins a trailing comment only when it is not part of a token
// (i.e. it is at the start of the line or preceded by whitespace).
func tokenizeFields(s string) []string {
	var tokens []string
	var cur []byte
	inToken := false
	escaped := false

	flush := func() {
		if inToken {
			tokens = append(tokens, string(cur))
			cur = cur[:0]
			inToken = false
		}
	}

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			cur = append(cur, c)
			inToken = true
			escaped = false
			continue
		}
		switch c {
		case '\\':
			escaped = true
		case ' ', '\t':
			flush()
		case '#':
			if !inToken {
				flush()
				return tokens
			}
			cur = append(cur, c)
		default:
			cur = append(cur, c)
			inToken = true
		}
	}
	if escaped { // trailing backslash: keep it literally
		cur = append(cur, '\\')
		inToken = true
	}
	flush()
	return tokens
}

var (
	// @username: letters, digits, hyphens (may not start/end with a hyphen).
	ownerUserRe = regexp.MustCompile(`^@[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?$`)
	// @org/team.
	ownerTeamRe = regexp.MustCompile(`^@[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._/-]*$`)
	// email address (deliberately permissive but requires user@host.tld).
	ownerEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
)

// validOwner reports whether tok is a well-formed owner reference.
func validOwner(tok string) bool {
	return ownerUserRe.MatchString(tok) ||
		ownerTeamRe.MatchString(tok) ||
		ownerEmailRe.MatchString(tok)
}

// validPattern reports whether every segment of pattern is a well-formed glob
// (the only source of invalidity is a malformed character class such as "[").
func validPattern(pattern string) bool {
	core := strings.TrimSuffix(strings.TrimPrefix(pattern, "/"), "/")
	for _, seg := range strings.Split(core, "/") {
		if seg == "**" || seg == "" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return false
		}
	}
	return true
}

// ClassifyPattern classifies a CODEOWNERS pattern's ownership breadth.
//
//   - fallback: the pattern matches everything ("*", "**", "/**", "/").
//   - broad: an anchored single top-level segment ("/src/**", "/src/", "src/")
//     or a bare extension pattern ("*.md", "*.js").
//   - specific: anything deeper (>= 2 concrete path segments, e.g.
//     "/src/parser/**").
func ClassifyPattern(pattern string) model.OwnershipMatchClass {
	p := strings.TrimSpace(pattern)
	switch p {
	case "", "*", "**", "/**", "/":
		return model.MatchFallback
	}
	core := strings.TrimSuffix(strings.TrimPrefix(p, "/"), "/")
	if core == "" {
		return model.MatchFallback
	}
	if !strings.Contains(core, "/") {
		return model.MatchBroad
	}
	concrete := 0
	allWild := true
	for _, seg := range strings.Split(core, "/") {
		if seg == "**" || seg == "*" || seg == "" {
			continue
		}
		allWild = false
		concrete++
	}
	switch {
	case allWild:
		return model.MatchFallback
	case concrete >= 2:
		return model.MatchSpecific
	default:
		return model.MatchBroad
	}
}

// Validate returns the "error:"/"warning:"-prefixed problems for the
// `codesteward codeowners validate` command. Invalid patterns and unparseable
// lines are errors; malformed owners, empty owner lists, unsupported syntax,
// section headers under the github dialect, and a catch-all rule ordered after
// a more-specific rule (which the last-match-wins semantics would let it
// silently override) are warnings. The result is sorted by line, then
// severity, then text for determinism.
func Validate(f *File) []string {
	if f == nil {
		return nil
	}
	type problem struct {
		line int
		sev  int // 0 = error, 1 = warning
		text string
	}
	var problems []problem

	for _, w := range f.Warnings {
		sev := 1
		if w.kind == kindInvalidPattern || w.kind == kindInvalidLine {
			sev = 0
		}
		problems = append(problems, problem{line: w.Line, sev: sev, text: w.Text})
	}
	for _, r := range f.Rules {
		if len(r.Owners) == 0 {
			problems = append(problems, problem{
				line: r.Line,
				sev:  1,
				text: fmt.Sprintf("pattern %q has no owners", r.Pattern),
			})
		}
	}

	// A fallback (catch-all) rule placed after a more-specific rule silently
	// overrides those earlier rules, because the last matching rule wins. Flag
	// it so authors move the catch-all first. For the github dialect this is
	// evaluated across the whole file; for the gitlab dialect last-match-wins
	// applies within a section, so the comparison is scoped per section.
	sectionScoped := f.Dialect == DialectGitLab
	seenMoreSpecific := map[string]bool{}
	for _, r := range f.Rules {
		key := ""
		if sectionScoped {
			key = strings.ToLower(r.Section)
		}
		switch ClassifyPattern(r.Pattern) {
		case model.MatchFallback:
			if seenMoreSpecific[key] {
				problems = append(problems, problem{
					line: r.Line,
					sev:  1,
					text: fmt.Sprintf("catch-all pattern %q overrides all earlier rules because the last matching rule wins; consider moving it first", r.Pattern),
				})
			}
		case model.MatchBroad, model.MatchSpecific:
			seenMoreSpecific[key] = true
		}
	}

	sort.SliceStable(problems, func(i, j int) bool {
		if problems[i].line != problems[j].line {
			return problems[i].line < problems[j].line
		}
		if problems[i].sev != problems[j].sev {
			return problems[i].sev < problems[j].sev
		}
		return problems[i].text < problems[j].text
	})

	out := make([]string, 0, len(problems))
	for _, p := range problems {
		label := "warning"
		if p.sev == 0 {
			label = "error"
		}
		out = append(out, fmt.Sprintf("%s: line %d: %s", label, p.line, p.text))
	}
	return out
}

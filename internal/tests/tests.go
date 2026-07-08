// Package tests implements the path-aware test expectation engine. It maps
// changed production files to the test files that should accompany them using
// configurable {path}/{name}/{ext} templates, then classifies each file into a
// deterministic test state and emits the CS-TST-* findings.
package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/internal/config"
	"github.com/codesteward-ai/codesteward/internal/globs"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Options configures test expectation analysis.
type Options struct {
	RequireFor []string             // globs; files needing tests
	TestPaths  []string             // globs identifying test files
	Mappings   []config.PathMapping // source->test path mappings
	Root       string               // for on-disk existence checks
	Enabled    bool
}

// Analyze evaluates whether changed production files have matching test
// updates. It returns the aggregate TestsSummary (with a per-file breakdown)
// and the deterministic set of CS-TST-* findings.
//
// A changed file is "required" when it matches Options.RequireFor and is not a
// test, ignored, binary, or deleted file. For each required file the engine
// unions the Expect expansions of every From template that matches it, then
// classifies the file:
//
//   - matching_test_changed:                a candidate is among the changed test files
//   - existing_test_found_but_not_changed:  a candidate exists on disk under Root (CS-TST-002)
//   - missing_matching_test:                at least one From matched but no test was found (CS-TST-001)
//   - not_evaluated (per file):             no From template matched the file (CS-TST-003)
//
// When Enabled is false or no mappings are configured the whole summary is
// not_evaluated with no findings.
func Analyze(files []model.ChangedFile, opts Options) (model.TestsSummary, []model.Finding) {
	// Disabled or nothing to map: cannot evaluate anything.
	if !opts.Enabled || len(opts.Mappings) == 0 {
		return model.TestsSummary{State: model.TestsNotEvaluated}, nil
	}

	compiled := compileMappings(opts.Mappings)

	// Set of changed files that are test files (any status). Membership of a
	// candidate in this set means the matching test was updated in this change.
	changedTests := make(map[string]bool)
	for _, f := range files {
		if f.IsTest || matchAny(opts.TestPaths, f.Path) {
			changedTests[f.Path] = true
		}
	}

	var (
		expectations []model.FileTestExpectation
		findings     []model.Finding
		anyRequired  bool
		anyMissing   bool
		anyExisting  bool
		anyMatching  bool
	)

	for _, f := range files {
		if !isRequired(f, opts.RequireFor) {
			continue
		}
		anyRequired = true

		candidates, fromMatched := candidatesFor(f.Path, compiled)

		exp := model.FileTestExpectation{Path: f.Path, Candidates: candidates}

		switch {
		case anyCandidateChanged(candidates, changedTests):
			exp.State = model.TestsMatchingChanged
			exp.MatchedTest = firstChanged(candidates, changedTests)
			anyMatching = true

		case existsOnDisk(candidates, opts.Root) != "":
			test := existsOnDisk(candidates, opts.Root)
			exp.State = model.TestsExistingNotChanged
			exp.MatchedTest = test
			anyExisting = true
			findings = append(findings, model.Finding{
				RuleID:   model.RuleTstNotUpdated,
				Severity: model.SeverityWarning,
				Message:  "A matching test exists for `" + f.Path + "` (`" + test + "`), but this PR did not update it.",
				Action:   "Update `" + test + "` to cover the changes in `" + f.Path + "`.",
				Paths:    []string{f.Path},
			})

		case fromMatched:
			exp.State = model.TestsMissingMatching
			anyMissing = true
			findings = append(findings, model.Finding{
				RuleID:   model.RuleTstMissing,
				Severity: model.SeverityActionRequired,
				Message:  "`" + f.Path + "` changed, but no matching test file was changed.",
				Action:   "Add or update matching tests for `" + f.Path + "`.",
				Paths:    []string{f.Path},
			})

		default:
			// No From template matched this file at all: cannot be evaluated.
			exp.State = model.TestsNotEvaluated
			findings = append(findings, model.Finding{
				RuleID:   model.RuleTstUnresolved,
				Severity: model.SeverityInfo,
				Message:  "`" + f.Path + "` requires tests, but no configured path mapping matched it.",
				Action:   "Add a `tests.path_mappings` entry that maps `" + f.Path + "` to its tests.",
				Paths:    []string{f.Path},
			})
		}

		expectations = append(expectations, exp)
	}

	sort.Slice(expectations, func(i, j int) bool {
		return expectations[i].Path < expectations[j].Path
	})

	summary := model.TestsSummary{
		State: aggregateState(anyRequired, anyMissing, anyExisting, anyMatching),
		Files: expectations,
	}
	return summary, findings
}

// aggregateState reduces the per-file outcomes to a single summary state using
// the contracted precedence: missing > existing > matching > not_required. When
// files required tests but none could be mapped (all per-file not_evaluated),
// the aggregate is not_evaluated.
func aggregateState(anyRequired, anyMissing, anyExisting, anyMatching bool) model.TestState {
	switch {
	case anyMissing:
		return model.TestsMissingMatching
	case anyExisting:
		return model.TestsExistingNotChanged
	case anyMatching:
		return model.TestsMatchingChanged
	case !anyRequired:
		return model.TestsNotRequired
	default:
		// Files required tests but every one was unmapped (CS-TST-003).
		return model.TestsNotEvaluated
	}
}

// isRequired reports whether a changed file needs test coverage: it matches a
// require_for glob and is not a test, ignored, binary, or deleted file.
func isRequired(f model.ChangedFile, requireFor []string) bool {
	if f.IsTest || f.IsIgnored || f.IsBinary || f.Status == "deleted" {
		return false
	}
	return matchAny(requireFor, f.Path)
}

// matchAny reports whether path matches any of the globs.
func matchAny(patterns []string, path string) bool {
	_, ok := globs.MatchAny(patterns, path)
	return ok
}

// candidatesFor returns the sorted, deduplicated union of test candidates for
// path across every From template that matches it, plus whether any From
// matched at all.
func candidatesFor(path string, compiled []compiledMapping) (candidates []string, fromMatched bool) {
	set := make(map[string]bool)
	for _, m := range compiled {
		if m.re == nil {
			continue
		}
		sub := m.re.FindStringSubmatch(path)
		if sub == nil {
			continue
		}
		fromMatched = true
		var pathVal, nameVal, extVal string
		for k, kind := range m.groupOrder {
			switch kind {
			case phPath:
				pathVal = sub[k+1]
			case phName:
				nameVal = sub[k+1]
			case phExt:
				extVal = sub[k+1]
			}
		}
		for _, tmpl := range m.expect {
			set[expand(tmpl, pathVal, nameVal, extVal)] = true
		}
	}
	if len(set) == 0 {
		return nil, fromMatched
	}
	candidates = make([]string, 0, len(set))
	for c := range set {
		candidates = append(candidates, c)
	}
	sort.Strings(candidates)
	return candidates, fromMatched
}

// anyCandidateChanged reports whether any candidate is among the changed test
// files.
func anyCandidateChanged(candidates []string, changedTests map[string]bool) bool {
	return firstChanged(candidates, changedTests) != ""
}

// firstChanged returns the smallest (sorted) candidate that is a changed test
// file, or "" if none is.
func firstChanged(candidates []string, changedTests map[string]bool) string {
	for _, c := range candidates {
		if changedTests[c] {
			return c
		}
	}
	return ""
}

// existsOnDisk returns the smallest (sorted) candidate that exists as a regular
// file under root, or "" if none does. When root is empty no check is done.
func existsOnDisk(candidates []string, root string) string {
	if root == "" {
		return ""
	}
	for _, c := range candidates {
		full := filepath.Join(root, filepath.FromSlash(c))
		info, err := os.Stat(full)
		if err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

// expand substitutes the placeholder values into an Expect template. When the
// path value is empty, doubled slashes produced by the empty {path} segment are
// collapsed.
func expand(tmpl, pathVal, nameVal, extVal string) string {
	r := strings.ReplaceAll(tmpl, "{path}", pathVal)
	r = strings.ReplaceAll(r, "{name}", nameVal)
	r = strings.ReplaceAll(r, "{ext}", extVal)
	if pathVal == "" {
		for strings.Contains(r, "//") {
			r = strings.ReplaceAll(r, "//", "/")
		}
	}
	return r
}

// placeholder kinds recognized in From templates.
type placeholder int

const (
	phLit placeholder = iota
	phPath
	phName
	phExt
)

// compiledMapping is a From template compiled to a regexp plus the ordered list
// of placeholder capture groups and the Expect templates.
type compiledMapping struct {
	re         *regexp.Regexp
	groupOrder []placeholder
	expect     []string
}

// compileMappings compiles every path mapping's From template once.
func compileMappings(mappings []config.PathMapping) []compiledMapping {
	out := make([]compiledMapping, 0, len(mappings))
	for _, m := range mappings {
		re, order := compileFrom(m.From)
		out = append(out, compiledMapping{re: re, groupOrder: order, expect: m.Expect})
	}
	return out
}

type token struct {
	kind placeholder
	val  string // for phLit
}

// tokenizeFrom splits a From template into literal runs and placeholder tokens.
// Unknown brace sequences are treated as literal text.
func tokenizeFrom(from string) []token {
	tags := []struct {
		tag  string
		kind placeholder
	}{
		{"{path}", phPath},
		{"{name}", phName},
		{"{ext}", phExt},
	}
	var toks []token
	var lit strings.Builder
	flush := func() {
		if lit.Len() > 0 {
			toks = append(toks, token{kind: phLit, val: lit.String()})
			lit.Reset()
		}
	}
	i := 0
	for i < len(from) {
		matched := false
		for _, t := range tags {
			if strings.HasPrefix(from[i:], t.tag) {
				flush()
				toks = append(toks, token{kind: t.kind})
				i += len(t.tag)
				matched = true
				break
			}
		}
		if !matched {
			lit.WriteByte(from[i])
			i++
		}
	}
	flush()
	return toks
}

// compileFrom converts a From template into an anchored regexp and the ordered
// list of placeholder capture groups. It returns (nil, nil) when the template
// is empty or fails to compile.
//
// The `/{path}/` idiom is collapsible: when {path} sits between two literal
// slashes the surrounding slashes and path segments become an optional group so
// that a template like "src/{path}/{name}.{ext}" matches both "src/a.ts"
// (path="") and "src/x/y/a.ts" (path="x/y").
func compileFrom(from string) (*regexp.Regexp, []placeholder) {
	if from == "" {
		return nil, nil
	}
	toks := tokenizeFrom(from)
	if len(toks) == 0 {
		return nil, nil
	}

	frags := make([]string, len(toks))
	trimTrail := make([]bool, len(toks))
	trimLead := make([]bool, len(toks))
	var order []placeholder

	for i, t := range toks {
		switch t.kind {
		case phName:
			frags[i] = `([^/]+)`
			order = append(order, phName)
		case phExt:
			frags[i] = `([^./]+)`
			order = append(order, phExt)
		case phPath:
			prevSlash := i > 0 && toks[i-1].kind == phLit && strings.HasSuffix(toks[i-1].val, "/")
			nextSlash := i+1 < len(toks) && toks[i+1].kind == phLit && strings.HasPrefix(toks[i+1].val, "/")
			if prevSlash && nextSlash {
				frags[i] = `/(?:(.+)/)?`
				trimTrail[i-1] = true
				trimLead[i+1] = true
			} else {
				frags[i] = `(.*)`
			}
			order = append(order, phPath)
		}
	}

	for i, t := range toks {
		if t.kind != phLit {
			continue
		}
		v := t.val
		if trimTrail[i] {
			v = strings.TrimSuffix(v, "/")
		}
		if trimLead[i] {
			v = strings.TrimPrefix(v, "/")
		}
		frags[i] = regexp.QuoteMeta(v)
	}

	re, err := regexp.Compile("^" + strings.Join(frags, "") + "$")
	if err != nil {
		return nil, nil
	}
	return re, order
}

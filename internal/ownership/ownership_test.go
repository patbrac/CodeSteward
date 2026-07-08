package ownership

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// fakeMatcher is a map-based model.OwnerMatcher used so the ownership tests do
// not depend on internal/codeowners. Any path not present is treated as
// unowned (Found=false, Class=missing).
type fakeMatcher struct {
	matches map[string]model.OwnershipMatch
}

func (f fakeMatcher) Match(path string) model.OwnershipMatch {
	if m, ok := f.matches[path]; ok {
		return m
	}
	return model.OwnershipMatch{Found: false, Class: model.MatchMissing}
}

func prod(path string) model.ChangedFile {
	return model.ChangedFile{Path: path, Status: "modified", IsProduction: true}
}

func sensitive(path string) model.ChangedFile {
	return model.ChangedFile{Path: path, Status: "modified", IsSensitive: true}
}

func owned(pattern string, class model.OwnershipMatchClass, owners ...string) model.OwnershipMatch {
	return model.OwnershipMatch{Found: true, Owners: owners, Pattern: pattern, Class: class}
}

// ruleIDs returns the sorted set of distinct rule IDs present in findings.
func ruleIDs(findings []model.Finding) []string {
	seen := map[string]struct{}{}
	for _, f := range findings {
		seen[f.RuleID] = struct{}{}
	}
	var out []string
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func findingsFor(findings []model.Finding, ruleID string) []model.Finding {
	var out []model.Finding
	for _, f := range findings {
		if f.RuleID == ruleID {
			out = append(out, f)
		}
	}
	return out
}

func TestAnalyzeStatesAndRules(t *testing.T) {
	// A demo-like matcher: parser/public/docs are specific, "*" is fallback.
	demo := fakeMatcher{matches: map[string]model.OwnershipMatch{
		"src/parser/tokenize.ts": owned("/src/parser/", model.MatchSpecific, "@parser-maintainers"),
		"src/parser/parse.ts":    owned("/src/parser/", model.MatchSpecific, "@parser-maintainers"),
		"src/public/index.ts":    owned("/src/public/", model.MatchSpecific, "@api-maintainers"),
		"src/runtime/cache.ts":   owned("*", model.MatchFallback, "@maintainers"),
		"lib/core/engine.ts":     owned("/lib/**", model.MatchBroad, "@core-team"),
	}}

	tests := []struct {
		name       string
		files      []model.ChangedFile
		matcher    model.OwnerMatcher
		opts       Options
		wantState  model.OwnershipState
		wantAreas  int
		wantRules  []string // distinct rule IDs, sorted
		wantNoRule []string // rule IDs that must NOT be present
	}{
		{
			name:      "complete specific only",
			files:     []model.ChangedFile{prod("src/parser/tokenize.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipComplete,
			wantAreas: 1,
			wantRules: nil,
		},
		{
			name:      "complete broad counts as covered",
			files:     []model.ChangedFile{prod("lib/core/engine.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipComplete,
			wantAreas: 1,
			wantRules: nil,
		},
		{
			name:       "partial fallback only",
			files:      []model.ChangedFile{prod("src/runtime/cache.ts")},
			matcher:    demo,
			opts:       Options{MaxAreas: 2, Enabled: true},
			wantState:  model.OwnershipPartial,
			wantAreas:  1,
			wantRules:  []string{model.RuleOwnFallbackOnly},
			wantNoRule: []string{model.RuleOwnNoOwner, model.RuleOwnTooManyAreas},
		},
		{
			name:      "missing beats partial",
			files:     []model.ChangedFile{prod("src/runtime/cache.ts"), prod("src/unowned/x.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 5, Enabled: true},
			wantState: model.OwnershipMissing,
			// areas: "*" (cache) + "unowned:src" (x.ts) = 2
			wantAreas: 2,
			wantRules: []string{model.RuleOwnNoOwner, model.RuleOwnFallbackOnly},
		},
		{
			name:      "not evaluated when disabled",
			files:     []model.ChangedFile{prod("src/parser/tokenize.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: false},
			wantState: model.OwnershipNotEvaluated,
			wantAreas: 0,
			wantRules: nil,
		},
		{
			name:      "not evaluated when no relevant files",
			files:     []model.ChangedFile{{Path: "docs/readme.md", IsIgnored: true}, {Path: "tests/a.test.ts", IsTest: true}},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipNotEvaluated,
			wantAreas: 0,
			wantRules: nil,
		},
		{
			name:      "ignored production file is not relevant",
			files:     []model.ChangedFile{{Path: "src/parser/tokenize.ts", IsProduction: true, IsIgnored: true}},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipNotEvaluated,
			wantAreas: 0,
			wantRules: nil,
		},
		{
			name: "too many areas over limit fires 003",
			files: []model.ChangedFile{
				prod("src/parser/parse.ts"), // /src/parser/
				prod("src/public/index.ts"), // /src/public/
				prod("lib/core/engine.ts"),  // /lib/**
			},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipComplete,
			wantAreas: 3,
			wantRules: []string{model.RuleOwnTooManyAreas},
		},
		{
			name: "areas exactly at limit does not fire 003",
			files: []model.ChangedFile{
				prod("src/parser/parse.ts"), // /src/parser/
				prod("src/public/index.ts"), // /src/public/
			},
			matcher:    demo,
			opts:       Options{MaxAreas: 2, Enabled: true},
			wantState:  model.OwnershipComplete,
			wantAreas:  2,
			wantRules:  nil,
			wantNoRule: []string{model.RuleOwnTooManyAreas},
		},
		{
			name:      "same pattern twice counts as one area",
			files:     []model.ChangedFile{prod("src/parser/parse.ts"), prod("src/parser/tokenize.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 1, Enabled: true},
			wantState: model.OwnershipComplete,
			wantAreas: 1,
			wantRules: nil,
		},
		{
			name:      "sensitive file with owner does not fire 004",
			files:     []model.ChangedFile{sensitive("src/runtime/cache.ts")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipNotEvaluated, // no production files
			wantAreas: 0,
			wantRules: nil,
		},
		{
			name:      "sensitive file without owner fires 004",
			files:     []model.ChangedFile{sensitive("ci/config.yml")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipNotEvaluated, // no production files
			wantAreas: 0,
			wantRules: []string{model.RuleOwnSensitiveNoOwn},
		},
		{
			name:      "production and sensitive together",
			files:     []model.ChangedFile{prod("src/parser/parse.ts"), sensitive("ci/config.yml")},
			matcher:   demo,
			opts:      Options{MaxAreas: 2, Enabled: true},
			wantState: model.OwnershipComplete,
			wantAreas: 1,
			wantRules: []string{model.RuleOwnSensitiveNoOwn},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, findings := Analyze(tt.files, tt.matcher, tt.opts)
			if summary.State != tt.wantState {
				t.Errorf("state = %q, want %q", summary.State, tt.wantState)
			}
			if summary.AreasTouched != tt.wantAreas {
				t.Errorf("areas = %d, want %d", summary.AreasTouched, tt.wantAreas)
			}
			if summary.MaxAreas != tt.opts.MaxAreas {
				t.Errorf("maxAreas = %d, want %d", summary.MaxAreas, tt.opts.MaxAreas)
			}
			got := ruleIDs(findings)
			if !reflect.DeepEqual(got, tt.wantRules) {
				t.Errorf("rule IDs = %v, want %v", got, tt.wantRules)
			}
			for _, id := range tt.wantNoRule {
				if len(findingsFor(findings, id)) != 0 {
					t.Errorf("did not expect rule %s to fire", id)
				}
			}
		})
	}
}

func TestAnalyzeNilMatcher(t *testing.T) {
	files := []model.ChangedFile{prod("src/a.ts"), prod("lib/b.ts")}

	summary, findings := Analyze(files, nil, Options{MaxAreas: 5, Enabled: true})
	if summary.State != model.OwnershipMissing {
		t.Fatalf("state = %q, want missing", summary.State)
	}
	own001 := findingsFor(findings, model.RuleOwnNoOwner)
	if len(own001) != 2 {
		t.Fatalf("CS-OWN-001 count = %d, want 2", len(own001))
	}
	// Unowned areas: "unowned:src" + "unowned:lib" = 2 distinct.
	if summary.AreasTouched != 2 {
		t.Errorf("areas = %d, want 2", summary.AreasTouched)
	}

	// Nil matcher but disabled -> not evaluated, no findings.
	summary, findings = Analyze(files, nil, Options{MaxAreas: 5, Enabled: false})
	if summary.State != model.OwnershipNotEvaluated || findings != nil {
		t.Errorf("disabled nil matcher: state=%q findings=%v, want not_evaluated/nil", summary.State, findings)
	}
}

func TestAnalyzeNilMatcherManyAreas(t *testing.T) {
	// Three distinct top-level dirs, all unowned -> 3 "unowned:<top>" areas.
	files := []model.ChangedFile{prod("src/a.ts"), prod("lib/b.ts"), prod("packages/c.ts")}
	summary, findings := Analyze(files, nil, Options{MaxAreas: 2, Enabled: true})
	if summary.AreasTouched != 3 {
		t.Fatalf("areas = %d, want 3", summary.AreasTouched)
	}
	if len(findingsFor(findings, model.RuleOwnTooManyAreas)) != 1 {
		t.Errorf("expected CS-OWN-003 to fire at 3 areas over limit 2")
	}
}

func TestAnalyzeMessageTemplates(t *testing.T) {
	m := fakeMatcher{matches: map[string]model.OwnershipMatch{
		"src/runtime/cache.ts": owned("*", model.MatchFallback, "@maintainers"),
	}}
	files := []model.ChangedFile{
		prod("src/runtime/cache.ts"),
		prod("src/unowned/widget.ts"),
		sensitive("scripts/deploy.sh"),
	}
	_, findings := Analyze(files, m, Options{MaxAreas: 5, Enabled: true})

	want := map[string]struct{ msg, action string }{
		model.RuleOwnNoOwner: {
			msg:    "No owner found for `src/unowned/widget.ts`.",
			action: "Add specific ownership for `src/unowned/widget.ts` or ask a maintainer to route this area.",
		},
		model.RuleOwnFallbackOnly: {
			msg:    "`src/runtime/cache.ts` is covered only by fallback ownership: `* @maintainers`.",
			action: "Add specific ownership for `src/runtime/**` or ask a maintainer to route this area.",
		},
		model.RuleOwnSensitiveNoOwn: {
			msg:    "No owner found for sensitive file `scripts/deploy.sh`.",
			action: "Add specific ownership for `scripts/deploy.sh` or ask a maintainer to route this area.",
		},
	}
	for id, w := range want {
		fs := findingsFor(findings, id)
		if len(fs) != 1 {
			t.Errorf("%s: got %d findings, want 1", id, len(fs))
			continue
		}
		if fs[0].Message != w.msg {
			t.Errorf("%s message =\n  %q\nwant\n  %q", id, fs[0].Message, w.msg)
		}
		if fs[0].Action != w.action {
			t.Errorf("%s action =\n  %q\nwant\n  %q", id, fs[0].Action, w.action)
		}
		if !reflect.DeepEqual(fs[0].Paths, []string{firstPath(id)}) {
			t.Errorf("%s paths = %v", id, fs[0].Paths)
		}
	}
}

func firstPath(ruleID string) string {
	switch ruleID {
	case model.RuleOwnNoOwner:
		return "src/unowned/widget.ts"
	case model.RuleOwnFallbackOnly:
		return "src/runtime/cache.ts"
	case model.RuleOwnSensitiveNoOwn:
		return "scripts/deploy.sh"
	}
	return ""
}

func TestAnalyzeTooManyAreasMessage(t *testing.T) {
	_, findings := Analyze(
		[]model.ChangedFile{prod("src/a.ts"), prod("lib/b.ts"), prod("packages/c.ts")},
		nil,
		Options{MaxAreas: 2, Enabled: true},
	)
	fs := findingsFor(findings, model.RuleOwnTooManyAreas)
	if len(fs) != 1 {
		t.Fatalf("CS-OWN-003 count = %d, want 1", len(fs))
	}
	wantMsg := "This change touches 3 ownership areas, above the configured limit of 2."
	if fs[0].Message != wantMsg {
		t.Errorf("message = %q, want %q", fs[0].Message, wantMsg)
	}
	if fs[0].Severity != model.SeverityWarning {
		t.Errorf("severity = %q, want warning", fs[0].Severity)
	}
	if len(fs[0].Paths) != 0 {
		t.Errorf("CS-OWN-003 should carry no paths, got %v", fs[0].Paths)
	}
}

func TestAnalyzeFindingsSortedByPathWithinRule(t *testing.T) {
	files := []model.ChangedFile{
		prod("src/zeta.ts"),
		prod("src/alpha.ts"),
		prod("src/mid.ts"),
	}
	_, findings := Analyze(files, nil, Options{MaxAreas: 10, Enabled: true})
	own001 := findingsFor(findings, model.RuleOwnNoOwner)
	var paths []string
	for _, f := range own001 {
		paths = append(paths, f.Paths[0])
	}
	want := []string{"src/alpha.ts", "src/mid.ts", "src/zeta.ts"}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("CS-OWN-001 paths = %v, want %v", paths, want)
	}
}

func TestAnalyzeSummaryFiles(t *testing.T) {
	m := fakeMatcher{matches: map[string]model.OwnershipMatch{
		"src/parser/parse.ts": owned("/src/parser/", model.MatchSpecific, "@parser"),
	}}
	files := []model.ChangedFile{prod("src/parser/parse.ts"), prod("src/orphan.ts")}
	summary, _ := Analyze(files, m, Options{MaxAreas: 5, Enabled: true})
	if len(summary.Files) != 2 {
		t.Fatalf("summary.Files len = %d, want 2", len(summary.Files))
	}
	// Sorted by path: src/orphan.ts, src/parser/parse.ts
	if summary.Files[0].Path != "src/orphan.ts" || summary.Files[1].Path != "src/parser/parse.ts" {
		t.Fatalf("summary.Files order = %v", summary.Files)
	}
	if summary.Files[0].Class != model.MatchMissing {
		t.Errorf("orphan class = %q, want missing", summary.Files[0].Class)
	}
	if summary.Files[1].Class != model.MatchSpecific || summary.Files[1].Pattern != "/src/parser/" {
		t.Errorf("parse.ts entry = %+v", summary.Files[1])
	}
}

// --- Audit tests ---

func gitInit(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
}

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func auditMatcher() fakeMatcher {
	return fakeMatcher{matches: map[string]model.OwnershipMatch{
		"src/parser/tokenize.ts": owned("/src/parser/", model.MatchSpecific, "@parser"),
		"src/public/index.ts":    owned("/src/**", model.MatchBroad, "@core"),
		"src/runtime/cache.ts":   owned("*", model.MatchFallback, "@maintainers"),
		// src/orphan.ts intentionally absent -> unowned
	}}
}

func assertAudit(t *testing.T, res *AuditResult) {
	t.Helper()
	if res.Total != 4 {
		t.Errorf("Total = %d, want 4", res.Total)
	}
	if res.Specific != 1 {
		t.Errorf("Specific = %d, want 1", res.Specific)
	}
	if res.Broad != 1 {
		t.Errorf("Broad = %d, want 1", res.Broad)
	}
	if res.Fallback != 1 {
		t.Errorf("Fallback = %d, want 1", res.Fallback)
	}
	if res.Unowned != 1 {
		t.Errorf("Unowned = %d, want 1", res.Unowned)
	}
	if res.Total != res.Specific+res.Broad+res.Fallback+res.Unowned {
		t.Errorf("counts do not sum to Total")
	}
	// Entries only include production files, sorted by path, docs excluded.
	var paths []string
	for _, e := range res.Entries {
		paths = append(paths, e.Path)
	}
	want := []string{
		"src/orphan.ts",
		"src/parser/tokenize.ts",
		"src/public/index.ts",
		"src/runtime/cache.ts",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("entry paths = %v, want %v", paths, want)
	}
	// Unowned entry carries no owners and missing class.
	for _, e := range res.Entries {
		if e.Path == "src/orphan.ts" {
			if e.Class != model.MatchMissing || len(e.Owners) != 0 {
				t.Errorf("orphan entry = %+v", e)
			}
		}
	}
}

// productionLayout writes a standard set of files (some production, one ignored
// docs file, one root file) used by both Audit code-path tests.
func productionLayout(t *testing.T, root string) {
	writeFile(t, root, "src/parser/tokenize.ts", "x")
	writeFile(t, root, "src/public/index.ts", "x")
	writeFile(t, root, "src/runtime/cache.ts", "x")
	writeFile(t, root, "src/orphan.ts", "x")
	writeFile(t, root, "docs/usage.md", "x") // excluded by ignorePaths
	writeFile(t, root, "README.md", "x")     // not production
	writeFile(t, root, "CODEOWNERS", "x")    // not production
}

func TestAuditGitLsFiles(t *testing.T) {
	root := t.TempDir()
	gitInit(t, root)
	productionLayout(t, root)

	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	res, err := Audit(root, auditMatcher(), []string{"src/**"}, []string{"docs/**"})
	if err != nil {
		t.Fatal(err)
	}
	assertAudit(t, res)
}

func TestAuditWalkFallback(t *testing.T) {
	// Plain directory: no git repo -> git ls-files fails -> WalkDir fallback.
	root := t.TempDir()
	productionLayout(t, root)
	// A stray .git directory must be skipped by the walk fallback.
	writeFile(t, root, ".git/objects/deadbeef", "should be skipped")
	writeFile(t, root, "src/.git/nested", "should be skipped")

	res, err := Audit(root, auditMatcher(), []string{"src/**"}, []string{"docs/**"})
	if err != nil {
		t.Fatal(err)
	}
	assertAudit(t, res)
	for _, e := range res.Entries {
		if filepath.Base(filepath.Dir(e.Path)) == ".git" || e.Path == ".git/objects/deadbeef" {
			t.Errorf(".git content leaked into audit: %s", e.Path)
		}
	}
}

func TestAuditNilMatcherAllUnowned(t *testing.T) {
	root := t.TempDir()
	productionLayout(t, root)

	res, err := Audit(root, nil, []string{"src/**"}, []string{"docs/**"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 4 || res.Unowned != 4 {
		t.Fatalf("Total=%d Unowned=%d, want 4/4", res.Total, res.Unowned)
	}
	if res.Specific+res.Broad+res.Fallback != 0 {
		t.Errorf("expected no owned entries with nil matcher")
	}
}

func TestAuditEmptyProduction(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "README.md", "x")
	writeFile(t, root, "docs/usage.md", "x")

	res, err := Audit(root, auditMatcher(), []string{"src/**"}, []string{"docs/**"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Total != 0 || len(res.Entries) != 0 {
		t.Errorf("expected empty audit, got %+v", res)
	}
}

package rules

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// ---- helpers ----------------------------------------------------------------

func ruleIDs(findings []model.Finding) []string {
	if len(findings) == 0 {
		return nil
	}
	out := make([]string, 0, len(findings))
	for _, f := range findings {
		out = append(out, f.RuleID)
	}
	sort.Strings(out)
	return out
}

func findingFor(findings []model.Finding, ruleID string) (model.Finding, bool) {
	for _, f := range findings {
		if f.RuleID == ruleID {
			return f, true
		}
	}
	return model.Finding{}, false
}

func countRule(findings []model.Finding, ruleID string) int {
	n := 0
	for _, f := range findings {
		if f.RuleID == ruleID {
			n++
		}
	}
	return n
}

// ---- Scope ------------------------------------------------------------------

func TestScopeFindings(t *testing.T) {
	defaults := ScopeOptions{MaxFiles: 12, MaxLines: 500}

	tests := []struct {
		name  string
		files []model.ChangedFile
		opts  ScopeOptions
		want  []string
	}{
		{
			name:  "no files",
			files: nil,
			opts:  defaults,
			want:  nil,
		},
		{
			name: "under all limits",
			files: []model.ChangedFile{
				{Path: "src/a.ts", Additions: 10, Deletions: 5, IsProduction: true},
			},
			opts: defaults,
			want: nil,
		},
		{
			name: "too many files fires CS-SCP-001",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "src/b.ts", IsProduction: true},
				{Path: "src/c.ts", IsProduction: true},
			},
			opts: ScopeOptions{MaxFiles: 2, MaxLines: 500},
			want: []string{model.RuleScpTooManyFiles},
		},
		{
			name: "ignored files excluded from file count",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "src/b.ts", IsProduction: true},
				{Path: "docs/guide.md", IsIgnored: true},
			},
			opts: ScopeOptions{MaxFiles: 2, MaxLines: 500},
			want: nil,
		},
		{
			name: "too many lines fires CS-SCP-002",
			files: []model.ChangedFile{
				{Path: "src/a.ts", Additions: 400, Deletions: 200, IsProduction: true},
			},
			opts: ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: []string{model.RuleScpTooManyLines},
		},
		{
			name: "ignored file lines excluded from line count",
			files: []model.ChangedFile{
				{Path: "src/a.ts", Additions: 100, IsProduction: true},
				{Path: "docs/huge.md", Additions: 1000, IsIgnored: true},
			},
			opts: ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: nil,
		},
		{
			name: "production plus manifest fires CS-SCP-003",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "package.json", IsSensitive: true},
			},
			opts: defaults,
			want: []string{model.RuleScpSrcPlusDeps},
		},
		{
			name: "production plus lockfile fires CS-SCP-003",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "yarn.lock", IsSensitive: true},
			},
			opts: defaults,
			want: []string{model.RuleScpSrcPlusDeps},
		},
		{
			name: "manifest without production does not fire CS-SCP-003",
			files: []model.ChangedFile{
				{Path: "package.json", IsSensitive: true},
			},
			opts: defaults,
			want: nil,
		},
		{
			name: "production without deps does not fire CS-SCP-003",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
			},
			opts: defaults,
			want: nil,
		},
		{
			name: "mixed concerns fires CS-SCP-004 (docs dir + CI)",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "docs/usage.md", IsIgnored: true},
				{Path: ".github/workflows/ci.yml", IsSensitive: true},
			},
			opts: defaults,
			want: []string{model.RuleScpMixedConcerns},
		},
		{
			name: "mixed concerns fires via markdown docs and root dotfile",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "README.md"},
				{Path: ".eslintrc.json"},
			},
			opts: defaults,
			want: []string{model.RuleScpMixedConcerns},
		},
		{
			name: "mixed concerns needs all three (missing config)",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: "docs/usage.md", IsIgnored: true},
			},
			opts: defaults,
			want: nil,
		},
		{
			name: "mixed concerns needs all three (missing docs)",
			files: []model.ChangedFile{
				{Path: "src/a.ts", IsProduction: true},
				{Path: ".github/workflows/ci.yml", IsSensitive: true},
			},
			opts: defaults,
			want: nil,
		},
		{
			name: "too many areas fires CS-SCP-005",
			files: []model.ChangedFile{
				{Path: "a/f.ts"},
				{Path: "b/f.ts"},
				{Path: "c/f.ts"},
				{Path: "d/f.ts"},
				{Path: "e/f.ts"},
			},
			opts: defaults,
			want: []string{model.RuleScpTooManyAreas},
		},
		{
			name: "exactly four areas does not fire CS-SCP-005",
			files: []model.ChangedFile{
				{Path: "a/f.ts"},
				{Path: "b/f.ts"},
				{Path: "c/f.ts"},
				{Path: "d/f.ts"},
			},
			opts: defaults,
			want: nil,
		},
		{
			name:  "reference scenario 3 scope findings",
			files: scenario3Files(),
			opts:  defaults,
			want:  []string{model.RuleScpMixedConcerns, model.RuleScpSrcPlusDeps},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, findings := Scope(tc.files, tc.opts)
			got := ruleIDs(findings)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rule IDs = %v, want %v", got, want)
			}
		})
	}
}

func TestScopeSummary(t *testing.T) {
	tests := []struct {
		name  string
		files []model.ChangedFile
		opts  ScopeOptions
		want  model.ScopeSummary
	}{
		{
			name:  "empty",
			files: nil,
			opts:  ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: model.ScopeSummary{
				MaxFilesChanged: 12,
				MaxLinesChanged: 500,
			},
		},
		{
			name: "ignored excluded from counts but present in areas",
			files: []model.ChangedFile{
				{Path: "src/a.ts", Additions: 10, Deletions: 5, IsProduction: true},
				{Path: "docs/x.md", Additions: 100, Deletions: 100, IsIgnored: true},
			},
			opts: ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: model.ScopeSummary{
				FilesChanged:    1,
				LinesAdded:      10,
				LinesDeleted:    5,
				TopLevelAreas:   []string{"docs", "src"},
				MaxFilesChanged: 12,
				MaxLinesChanged: 500,
			},
		},
		{
			name:  "reference scenario 3 summary",
			files: scenario3Files(),
			opts:  ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: model.ScopeSummary{
				FilesChanged:    4,
				LinesAdded:      40,
				LinesDeleted:    0,
				TopLevelAreas:   []string{"(root)", ".github", "docs", "src"},
				MaxFilesChanged: 12,
				MaxLinesChanged: 500,
			},
		},
		{
			name: "limits exceeded flags",
			files: []model.ChangedFile{
				{Path: "src/a.ts", Additions: 600, IsProduction: true},
				{Path: "src/b.ts", IsProduction: true},
				{Path: "src/c.ts", IsProduction: true},
			},
			opts: ScopeOptions{MaxFiles: 2, MaxLines: 500},
			want: model.ScopeSummary{
				FilesChanged:     3,
				LinesAdded:       600,
				LinesDeleted:     0,
				TopLevelAreas:    []string{"src"},
				MaxFilesChanged:  2,
				MaxLinesChanged:  500,
				ExceedsFileLimit: true,
				ExceedsLineLimit: true,
			},
		},
		{
			name: "root area naming",
			files: []model.ChangedFile{
				{Path: "Makefile"},
			},
			opts: ScopeOptions{MaxFiles: 12, MaxLines: 500},
			want: model.ScopeSummary{
				FilesChanged:    1,
				TopLevelAreas:   []string{"(root)"},
				MaxFilesChanged: 12,
				MaxLinesChanged: 500,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := Scope(tc.files, tc.opts)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("summary =\n  %#v\nwant\n  %#v", got, tc.want)
			}
		})
	}
}

// scenario3Files reproduces the CONTRACTS §7.2 reference scenario 3 change set
// with the classifications diff.Classify would produce under the default
// config.
func scenario3Files() []model.ChangedFile {
	return []model.ChangedFile{
		{Path: "src/parser/parse.ts", Additions: 10, IsProduction: true},
		{Path: "src/runtime/cache.ts", Additions: 10, IsProduction: true},
		{Path: "docs/usage.md", Additions: 10, IsIgnored: true},
		{Path: "package.json", Additions: 10, IsSensitive: true},
		{Path: ".github/workflows/release.yml", Additions: 10, IsSensitive: true},
	}
}

// ---- Description ------------------------------------------------------------

func TestDescription(t *testing.T) {
	tests := []struct {
		name        string
		opts        DescriptionOptions
		wantSummary model.DescriptionSummary
		wantRules   []string
	}{
		{
			name:        "not evaluated emits nothing",
			opts:        DescriptionOptions{Text: "anything at all", Evaluated: false, WarnIfEmpty: true, MinLength: 80, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Evaluated: false},
			wantRules:   nil,
		},
		{
			name:        "empty warns only CS-DSC-001",
			opts:        DescriptionOptions{Text: "", Evaluated: true, WarnIfEmpty: true, MinLength: 80, RequiredSections: []string{"Summary"}, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Evaluated: true},
			wantRules:   []string{model.RuleDscEmpty},
		},
		{
			name:        "whitespace only treated as empty",
			opts:        DescriptionOptions{Text: "   \n\t  ", Evaluated: true, WarnIfEmpty: true, MinLength: 80},
			wantSummary: model.DescriptionSummary{Evaluated: true},
			wantRules:   []string{model.RuleDscEmpty},
		},
		{
			name:        "empty without warn emits nothing",
			opts:        DescriptionOptions{Text: "", Evaluated: true, WarnIfEmpty: false, MinLength: 80, RequiredSections: []string{"Summary"}, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "too short fires CS-DSC-002",
			opts:        DescriptionOptions{Text: "short text", Evaluated: true, WarnIfEmpty: true, MinLength: 80},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 10, Evaluated: true},
			wantRules:   []string{model.RuleDscTooShort},
		},
		{
			name:        "length is rune count not byte count",
			opts:        DescriptionOptions{Text: "café", Evaluated: true, WarnIfEmpty: true, MinLength: 5},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 4, Evaluated: true},
			wantRules:   []string{model.RuleDscTooShort},
		},
		{
			name:        "meets minimum length no finding",
			opts:        DescriptionOptions{Text: "café", Evaluated: true, WarnIfEmpty: true, MinLength: 4},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 4, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "min length zero disables check",
			opts:        DescriptionOptions{Text: "x", Evaluated: true, WarnIfEmpty: true, MinLength: 0},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 1, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "required section present via heading",
			opts:        DescriptionOptions{Text: "## Summary\n\nlots of detail here", Evaluated: true, MinLength: 0, RequiredSections: []string{"Summary"}},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 31, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "required section present via bold line",
			opts:        DescriptionOptions{Text: "**Test Plan**\n\nran the suite", Evaluated: true, MinLength: 0, RequiredSections: []string{"test plan"}},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 28, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "required section case insensitive",
			opts:        DescriptionOptions{Text: "# MOTIVATION here", Evaluated: true, MinLength: 0, RequiredSections: []string{"Motivation"}},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 17, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "missing section fires CS-DSC-003",
			opts:        DescriptionOptions{Text: "Just some prose with no headings.", Evaluated: true, MinLength: 0, RequiredSections: []string{"Summary"}},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 33, Evaluated: true},
			wantRules:   []string{model.RuleDscMissingSection},
		},
		{
			name:        "plain-text mention without heading still missing",
			opts:        DescriptionOptions{Text: "This describes the summary in a paragraph.", Evaluated: true, MinLength: 0, RequiredSections: []string{"summary"}},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 42, Evaluated: true},
			wantRules:   []string{model.RuleDscMissingSection},
		},
		{
			name:        "linked issue present via hash reference",
			opts:        DescriptionOptions{Text: "Fixes #123 in the parser.", Evaluated: true, MinLength: 0, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 25, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "linked issue present via issues url",
			opts:        DescriptionOptions{Text: "See https://github.com/o/r/issues/45 for context.", Evaluated: true, MinLength: 0, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 49, Evaluated: true},
			wantRules:   nil,
		},
		{
			name:        "linked issue missing fires CS-DSC-004",
			opts:        DescriptionOptions{Text: "No references here at all.", Evaluated: true, MinLength: 0, RequireLinkedIssue: true},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 26, Evaluated: true},
			wantRules:   []string{model.RuleDscNoLinkedIssue},
		},
		{
			name: "combination of short missing-section and no-issue",
			opts: DescriptionOptions{
				Text: "tiny", Evaluated: true, WarnIfEmpty: true, MinLength: 80,
				RequiredSections: []string{"Summary", "Test Plan"}, RequireLinkedIssue: true,
			},
			wantSummary: model.DescriptionSummary{Provided: true, Length: 4, Evaluated: true},
			wantRules: []string{
				model.RuleDscTooShort,
				model.RuleDscMissingSection,
				model.RuleDscMissingSection,
				model.RuleDscNoLinkedIssue,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary, findings := Description(tc.opts)
			if summary != tc.wantSummary {
				t.Fatalf("summary = %#v, want %#v", summary, tc.wantSummary)
			}
			got := ruleIDs(findings)
			want := append([]string(nil), tc.wantRules...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rule IDs = %v, want %v", got, want)
			}
		})
	}
}

func TestDescriptionMultipleMissingSections(t *testing.T) {
	_, findings := Description(DescriptionOptions{
		Text:             "no headings whatsoever",
		Evaluated:        true,
		MinLength:        0,
		RequiredSections: []string{"Summary", "Motivation", "Test Plan"},
	})
	if got := countRule(findings, model.RuleDscMissingSection); got != 3 {
		t.Fatalf("missing-section findings = %d, want 3", got)
	}
	// One finding per section, each naming its section.
	names := map[string]bool{}
	for _, f := range findings {
		names[f.Message] = true
	}
	for _, want := range []string{
		"The PR description is missing the `Summary` section.",
		"The PR description is missing the `Motivation` section.",
		"The PR description is missing the `Test Plan` section.",
	} {
		if !names[want] {
			t.Errorf("missing expected message %q", want)
		}
	}
}

func TestDescriptionExactTemplates(t *testing.T) {
	_, findings := Description(DescriptionOptions{Text: "  ", Evaluated: true, WarnIfEmpty: true})
	f, ok := findingFor(findings, model.RuleDscEmpty)
	if !ok {
		t.Fatal("expected CS-DSC-001")
	}
	if f.Message != "The PR description is empty." {
		t.Errorf("CS-DSC-001 message = %q", f.Message)
	}
	if f.Action != "Add a short description explaining the motivation and test plan." {
		t.Errorf("CS-DSC-001 action = %q", f.Action)
	}
	if f.Severity != model.SeverityWarning {
		t.Errorf("CS-DSC-001 severity = %q", f.Severity)
	}
}

func TestDescriptionFixtures(t *testing.T) {
	full := readFixture(t, "description_full.md")
	_, findings := Description(DescriptionOptions{
		Text:               full,
		Evaluated:          true,
		WarnIfEmpty:        true,
		MinLength:          80,
		RequiredSections:   []string{"Summary", "Motivation", "Test plan"},
		RequireLinkedIssue: true,
	})
	if len(findings) != 0 {
		t.Fatalf("well-formed description produced findings: %v", ruleIDs(findings))
	}

	sparse := readFixture(t, "description_sparse.md")
	_, findings = Description(DescriptionOptions{
		Text:               sparse,
		Evaluated:          true,
		WarnIfEmpty:        true,
		MinLength:          80,
		RequiredSections:   []string{"Summary", "Test plan"},
		RequireLinkedIssue: true,
	})
	got := ruleIDs(findings)
	want := []string{
		model.RuleDscMissingSection,
		model.RuleDscMissingSection,
		model.RuleDscNoLinkedIssue,
		model.RuleDscTooShort,
	}
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sparse fixture rule IDs = %v, want %v", got, want)
	}
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// ---- Sensitive --------------------------------------------------------------

func TestSensitive(t *testing.T) {
	tests := []struct {
		name  string
		files []model.ChangedFile
		want  []string
	}{
		{
			name:  "no files",
			files: nil,
			want:  nil,
		},
		{
			name:  "plain source is not sensitive",
			files: []model.ChangedFile{{Path: "src/a.ts", IsProduction: true}},
			want:  nil,
		},
		{
			name:  "builtin lockfile fires regardless of IsSensitive flag",
			files: []model.ChangedFile{{Path: "go.sum"}},
			want:  []string{model.RuleSnsLockfile},
		},
		{
			name:  "nested lockfile by basename",
			files: []model.ChangedFile{{Path: "frontend/yarn.lock"}},
			want:  []string{model.RuleSnsLockfile},
		},
		{
			name:  "ci workflow",
			files: []model.ChangedFile{{Path: ".github/workflows/ci.yml"}},
			want:  []string{model.RuleSnsCIWorkflow},
		},
		{
			name:  "gitlab ci file",
			files: []model.ChangedFile{{Path: ".gitlab-ci.yml"}},
			want:  []string{model.RuleSnsCIWorkflow},
		},
		{
			name:  "release script",
			files: []model.ChangedFile{{Path: "scripts/release/publish.sh"}},
			want:  []string{model.RuleSnsCIWorkflow},
		},
		{
			name:  "manifest",
			files: []model.ChangedFile{{Path: "go.mod"}},
			want:  []string{model.RuleSnsManifest},
		},
		{
			name:  "configured other requires IsSensitive",
			files: []model.ChangedFile{{Path: "config/secrets.env", IsSensitive: true}},
			want:  []string{model.RuleSnsConfigured},
		},
		{
			name:  "non-builtin without IsSensitive fires nothing",
			files: []model.ChangedFile{{Path: "config/secrets.env"}},
			want:  nil,
		},
		{
			name:  "lockfile priority over configured-other",
			files: []model.ChangedFile{{Path: "package-lock.json", IsSensitive: true}},
			want:  []string{model.RuleSnsLockfile},
		},
		{
			name:  "manifest priority over configured-other",
			files: []model.ChangedFile{{Path: "package.json", IsSensitive: true}},
			want:  []string{model.RuleSnsManifest},
		},
		{
			name:  "ci priority over configured-other",
			files: []model.ChangedFile{{Path: ".github/workflows/ci.yml", IsSensitive: true}},
			want:  []string{model.RuleSnsCIWorkflow},
		},
		{
			name: "all four categories each fire once",
			files: []model.ChangedFile{
				{Path: "go.sum"},
				{Path: ".gitlab-ci.yml"},
				{Path: "go.mod"},
				{Path: "secrets.txt", IsSensitive: true},
			},
			want: []string{
				model.RuleSnsLockfile,
				model.RuleSnsCIWorkflow,
				model.RuleSnsManifest,
				model.RuleSnsConfigured,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			findings := Sensitive(tc.files)
			got := ruleIDs(findings)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("rule IDs = %v, want %v", got, want)
			}
			// Each fired rule must appear exactly once (aggregated).
			for _, id := range want {
				if c := countRule(findings, id); c != 1 {
					t.Errorf("rule %s appeared %d times, want 1", id, c)
				}
			}
		})
	}
}

func TestSensitiveAggregatesAndSorts(t *testing.T) {
	files := []model.ChangedFile{
		{Path: "yarn.lock"},
		{Path: "package-lock.json"},
		{Path: "go.sum"},
	}
	findings := Sensitive(files)
	if len(findings) != 1 {
		t.Fatalf("expected 1 aggregated lockfile finding, got %d", len(findings))
	}
	f := findings[0]
	wantPaths := []string{"go.sum", "package-lock.json", "yarn.lock"}
	if !reflect.DeepEqual(f.Paths, wantPaths) {
		t.Errorf("paths = %v, want %v", f.Paths, wantPaths)
	}
	wantMsg := "`go.sum`, `package-lock.json`, `yarn.lock` changed (lockfile)."
	if f.Message != wantMsg {
		t.Errorf("message = %q, want %q", f.Message, wantMsg)
	}
}

func TestSensitiveExactTemplate(t *testing.T) {
	findings := Sensitive([]model.ChangedFile{{Path: ".github/workflows/release.yml"}})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.RuleID != model.RuleSnsCIWorkflow {
		t.Fatalf("rule = %q", f.RuleID)
	}
	if f.Severity != model.SeverityWarning {
		t.Errorf("severity = %q", f.Severity)
	}
	if want := "`.github/workflows/release.yml` changed (CI workflow)."; f.Message != want {
		t.Errorf("message = %q, want %q", f.Message, want)
	}
	if want := "Call out the CI workflow change in the description so maintainers can verify it."; f.Action != want {
		t.Errorf("action = %q, want %q", f.Action, want)
	}
	if want := []string{".github/workflows/release.yml"}; !reflect.DeepEqual(f.Paths, want) {
		t.Errorf("paths = %v, want %v", f.Paths, want)
	}
}

func TestScpSrcPlusDepsTemplate(t *testing.T) {
	_, findings := Scope([]model.ChangedFile{
		{Path: "src/a.ts", IsProduction: true},
		{Path: "package.json", IsSensitive: true},
	}, ScopeOptions{MaxFiles: 12, MaxLines: 500})
	f, ok := findingFor(findings, model.RuleScpSrcPlusDeps)
	if !ok {
		t.Fatal("expected CS-SCP-003")
	}
	if want := "Dependency files changed alongside production source files."; f.Message != want {
		t.Errorf("message = %q, want %q", f.Message, want)
	}
	if want := "Consider splitting dependency changes from runtime changes."; f.Action != want {
		t.Errorf("action = %q, want %q", f.Action, want)
	}
}

// ---- Determinism ------------------------------------------------------------

func TestDeterminism(t *testing.T) {
	files := scenario3Files()

	s1, f1 := Scope(files, ScopeOptions{MaxFiles: 12, MaxLines: 500})
	s2, f2 := Scope(files, ScopeOptions{MaxFiles: 12, MaxLines: 500})
	if !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(f1, f2) {
		t.Error("Scope is not deterministic")
	}

	sens := []model.ChangedFile{
		{Path: "yarn.lock"}, {Path: "go.sum"}, {Path: "package.json"},
		{Path: ".github/workflows/ci.yml"}, {Path: "x.conf", IsSensitive: true},
	}
	if !reflect.DeepEqual(Sensitive(sens), Sensitive(sens)) {
		t.Error("Sensitive is not deterministic")
	}

	opts := DescriptionOptions{Text: "short", Evaluated: true, WarnIfEmpty: true, MinLength: 80, RequiredSections: []string{"A", "B"}, RequireLinkedIssue: true}
	d1, df1 := Description(opts)
	d2, df2 := Description(opts)
	if d1 != d2 || !reflect.DeepEqual(df1, df2) {
		t.Error("Description is not deterministic")
	}
}

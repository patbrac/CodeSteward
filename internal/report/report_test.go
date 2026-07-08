package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// tool is the fixed ToolInfo used across fixtures.
var tool = model.ToolInfo{Name: "codesteward", Version: "0.1.0-dev"}

// readyReport: status ready, no findings.
func readyReport() *model.Report {
	return &model.Report{
		SchemaVersion: "0.1",
		Tool:          tool,
		Base:          "main",
		Head:          "HEAD",
		CommentOnly:   true,
		Status:        model.StatusReady,
		Score:         100,
		ReviewBurden:  model.BurdenLow,
		Ownership:     model.OwnershipSummary{State: model.OwnershipComplete, AreasTouched: 1, MaxAreas: 2},
		Tests:         model.TestsSummary{State: model.TestsMatchingChanged},
		Scope: model.ScopeSummary{
			FilesChanged: 2, LinesAdded: 40, LinesDeleted: 3,
			TopLevelAreas:   []string{"src", "tests"},
			MaxFilesChanged: 12, MaxLinesChanged: 500,
		},
		Description: model.DescriptionSummary{Provided: true, Length: 120, Evaluated: true},
	}
}

// needsActionReport mirrors reference scenario 2 (CONTRACTS §7.2): a single
// unowned-by-fallback production file with no matching test and an empty
// description. Findings are supplied in canonical order (§7.6).
func needsActionReport() *model.Report {
	findings := []model.Finding{
		{
			RuleID:   model.RuleTstMissing, // CS-TST-001, action_required
			Severity: model.SeverityActionRequired,
			Message:  "`src/runtime/cache.ts` changed, but no matching test file was changed.",
			Action:   "Add or update matching tests for `src/runtime/cache.ts`.",
			Paths:    []string{"src/runtime/cache.ts"},
		},
		{
			RuleID:   model.RuleDscEmpty, // CS-DSC-001, warning
			Severity: model.SeverityWarning,
			Message:  "The PR description is empty.",
			Action:   "Add a short description explaining the motivation and test plan.",
		},
		{
			RuleID:   model.RuleOwnFallbackOnly, // CS-OWN-002, warning
			Severity: model.SeverityWarning,
			Message:  "`src/runtime/cache.ts` is covered only by fallback ownership: `* @maintainers`.",
			Action:   "Add specific ownership for `src/**` or ask a maintainer to route this area.",
			Paths:    []string{"src/runtime/cache.ts"},
		},
	}
	return &model.Report{
		SchemaVersion: "0.1",
		Tool:          tool,
		Base:          "main",
		Head:          "HEAD",
		CommentOnly:   true,
		Status:        model.StatusNeedsAction,
		Score:         60,
		ReviewBurden:  model.BurdenMedium,
		Ownership: model.OwnershipSummary{
			State: model.OwnershipPartial, AreasTouched: 1, MaxAreas: 2,
			Files: []model.FileOwnership{{
				Path: "src/runtime/cache.ts", Owners: []string{"@maintainers"},
				Pattern: "*", Class: model.MatchFallback,
			}},
		},
		Tests: model.TestsSummary{
			State: model.TestsMissingMatching,
			Files: []model.FileTestExpectation{{
				Path:  "src/runtime/cache.ts",
				State: model.TestsMissingMatching,
				Candidates: []string{
					"tests/runtime/cache.test.ts",
				},
			}},
		},
		Scope: model.ScopeSummary{
			FilesChanged: 1, LinesAdded: 30, LinesDeleted: 4,
			TopLevelAreas:   []string{"src"},
			MaxFilesChanged: 12, MaxLinesChanged: 500,
		},
		Description: model.DescriptionSummary{Provided: false, Length: 0, Evaluated: true},
		Findings:    findings,
		ExitCriteria: []model.ExitCriterion{{
			RuleID:      model.RuleTstMissing,
			Description: "Add or update matching tests for `src/runtime/cache.ts`.",
		}},
	}
}

// highBurdenReport carries seven action_required findings with distinct
// actions, proving the 5-item cap on visible action items while every finding
// message still appears in the details block. Findings are in canonical order.
func highBurdenReport() *model.Report {
	prodFiles := []string{"src/a.ts", "src/b.ts", "src/c.ts", "src/d.ts"}
	findings := []model.Finding{
		{
			RuleID: model.RuleOwnNoOwner, Severity: model.SeverityActionRequired,
			Message: "No owner found for `src/a.ts`.",
			Action:  "Add specific ownership for `src/a.ts` or ask a maintainer to route this area.",
			Paths:   []string{"src/a.ts"},
		},
		{
			RuleID: model.RuleOwnNoOwner, Severity: model.SeverityActionRequired,
			Message: "No owner found for `src/b.ts`.",
			Action:  "Add specific ownership for `src/b.ts` or ask a maintainer to route this area.",
			Paths:   []string{"src/b.ts"},
		},
		{
			RuleID: model.RuleOwnSensitiveNoOwn, Severity: model.SeverityActionRequired,
			Message: "No owner found for sensitive file `config/prod.env`.",
			Action:  "Add specific ownership for `config/prod.env` or ask a maintainer to route this area.",
			Paths:   []string{"config/prod.env"},
		},
	}
	for _, p := range prodFiles {
		findings = append(findings, model.Finding{
			RuleID: model.RuleTstMissing, Severity: model.SeverityActionRequired,
			Message: "`" + p + "` changed, but no matching test file was changed.",
			Action:  "Add or update matching tests for `" + p + "`.",
			Paths:   []string{p},
		})
	}
	return &model.Report{
		SchemaVersion: "0.1",
		Tool:          tool,
		Base:          "main",
		Head:          "HEAD",
		CommentOnly:   true,
		Status:        model.StatusHighBurden,
		Score:         0,
		ReviewBurden:  model.BurdenHigh,
		Ownership:     model.OwnershipSummary{State: model.OwnershipMissing, AreasTouched: 3, MaxAreas: 2},
		Tests:         model.TestsSummary{State: model.TestsMissingMatching},
		Scope: model.ScopeSummary{
			FilesChanged: 5, LinesAdded: 300, LinesDeleted: 50,
			TopLevelAreas:   []string{"config", "src"},
			MaxFilesChanged: 12, MaxLinesChanged: 500,
		},
		Description: model.DescriptionSummary{Provided: true, Length: 90, Evaluated: true},
		Findings:    findings,
	}
}

// notEvaluatedReport exercises every not_evaluated display state with no
// findings; the two static header stat labels must still render.
func notEvaluatedReport() *model.Report {
	return &model.Report{
		SchemaVersion: "0.1",
		Tool:          tool,
		Base:          "main",
		Head:          "HEAD",
		CommentOnly:   true,
		Status:        model.StatusReady,
		Score:         100,
		ReviewBurden:  model.BurdenLow,
		Ownership:     model.OwnershipSummary{State: model.OwnershipNotEvaluated},
		Tests:         model.TestsSummary{State: model.TestsNotEvaluated},
		Scope: model.ScopeSummary{
			FilesChanged: 1, LinesAdded: 5, LinesDeleted: 0,
			TopLevelAreas:   []string{"docs"},
			MaxFilesChanged: 12, MaxLinesChanged: 500,
		},
		Description: model.DescriptionSummary{Evaluated: false},
	}
}

func TestRenderMarkdownGolden(t *testing.T) {
	cases := []struct {
		name   string
		report func() *model.Report
		opts   MarkdownOptions
	}{
		{"ready-no-findings", readyReport, MarkdownOptions{}},
		{"needs-action", needsActionReport, MarkdownOptions{}},
		{"high-burden-cap", highBurdenReport, MarkdownOptions{}},
		{"not-evaluated", notEvaluatedReport, MarkdownOptions{}},
		{"show-score", needsActionReport, MarkdownOptions{ShowScore: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderMarkdown(tc.report(), tc.opts)
			golden := filepath.Join("testdata", "golden", tc.name+".md")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatalf("mkdir golden: %v", err)
				}
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden %s: %v (run with UPDATE_GOLDEN=1 to create)", golden, err)
			}
			if got != string(want) {
				t.Errorf("markdown mismatch for %s\n--- got ---\n%q\n--- want ---\n%q", tc.name, got, string(want))
			}
		})
	}
}

// TestRenderMarkdownCap asserts the visible-action-item cap directly, so the
// cap is verified independently of the golden snapshot.
func TestRenderMarkdownCap(t *testing.T) {
	got := RenderMarkdown(highBurdenReport(), MarkdownOptions{})
	// Action items live between the section header and the <details> block.
	_, after, ok := strings.Cut(got, "### Before maintainer review\n\n")
	if !ok {
		t.Fatal("missing action-item section")
	}
	section, _, ok := strings.Cut(after, "\n\n<details>")
	if !ok {
		t.Fatal("missing details block after action items")
	}
	n := strings.Count(section, "\n- ") + 1
	if n != maxActionItems {
		t.Fatalf("visible action items = %d, want %d\nsection:\n%s", n, maxActionItems, section)
	}
	// Every finding message still appears in the details block.
	for _, f := range highBurdenReport().Findings {
		if !strings.Contains(got, "- "+f.Message) {
			t.Errorf("details block missing message: %q", f.Message)
		}
	}
}

// TestRenderMarkdownStructure checks invariants that hold for every report.
func TestRenderMarkdownStructure(t *testing.T) {
	reports := map[string]*model.Report{
		"ready":         readyReport(),
		"needs-action":  needsActionReport(),
		"high-burden":   highBurdenReport(),
		"not-evaluated": notEvaluatedReport(),
	}
	for name, r := range reports {
		t.Run(name, func(t *testing.T) {
			out := RenderMarkdown(r, MarkdownOptions{})
			if !strings.HasPrefix(out, Marker+"\n\n") {
				t.Errorf("output must start with marker then blank line")
			}
			if !strings.HasSuffix(out, disclaimer+"\n") {
				t.Errorf("output must end with disclaimer then newline")
			}
			// Score must never leak without ShowScore.
			if strings.Contains(out, "Internal score") {
				t.Errorf("score leaked into non-ShowScore markdown")
			}
			// The two static stat labels always render.
			for _, label := range []string{"**Review burden:**", "**Ownership:**", "**Tests:**"} {
				if !strings.Contains(out, label) {
					t.Errorf("missing stat label %q", label)
				}
			}
		})
	}
}

// TestRenderMarkdownShowScore verifies the internal score line is placed after
// the Tests line and that the Tests line gains a hard break before it.
func TestRenderMarkdownShowScore(t *testing.T) {
	out := RenderMarkdown(needsActionReport(), MarkdownOptions{ShowScore: true})
	want := "**Tests:** Missing matching updates  \n**Internal score:** 60/100"
	if !strings.Contains(out, want) {
		t.Errorf("score line placement wrong; output:\n%s", out)
	}
}

// TestRenderMarkdownDeterministic renders the same report twice and requires
// byte-identical output.
func TestRenderMarkdownDeterministic(t *testing.T) {
	for _, opts := range []MarkdownOptions{{}, {ShowScore: true}} {
		r := needsActionReport()
		a := RenderMarkdown(r, opts)
		b := RenderMarkdown(r, opts)
		if a != b {
			t.Fatalf("non-deterministic markdown (ShowScore=%v)", opts.ShowScore)
		}
	}
}

func TestRenderJSONShape(t *testing.T) {
	b, err := RenderJSON(needsActionReport())
	if err != nil {
		t.Fatalf("RenderJSON: %v", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatal("JSON output must end with a trailing newline")
	}
	// Two-space indentation.
	if !strings.Contains(string(b), "\n  \"schema_version\": \"0.1\"") {
		t.Errorf("expected two-space indented schema_version line; got:\n%s", b)
	}
	// Deterministic across renders.
	b2, _ := RenderJSON(needsActionReport())
	if string(b) != string(b2) {
		t.Fatal("non-deterministic JSON output")
	}
}

// TestRenderJSONRoundTrip marshals a report, unmarshals it back into a
// model.Report, and requires deep equality.
func TestRenderJSONRoundTrip(t *testing.T) {
	for _, mk := range []func() *model.Report{readyReport, needsActionReport, highBurdenReport, notEvaluatedReport} {
		orig := mk()
		b, err := RenderJSON(orig)
		if err != nil {
			t.Fatalf("RenderJSON: %v", err)
		}
		var got model.Report
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if !reflect.DeepEqual(*orig, got) {
			t.Errorf("round-trip mismatch\norig: %+v\ngot:  %+v", *orig, got)
		}
	}
}

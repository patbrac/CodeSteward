package readiness

import (
	"reflect"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// ruleSeverity maps each rule ID to its contractual severity (CONTRACTS §4).
// Used by the test finding builder so synthesized findings carry realistic
// severities (which drives ExitCriteria and canonical ordering).
var ruleSeverity = map[string]model.Severity{
	model.RuleOwnNoOwner:        model.SeverityActionRequired,
	model.RuleOwnFallbackOnly:   model.SeverityWarning,
	model.RuleOwnTooManyAreas:   model.SeverityWarning,
	model.RuleOwnSensitiveNoOwn: model.SeverityActionRequired,
	model.RuleTstMissing:        model.SeverityActionRequired,
	model.RuleTstNotUpdated:     model.SeverityWarning,
	model.RuleTstUnresolved:     model.SeverityInfo,
	model.RuleScpTooManyFiles:   model.SeverityWarning,
	model.RuleScpTooManyLines:   model.SeverityWarning,
	model.RuleScpSrcPlusDeps:    model.SeverityWarning,
	model.RuleScpMixedConcerns:  model.SeverityWarning,
	model.RuleScpTooManyAreas:   model.SeverityInfo,
	model.RuleDscEmpty:          model.SeverityWarning,
	model.RuleDscTooShort:       model.SeverityWarning,
	model.RuleDscMissingSection: model.SeverityWarning,
	model.RuleDscNoLinkedIssue:  model.SeverityWarning,
	model.RuleSnsLockfile:       model.SeverityWarning,
	model.RuleSnsCIWorkflow:     model.SeverityWarning,
	model.RuleSnsManifest:       model.SeverityWarning,
	model.RuleSnsConfigured:     model.SeverityWarning,
}

// mk builds a finding for a rule ID with the contractual severity and a
// synthetic message/action, attaching the given paths.
func mk(ruleID string, paths ...string) model.Finding {
	return model.Finding{
		RuleID:   ruleID,
		Severity: ruleSeverity[ruleID],
		Message:  "msg " + ruleID,
		Action:   "do " + ruleID,
		Paths:    paths,
	}
}

// TestPenaltyTable locks every penalty value from CONTRACTS §4 so drift is
// caught immediately.
func TestPenaltyTable(t *testing.T) {
	want := map[string]int{
		model.RuleOwnNoOwner:        25,
		model.RuleOwnFallbackOnly:   10,
		model.RuleOwnTooManyAreas:   10,
		model.RuleOwnSensitiveNoOwn: 15,
		model.RuleTstMissing:        25,
		model.RuleTstNotUpdated:     15,
		model.RuleTstUnresolved:     10,
		model.RuleScpTooManyFiles:   20,
		model.RuleScpTooManyLines:   15,
		model.RuleScpSrcPlusDeps:    10,
		model.RuleScpMixedConcerns:  10,
		model.RuleScpTooManyAreas:   0,
		model.RuleDscEmpty:          5,
		model.RuleDscTooShort:       10,
		model.RuleDscMissingSection: 15,
		model.RuleDscNoLinkedIssue:  10,
		model.RuleSnsLockfile:       15,
		model.RuleSnsCIWorkflow:     15,
		model.RuleSnsManifest:       10,
		model.RuleSnsConfigured:     10,
	}
	if len(rulePenalties) != len(want) {
		t.Fatalf("rulePenalties has %d entries, want %d", len(rulePenalties), len(want))
	}
	for id, p := range want {
		if got := rulePenalties[id]; got != p {
			t.Errorf("penalty[%s] = %d, want %d", id, got, p)
		}
	}
}

// referenceScenario captures the acceptance data from CONTRACTS §7.2.
type referenceScenario struct {
	name       string
	findings   []model.Finding
	ownership  model.OwnershipState
	wantScore  int
	wantStatus model.ReadinessStatus
	wantBurden model.ReviewBurden
}

func TestReferenceScenarios(t *testing.T) {
	scenarios := []referenceScenario{
		{
			name:       "scenario1_good_pr",
			findings:   nil,
			ownership:  model.OwnershipComplete,
			wantScore:  100,
			wantStatus: model.StatusReady,
			wantBurden: model.BurdenLow,
		},
		{
			name: "scenario2_missing_tests_weak_ownership",
			findings: []model.Finding{
				mk(model.RuleOwnFallbackOnly, "src/runtime/cache.ts"),
				mk(model.RuleTstMissing, "src/runtime/cache.ts"),
				mk(model.RuleDscEmpty),
			},
			ownership:  model.OwnershipPartial,
			wantScore:  60,
			wantStatus: model.StatusNeedsAction,
			wantBurden: model.BurdenMedium,
		},
		{
			name: "scenario3_broad_pr",
			findings: []model.Finding{
				mk(model.RuleOwnFallbackOnly, "src/runtime/cache.ts"),
				mk(model.RuleTstMissing, "src/runtime/cache.ts"),
				mk(model.RuleScpSrcPlusDeps),
				mk(model.RuleScpMixedConcerns),
				mk(model.RuleDscEmpty),
				mk(model.RuleSnsCIWorkflow, ".github/workflows/release.yml"),
				mk(model.RuleSnsManifest, "package.json"),
			},
			ownership:  model.OwnershipPartial,
			wantScore:  15,
			wantStatus: model.StatusHighBurden,
			wantBurden: model.BurdenHigh,
		},
	}
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			gotScore := Score(sc.findings)
			if gotScore != sc.wantScore {
				t.Errorf("Score = %d, want %d", gotScore, sc.wantScore)
			}
			own := model.OwnershipSummary{State: sc.ownership}
			if got := Status(gotScore, own, sc.findings); got != sc.wantStatus {
				t.Errorf("Status = %q, want %q", got, sc.wantStatus)
			}
			if got := Burden(gotScore, sc.findings); got != sc.wantBurden {
				t.Errorf("Burden = %q, want %q", got, sc.wantBurden)
			}
		})
	}
}

func TestScoreDistinctDedup(t *testing.T) {
	// Same rule fired twice (different paths) must count once.
	findings := []model.Finding{
		mk(model.RuleOwnNoOwner, "src/a.ts"),
		mk(model.RuleOwnNoOwner, "src/b.ts"),
	}
	if got := Score(findings); got != 75 {
		t.Fatalf("Score = %d, want 75 (25 counted once)", got)
	}
}

func TestScoreClampAtZero(t *testing.T) {
	// Penalties sum to 130 (>100); score must clamp to 0, not go negative.
	findings := []model.Finding{
		mk(model.RuleOwnNoOwner),        // 25
		mk(model.RuleOwnSensitiveNoOwn), // 15
		mk(model.RuleTstMissing),        // 25
		mk(model.RuleScpTooManyFiles),   // 20
		mk(model.RuleScpTooManyLines),   // 15
		mk(model.RuleDscMissingSection), // 15
		mk(model.RuleSnsLockfile),       // 15
	}
	if got := Score(findings); got != 0 {
		t.Fatalf("Score = %d, want 0 (clamped)", got)
	}
}

func TestScoreZeroPenaltyRule(t *testing.T) {
	// CS-SCP-005 is advisory (penalty 0): score stays 100.
	if got := Score([]model.Finding{mk(model.RuleScpTooManyAreas)}); got != 100 {
		t.Fatalf("Score = %d, want 100 for advisory-only finding", got)
	}
}

func TestStatusBoundaries(t *testing.T) {
	// Non-missing ownership + no findings => pure score->status mapping,
	// override never triggers.
	own := model.OwnershipSummary{State: model.OwnershipNotEvaluated}
	cases := []struct {
		score int
		want  model.ReadinessStatus
	}{
		{0, model.StatusHighBurden},
		{39, model.StatusHighBurden},
		{40, model.StatusNeedsAction},
		{64, model.StatusNeedsAction},
		{65, model.StatusReviewableNotes},
		{84, model.StatusReviewableNotes},
		{85, model.StatusReady},
		{100, model.StatusReady},
	}
	for _, c := range cases {
		if got := Status(c.score, own, nil); got != c.want {
			t.Errorf("Status(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestStatusOwnerRoutingOverride(t *testing.T) {
	cases := []struct {
		name      string
		score     int
		ownership model.OwnershipState
		findings  []model.Finding
		want      model.ReadinessStatus
	}{
		{
			name:      "positive_owner_only",
			score:     75,
			ownership: model.OwnershipMissing,
			findings:  []model.Finding{mk(model.RuleOwnNoOwner, "src/a.ts")}, // own 25, other 0
			want:      model.StatusNeedsOwnerRoute,
		},
		{
			name:      "positive_exact_tie",
			score:     80,
			ownership: model.OwnershipMissing,
			// own 10 (CS-OWN-002), other 10 (CS-DSC-002) -> tie counts as >=.
			findings: []model.Finding{
				mk(model.RuleOwnFallbackOnly, "src/a.ts"),
				mk(model.RuleDscTooShort),
			},
			want: model.StatusNeedsOwnerRoute,
		},
		{
			name:      "negative_other_dominates",
			score:     30,
			ownership: model.OwnershipMissing,
			// own 25, other 45 -> no override, base mapping (high burden).
			findings: []model.Finding{
				mk(model.RuleOwnNoOwner, "src/a.ts"),
				mk(model.RuleTstMissing, "src/a.ts"),
				mk(model.RuleScpTooManyFiles),
			},
			want: model.StatusHighBurden,
		},
		{
			name:      "negative_state_not_missing",
			score:     75,
			ownership: model.OwnershipPartial,
			findings:  []model.Finding{mk(model.RuleOwnNoOwner, "src/a.ts")},
			want:      model.StatusReviewableNotes,
		},
		{
			name:      "negative_score_not_below_85",
			score:     85,
			ownership: model.OwnershipMissing,
			findings:  nil,
			want:      model.StatusReady,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			own := model.OwnershipSummary{State: c.ownership}
			if got := Status(c.score, own, c.findings); got != c.want {
				t.Errorf("Status = %q, want %q", got, c.want)
			}
		})
	}
}

func TestBurden(t *testing.T) {
	cases := []struct {
		name     string
		score    int
		findings []model.Finding
		want     model.ReviewBurden
	}{
		{"high_low_score", 10, nil, model.BurdenHigh},
		{"high_scp001_even_if_high_score", 100, []model.Finding{mk(model.RuleScpTooManyFiles)}, model.BurdenHigh},
		{"high_scp002", 90, []model.Finding{mk(model.RuleScpTooManyLines)}, model.BurdenHigh},
		{"low_clean_high_score", 90, []model.Finding{mk(model.RuleDscEmpty)}, model.BurdenLow},
		{"low_no_findings", 100, nil, model.BurdenLow},
		{"medium_mid_score", 70, nil, model.BurdenMedium},
		{"medium_high_score_with_sns", 90, []model.Finding{mk(model.RuleSnsManifest, "package.json")}, model.BurdenMedium},
		{"medium_high_score_with_advisory_scp005", 100, []model.Finding{mk(model.RuleScpTooManyAreas)}, model.BurdenMedium},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Burden(c.score, c.findings); got != c.want {
				t.Errorf("Burden(%d) = %q, want %q", c.score, got, c.want)
			}
		})
	}
}

func TestExitCriteria(t *testing.T) {
	findings := []model.Finding{
		// action_required, dedup key (rule, first path)
		mk(model.RuleTstMissing, "src/a.ts"),
		mk(model.RuleTstMissing, "src/a.ts"), // duplicate rule+path -> dropped
		mk(model.RuleTstMissing, "src/b.ts"), // same rule, different first path -> kept
		mk(model.RuleOwnNoOwner, "src/a.ts"),
		// warnings and info must be excluded
		mk(model.RuleOwnFallbackOnly, "src/a.ts"),
		mk(model.RuleTstUnresolved, "src/a.ts"),
	}
	got := ExitCriteria(findings)
	want := []model.ExitCriterion{
		{RuleID: model.RuleTstMissing, Description: "do " + model.RuleTstMissing},
		{RuleID: model.RuleTstMissing, Description: "do " + model.RuleTstMissing},
		{RuleID: model.RuleOwnNoOwner, Description: "do " + model.RuleOwnNoOwner},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExitCriteria = %+v, want %+v", got, want)
	}
}

func TestExitCriteriaFirstPathFromUnsortedPaths(t *testing.T) {
	// Two findings with the same rule and the same *minimum* path (differing
	// only in path order) dedup to one, since "first path" is the smallest.
	f1 := model.Finding{RuleID: model.RuleOwnNoOwner, Severity: model.SeverityActionRequired, Action: "x", Paths: []string{"src/a.ts", "src/z.ts"}}
	f2 := model.Finding{RuleID: model.RuleOwnNoOwner, Severity: model.SeverityActionRequired, Action: "x", Paths: []string{"src/z.ts", "src/a.ts"}}
	got := ExitCriteria([]model.Finding{f1, f2})
	if len(got) != 1 {
		t.Fatalf("ExitCriteria len = %d, want 1 (deduped by min path)", len(got))
	}
}

func TestExitCriteriaEmptyIsNonNil(t *testing.T) {
	got := ExitCriteria(nil)
	if got == nil {
		t.Fatal("ExitCriteria(nil) = nil, want non-nil empty slice")
	}
	if len(got) != 0 {
		t.Fatalf("ExitCriteria(nil) len = %d, want 0", len(got))
	}
}

func TestCanonicalFindingOrder(t *testing.T) {
	in := []model.Finding{
		mk(model.RuleTstUnresolved, "src/x.ts"),   // info
		mk(model.RuleOwnFallbackOnly, "src/z.ts"), // warning, path z
		mk(model.RuleTstMissing, "src/b.ts"),      // action_required
		mk(model.RuleOwnNoOwner, "src/c.ts"),      // action_required, same rule, path c
		mk(model.RuleOwnNoOwner, "src/a.ts"),      // action_required, same rule, path a
		mk(model.RuleOwnFallbackOnly, "src/a.ts"), // warning, path a
	}
	got := canonicalFindings(in)

	type key struct {
		rule string
		path string
	}
	wantOrder := []key{
		{model.RuleOwnNoOwner, "src/a.ts"},
		{model.RuleOwnNoOwner, "src/c.ts"},
		{model.RuleTstMissing, "src/b.ts"},
		{model.RuleOwnFallbackOnly, "src/a.ts"},
		{model.RuleOwnFallbackOnly, "src/z.ts"},
		{model.RuleTstUnresolved, "src/x.ts"},
	}
	if len(got) != len(wantOrder) {
		t.Fatalf("got %d findings, want %d", len(got), len(wantOrder))
	}
	for i, w := range wantOrder {
		if got[i].RuleID != w.rule || firstPath(got[i].Paths) != w.path {
			t.Errorf("pos %d = (%s,%s), want (%s,%s)", i, got[i].RuleID, firstPath(got[i].Paths), w.rule, w.path)
		}
	}
}

func TestCanonicalMessageTiebreak(t *testing.T) {
	// Same severity, rule, and first path -> Message breaks the tie.
	in := []model.Finding{
		{RuleID: model.RuleSnsConfigured, Severity: model.SeverityWarning, Message: "zeta", Paths: []string{"config/a.yml"}},
		{RuleID: model.RuleSnsConfigured, Severity: model.SeverityWarning, Message: "alpha", Paths: []string{"config/a.yml"}},
	}
	got := canonicalFindings(in)
	if got[0].Message != "alpha" || got[1].Message != "zeta" {
		t.Fatalf("message tiebreak order = [%q,%q], want [alpha,zeta]", got[0].Message, got[1].Message)
	}
}

func TestCanonicalSortsPathsWithinFinding(t *testing.T) {
	in := []model.Finding{
		{RuleID: model.RuleSnsLockfile, Severity: model.SeverityWarning, Paths: []string{"yarn.lock", "go.sum", "Cargo.lock"}},
	}
	got := canonicalFindings(in)
	want := []string{"Cargo.lock", "go.sum", "yarn.lock"}
	if !reflect.DeepEqual(got[0].Paths, want) {
		t.Fatalf("paths within finding = %v, want %v", got[0].Paths, want)
	}
}

func TestCanonicalDoesNotMutateInput(t *testing.T) {
	in := []model.Finding{
		{RuleID: model.RuleSnsLockfile, Severity: model.SeverityWarning, Paths: []string{"yarn.lock", "go.sum"}},
	}
	_ = canonicalFindings(in)
	if !reflect.DeepEqual(in[0].Paths, []string{"yarn.lock", "go.sum"}) {
		t.Fatalf("input Paths mutated: %v", in[0].Paths)
	}
}

func TestBuildReport(t *testing.T) {
	in := BuildInput{
		Version:     "9.9.9",
		Base:        "main",
		Head:        "HEAD",
		CommentOnly: true,
		Ownership:   model.OwnershipSummary{State: model.OwnershipPartial, AreasTouched: 1, MaxAreas: 2},
		Tests:       model.TestsSummary{State: model.TestsMissingMatching},
		Scope:       model.ScopeSummary{FilesChanged: 1},
		Description: model.DescriptionSummary{Provided: false, Evaluated: true},
		Findings: []model.Finding{
			mk(model.RuleDscEmpty),                                // warning, -5
			mk(model.RuleTstMissing, "src/runtime/cache.ts"),      // action_required, -25
			mk(model.RuleOwnFallbackOnly, "src/runtime/cache.ts"), // warning, -10
		},
		Warnings: []string{"no CODEOWNERS found"},
	}
	r := BuildReport(in)

	if r.SchemaVersion != "0.1" {
		t.Errorf("SchemaVersion = %q, want 0.1", r.SchemaVersion)
	}
	if r.Tool.Name != "codesteward" || r.Tool.Version != "9.9.9" {
		t.Errorf("Tool = %+v, want {codesteward 9.9.9}", r.Tool)
	}
	if r.Base != "main" || r.Head != "HEAD" || !r.CommentOnly {
		t.Errorf("base/head/commentOnly not propagated: %+v", r)
	}
	if r.Score != 60 {
		t.Errorf("Score = %d, want 60", r.Score)
	}
	if r.Status != model.StatusNeedsAction {
		t.Errorf("Status = %q, want %q", r.Status, model.StatusNeedsAction)
	}
	if r.ReviewBurden != model.BurdenMedium {
		t.Errorf("ReviewBurden = %q, want medium", r.ReviewBurden)
	}
	if !reflect.DeepEqual(r.Ownership, in.Ownership) {
		t.Errorf("Ownership not propagated: %+v", r.Ownership)
	}
	if !reflect.DeepEqual(r.Tests, in.Tests) {
		t.Errorf("Tests not propagated: %+v", r.Tests)
	}
	if !reflect.DeepEqual(r.Scope, in.Scope) {
		t.Errorf("Scope not propagated: %+v", r.Scope)
	}
	if !reflect.DeepEqual(r.Description, in.Description) {
		t.Errorf("Description not propagated: %+v", r.Description)
	}
	if !reflect.DeepEqual(r.Warnings, in.Warnings) {
		t.Errorf("Warnings not propagated: %+v", r.Warnings)
	}
	// Canonical order: action_required (CS-TST-001) first, then warnings by
	// RuleID ascending (CS-DSC-001 < CS-OWN-002).
	wantRules := []string{model.RuleTstMissing, model.RuleDscEmpty, model.RuleOwnFallbackOnly}
	if len(r.Findings) != len(wantRules) {
		t.Fatalf("got %d findings, want %d", len(r.Findings), len(wantRules))
	}
	for i, w := range wantRules {
		if r.Findings[i].RuleID != w {
			t.Errorf("finding[%d] = %s, want %s", i, r.Findings[i].RuleID, w)
		}
	}
	// One exit criterion from the single action_required finding.
	if len(r.ExitCriteria) != 1 || r.ExitCriteria[0].RuleID != model.RuleTstMissing {
		t.Fatalf("ExitCriteria = %+v, want single CS-TST-001", r.ExitCriteria)
	}
}

func TestBuildReportFindingsNonNil(t *testing.T) {
	r := BuildReport(BuildInput{Version: "1"})
	if r.Findings == nil {
		t.Error("Findings = nil, want empty non-nil slice")
	}
	if r.ExitCriteria == nil {
		t.Error("ExitCriteria = nil, want empty non-nil slice")
	}
}

func TestBuildReportDeterministic(t *testing.T) {
	in := BuildInput{
		Version: "1",
		Findings: []model.Finding{
			mk(model.RuleTstMissing, "src/b.ts"),
			mk(model.RuleOwnNoOwner, "src/a.ts"),
			mk(model.RuleDscEmpty),
		},
	}
	a := BuildReport(in)
	b := BuildReport(in)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("BuildReport is not deterministic across identical inputs")
	}
}

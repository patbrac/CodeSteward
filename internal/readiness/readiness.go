// Package readiness scores findings and assembles the final report.
//
// It converts a set of deterministic findings into an internal numeric
// score, a user-facing readiness status, a review-burden level, exit
// criteria, and finally a fully assembled model.Report. All output is
// deterministic: identical inputs always produce identical results.
package readiness

import (
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// rulePenalties is the scoring penalty table from CONTRACTS §4, keyed by the
// pkg/model rule ID constants. A rule ID absent from this map contributes no
// penalty (map lookup yields 0).
var rulePenalties = map[string]int{
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

const (
	ownPrefix = "CS-OWN-"
	scpPrefix = "CS-SCP-"
	snsPrefix = "CS-SNS-"
)

// Score computes the internal readiness score in [0,100].
//
// It starts at 100 and subtracts the §4 penalty once per DISTINCT rule ID
// present in findings (a rule that fires multiple times still costs its
// penalty only once), then clamps the result to [0,100].
func Score(findings []model.Finding) int {
	seen := make(map[string]bool, len(findings))
	total := 0
	for _, f := range findings {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true
		total += rulePenalties[f.RuleID]
	}
	score := 100 - total
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// Burden maps score and findings to a review burden level.
//
//   - high   when score < 40, or CS-SCP-001 or CS-SCP-002 fired.
//   - low    when score >= 85 and no CS-SCP-* or CS-SNS-* findings are present.
//   - medium otherwise.
func Burden(score int, findings []model.Finding) model.ReviewBurden {
	var scp001, scp002, anyScp, anySns bool
	for _, f := range findings {
		switch f.RuleID {
		case model.RuleScpTooManyFiles:
			scp001 = true
		case model.RuleScpTooManyLines:
			scp002 = true
		}
		if strings.HasPrefix(f.RuleID, scpPrefix) {
			anyScp = true
		}
		if strings.HasPrefix(f.RuleID, snsPrefix) {
			anySns = true
		}
	}
	if score < 40 || scp001 || scp002 {
		return model.BurdenHigh
	}
	if score >= 85 && !anyScp && !anySns {
		return model.BurdenLow
	}
	return model.BurdenMedium
}

// Status maps score, ownership, and findings to a readiness status.
//
// Base mapping: 85-100 ready, 65-84 reviewable_with_notes, 40-64
// needs_contributor_action, 0-39 high_review_burden. The status is overridden
// to needs_owner_routing when ownership is the dominant problem, i.e. all of:
//   - ownership.State == missing, and
//   - score < 85, and
//   - the combined penalty of CS-OWN-* findings is >= the combined penalty of
//     all other findings (both counted once per distinct rule ID).
func Status(score int, ownership model.OwnershipSummary, findings []model.Finding) model.ReadinessStatus {
	if ownership.State == model.OwnershipMissing && score < 85 {
		own, other := categoryPenalties(findings)
		if own >= other {
			return model.StatusNeedsOwnerRoute
		}
	}
	switch {
	case score >= 85:
		return model.StatusReady
	case score >= 65:
		return model.StatusReviewableNotes
	case score >= 40:
		return model.StatusNeedsAction
	default:
		return model.StatusHighBurden
	}
}

// categoryPenalties returns the combined penalty of CS-OWN-* findings and the
// combined penalty of all other findings, counting each distinct rule ID once.
func categoryPenalties(findings []model.Finding) (own, other int) {
	seen := make(map[string]bool, len(findings))
	for _, f := range findings {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true
		p := rulePenalties[f.RuleID]
		if strings.HasPrefix(f.RuleID, ownPrefix) {
			own += p
		} else {
			other += p
		}
	}
	return own, other
}

// ExitCriteria derives exit criteria from action-required findings: one per
// action_required finding, deduplicated by (RuleID, first path), with the
// description taken from the finding's Action. Input order is preserved among
// surviving entries.
func ExitCriteria(findings []model.Finding) []model.ExitCriterion {
	out := []model.ExitCriterion{}
	seen := map[string]bool{}
	for _, f := range findings {
		if f.Severity != model.SeverityActionRequired {
			continue
		}
		key := f.RuleID + "\x00" + firstPath(f.Paths)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, model.ExitCriterion{
			RuleID:      f.RuleID,
			Description: f.Action,
		})
	}
	return out
}

// firstPath returns the lexicographically smallest path in paths (the "first
// path" once paths are sorted), or "" when there are none. Computing the
// minimum makes the result independent of input order.
func firstPath(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	min := paths[0]
	for _, p := range paths[1:] {
		if p < min {
			min = p
		}
	}
	return min
}

// severityRank returns the canonical severity ordering rank (§7.6):
// action_required=0, warning=1, info=2, anything else last.
func severityRank(s model.Severity) int {
	switch s {
	case model.SeverityActionRequired:
		return 0
	case model.SeverityWarning:
		return 1
	case model.SeverityInfo:
		return 2
	default:
		return 3
	}
}

// canonicalFindings returns a new slice containing copies of the input
// findings with their Paths sorted ascending, ordered by the §7.6 canonical
// rule: severity rank, then RuleID, then first path, then Message. The input
// slice and its Paths slices are never mutated.
func canonicalFindings(in []model.Finding) []model.Finding {
	out := make([]model.Finding, len(in))
	for i, f := range in {
		if len(f.Paths) > 0 {
			paths := make([]string, len(f.Paths))
			copy(paths, f.Paths)
			sort.Strings(paths)
			f.Paths = paths
		}
		out[i] = f
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := severityRank(a.Severity), severityRank(b.Severity); ra != rb {
			return ra < rb
		}
		if a.RuleID != b.RuleID {
			return a.RuleID < b.RuleID
		}
		if fa, fb := firstPath(a.Paths), firstPath(b.Paths); fa != fb {
			return fa < fb
		}
		return a.Message < b.Message
	})
	return out
}

// BuildInput carries everything needed to assemble a report.
type BuildInput struct {
	Version     string
	Base        string
	Head        string
	CommentOnly bool
	Ownership   model.OwnershipSummary
	Tests       model.TestsSummary
	Scope       model.ScopeSummary
	Description model.DescriptionSummary
	Findings    []model.Finding
	Warnings    []string
}

// BuildReport assembles the full model.Report. It applies the canonical
// finding order (§7.6, including sorting Paths within each finding), derives
// Score/Status/ReviewBurden/ExitCriteria from the sorted findings, and sets
// SchemaVersion "0.1" and ToolInfo{Name: "codesteward", Version}.
func BuildReport(in BuildInput) *model.Report {
	findings := canonicalFindings(in.Findings)
	score := Score(findings)
	return &model.Report{
		SchemaVersion: "0.1",
		Tool:          model.ToolInfo{Name: "codesteward", Version: in.Version},
		Base:          in.Base,
		Head:          in.Head,
		CommentOnly:   in.CommentOnly,
		Status:        Status(score, in.Ownership, findings),
		Score:         score,
		ReviewBurden:  Burden(score, findings),
		Ownership:     in.Ownership,
		Tests:         in.Tests,
		Scope:         in.Scope,
		Description:   in.Description,
		Findings:      findings,
		ExitCriteria:  ExitCriteria(findings),
		Warnings:      in.Warnings,
	}
}

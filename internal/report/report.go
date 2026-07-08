// Package report renders a model.Report to Markdown and JSON.
//
// Both renderers are fully deterministic: identical reports produce
// byte-identical output. The Markdown renderer preserves the canonical finding
// order established by internal/readiness (CONTRACTS §7.6) and never emits the
// numeric score unless explicitly requested.
package report

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Marker is the hidden HTML comment that identifies a CodeSteward comment/note
// so it can be updated in place instead of duplicated.
const Marker = "<!-- codesteward-report -->"

// maxActionItems caps how many visible action items appear in the
// "Before maintainer review" section (CONTRACTS §6.9).
const maxActionItems = 5

// disclaimer is the trailing comment-only line, always rendered.
const disclaimer = "_Comment-only mode. CodeSteward is not blocking this PR._"

// summaryLine is the fixed <summary> text for the details block.
const summaryLine = "<summary>Why CodeSteward flagged this</summary>"

const (
	introReady  = "Thanks for the contribution. This looks ready for maintainer review."
	introAction = "Thanks for the contribution. A few changes would make this easier for maintainers to review."
)

// MarkdownOptions configures Markdown rendering.
type MarkdownOptions struct {
	ShowScore bool // default false
}

// Display-string maps (CONTRACTS §6.9). Unknown values fall back to the raw
// underlying string so rendering never silently drops information.

var statusDisplay = map[model.ReadinessStatus]string{
	model.StatusReady:           "Ready for maintainer review",
	model.StatusReviewableNotes: "Reviewable with notes",
	model.StatusNeedsAction:     "Needs contributor action",
	model.StatusHighBurden:      "High review burden",
	model.StatusNeedsOwnerRoute: "Needs owner routing",
}

var burdenDisplay = map[model.ReviewBurden]string{
	model.BurdenLow:    "Low",
	model.BurdenMedium: "Medium",
	model.BurdenHigh:   "High",
}

var ownershipDisplay = map[model.OwnershipState]string{
	model.OwnershipComplete:     "Complete",
	model.OwnershipPartial:      "Partial",
	model.OwnershipMissing:      "Missing",
	model.OwnershipNotEvaluated: "Not evaluated",
}

var testsDisplay = map[model.TestState]string{
	model.TestsMatchingChanged:    "Present",
	model.TestsMissingMatching:    "Missing matching updates",
	model.TestsExistingNotChanged: "Existing tests not updated",
	model.TestsNotRequired:        "Not required",
	model.TestsNotEvaluated:       "Not evaluated",
}

func displayStatus(s model.ReadinessStatus) string {
	if v, ok := statusDisplay[s]; ok {
		return v
	}
	return string(s)
}

func displayBurden(b model.ReviewBurden) string {
	if v, ok := burdenDisplay[b]; ok {
		return v
	}
	return string(b)
}

func displayOwnership(o model.OwnershipState) string {
	if v, ok := ownershipDisplay[o]; ok {
		return v
	}
	return string(o)
}

func displayTests(t model.TestState) string {
	if v, ok := testsDisplay[t]; ok {
		return v
	}
	return string(t)
}

// actionItems returns the deduplicated, capped list of contributor action
// items in canonical finding order. Findings must already be in canonical
// order (readiness.BuildReport guarantees this; §7.6).
func actionItems(findings []model.Finding) []string {
	items := make([]string, 0, maxActionItems)
	seen := make(map[string]struct{})
	for _, f := range findings {
		if f.Action == "" {
			continue
		}
		if _, ok := seen[f.Action]; ok {
			continue
		}
		seen[f.Action] = struct{}{}
		items = append(items, f.Action)
		if len(items) == maxActionItems {
			break
		}
	}
	return items
}

// RenderMarkdown renders the report as a compact Markdown comment following the
// exact template in CONTRACTS §6.9. The numeric score is hidden unless
// opts.ShowScore is set.
func RenderMarkdown(r *model.Report, opts MarkdownOptions) string {
	// The document is a sequence of blocks joined by a single blank line.
	blocks := make([]string, 0, 8)

	// Marker.
	blocks = append(blocks, Marker)

	// Header.
	blocks = append(blocks, "## CodeSteward: "+displayStatus(r.Status))

	// Stat lines. Consecutive lines are separated by a Markdown hard break
	// (two trailing spaces); the last line of the block carries no trailing
	// spaces. The optional internal score line is appended after Tests.
	statLines := []string{
		"**Review burden:** " + displayBurden(r.ReviewBurden),
		"**Ownership:** " + displayOwnership(r.Ownership.State),
		"**Tests:** " + displayTests(r.Tests.State),
	}
	if opts.ShowScore {
		statLines = append(statLines, "**Internal score:** "+strconv.Itoa(r.Score)+"/100")
	}
	blocks = append(blocks, strings.Join(statLines, "  \n"))

	// Intro line, keyed on the presence of visible action items.
	actions := actionItems(r.Findings)
	if len(actions) == 0 {
		blocks = append(blocks, introReady)
	} else {
		blocks = append(blocks, introAction)
	}

	// "Before maintainer review" section, omitted entirely when no actions.
	if len(actions) > 0 {
		lines := make([]string, 0, len(actions)+2)
		lines = append(lines, "### Before maintainer review", "")
		for _, a := range actions {
			lines = append(lines, "- "+a)
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	// Collapsed <details> block with every finding Message in canonical order,
	// omitted entirely when there are no findings.
	if len(r.Findings) > 0 {
		lines := make([]string, 0, len(r.Findings)+4)
		lines = append(lines, "<details>", summaryLine, "")
		for _, f := range r.Findings {
			lines = append(lines, "- "+f.Message)
		}
		lines = append(lines, "", "</details>")
		blocks = append(blocks, strings.Join(lines, "\n"))
	}

	// Comment-only disclaimer, always present.
	blocks = append(blocks, disclaimer)

	return strings.Join(blocks, "\n\n") + "\n"
}

// RenderJSON renders the report as stable, indented JSON: two-space indent with
// a single trailing newline. Field order follows the model.Report struct order
// and is part of the stable schema.
func RenderJSON(r *model.Report) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("codesteward: marshaling report to JSON: %w", err)
	}
	return append(b, '\n'), nil
}

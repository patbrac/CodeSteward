// Package rules implements the deterministic scope, description, and
// sensitive-path readiness rules (CS-SCP-*, CS-DSC-*, CS-SNS-*).
//
// Every function in this package is a pure function of its inputs: given the
// same changed files and options it always emits the same findings in the same
// order. Callers combine these findings with the ownership and test findings
// and apply the canonical finding sort in internal/readiness.
package rules

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/internal/globs"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// rootArea is the top-level area name used for files that live at the
// repository root (no directory component).
const rootArea = "(root)"

// Built-in sensitive file sets (§4 of CONTRACTS.md). Lockfiles and manifests
// are matched by exact, case-sensitive basename; CI/release files are matched
// by path glob.
var (
	lockfileBasenames = map[string]bool{
		"package-lock.json": true,
		"pnpm-lock.yaml":    true,
		"yarn.lock":         true,
		"bun.lockb":         true,
		"go.sum":            true,
		"Cargo.lock":        true,
		"poetry.lock":       true,
		"Gemfile.lock":      true,
		"composer.lock":     true,
	}
	manifestBasenames = map[string]bool{
		"package.json":   true,
		"go.mod":         true,
		"Cargo.toml":     true,
		"pyproject.toml": true,
		"Gemfile":        true,
		"composer.json":  true,
	}
	ciReleaseGlobs = []string{
		".github/workflows/**",
		".gitlab-ci.yml",
		"scripts/release/**",
	}
)

// linkedIssueRe matches a bare issue reference ("#123") or an issue URL
// fragment ("/issues/45").
var linkedIssueRe = regexp.MustCompile(`#\d+|/issues/\d+`)

// ScopeOptions configures scope analysis.
type ScopeOptions struct {
	MaxFiles int
	MaxLines int
}

// Scope evaluates PR/MR breadth and size and emits the CS-SCP-* findings.
//
// The reported FilesChanged/LinesAdded/LinesDeleted counts (and therefore the
// limit checks) exclude files flagged IsIgnored. TopLevelAreas, by contrast,
// lists the top-level segment of every changed file (ignored files included),
// using "(root)" for files that live at the repository root; it is sorted and
// deduplicated.
func Scope(files []model.ChangedFile, opts ScopeOptions) (model.ScopeSummary, []model.Finding) {
	summary := model.ScopeSummary{
		MaxFilesChanged: opts.MaxFiles,
		MaxLinesChanged: opts.MaxLines,
	}

	areaSet := make(map[string]bool)
	var hasProduction, hasLockOrManifest, hasDocs, hasConfigCI bool

	for _, f := range files {
		areaSet[topLevelArea(f.Path)] = true

		if !f.IsIgnored {
			summary.FilesChanged++
			summary.LinesAdded += f.Additions
			summary.LinesDeleted += f.Deletions
		}

		if f.IsProduction {
			hasProduction = true
		}
		if isLockfile(f.Path) || isManifest(f.Path) {
			hasLockOrManifest = true
		}
		if isDocs(f.Path) {
			hasDocs = true
		}
		if isConfigOrCI(f.Path) {
			hasConfigCI = true
		}
	}

	summary.TopLevelAreas = sortedKeys(areaSet)

	linesChanged := summary.LinesAdded + summary.LinesDeleted
	summary.ExceedsFileLimit = summary.FilesChanged > opts.MaxFiles
	summary.ExceedsLineLimit = linesChanged > opts.MaxLines

	var findings []model.Finding

	if summary.ExceedsFileLimit {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleScpTooManyFiles,
			Severity: model.SeverityWarning,
			Message:  fmt.Sprintf("This change touches %d files, above the configured limit of %d.", summary.FilesChanged, opts.MaxFiles),
			Action:   "Consider splitting this change into smaller, more focused PRs.",
		})
	}
	if summary.ExceedsLineLimit {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleScpTooManyLines,
			Severity: model.SeverityWarning,
			Message:  fmt.Sprintf("This change modifies %d lines, above the configured limit of %d.", linesChanged, opts.MaxLines),
			Action:   "Consider splitting this change into smaller, more focused PRs.",
		})
	}
	if hasProduction && hasLockOrManifest {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleScpSrcPlusDeps,
			Severity: model.SeverityWarning,
			Message:  "Dependency files changed alongside production source files.",
			Action:   "Consider splitting dependency changes from runtime changes.",
		})
	}
	if hasProduction && hasDocs && hasConfigCI {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleScpMixedConcerns,
			Severity: model.SeverityWarning,
			Message:  "Production source, documentation, and configuration files changed together.",
			Action:   "Consider splitting documentation and configuration changes into separate PRs.",
		})
	}
	if len(summary.TopLevelAreas) > 4 {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleScpTooManyAreas,
			Severity: model.SeverityInfo,
			Message:  fmt.Sprintf("This change touches %d top-level areas of the repository.", len(summary.TopLevelAreas)),
			Action:   "Consider whether this change can be split across fewer areas of the codebase.",
		})
	}

	return summary, findings
}

// DescriptionOptions configures PR/MR description analysis.
type DescriptionOptions struct {
	Text               string
	Evaluated          bool // false when there is no description source; emit no findings
	WarnIfEmpty        bool
	MinLength          int
	RequiredSections   []string
	RequireLinkedIssue bool
}

// Description evaluates PR/MR description quality and emits the CS-DSC-*
// findings.
//
// When Evaluated is false there is no description source (for example a local
// run without --description); the summary reports Evaluated=false and no
// findings are emitted. When the description is empty or whitespace-only, the
// only possible finding is CS-DSC-001 (gated on WarnIfEmpty); the length,
// required-section, and linked-issue checks are skipped. Otherwise length is
// measured in runes.
func Description(opts DescriptionOptions) (model.DescriptionSummary, []model.Finding) {
	if !opts.Evaluated {
		return model.DescriptionSummary{Evaluated: false}, nil
	}

	trimmed := strings.TrimSpace(opts.Text)
	summary := model.DescriptionSummary{
		Provided:  len(trimmed) > 0,
		Length:    len([]rune(trimmed)),
		Evaluated: true,
	}

	var findings []model.Finding

	if len(trimmed) == 0 {
		if opts.WarnIfEmpty {
			findings = append(findings, model.Finding{
				RuleID:   model.RuleDscEmpty,
				Severity: model.SeverityWarning,
				Message:  "The PR description is empty.",
				Action:   "Add a short description explaining the motivation and test plan.",
			})
		}
		return summary, findings
	}

	if opts.MinLength > 0 && summary.Length < opts.MinLength {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleDscTooShort,
			Severity: model.SeverityWarning,
			Message:  fmt.Sprintf("The PR description is %d characters, shorter than the configured minimum of %d.", summary.Length, opts.MinLength),
			Action:   fmt.Sprintf("Expand the PR description to at least %d characters explaining the motivation and test plan.", opts.MinLength),
		})
	}

	for _, section := range opts.RequiredSections {
		if sectionPresent(opts.Text, section) {
			continue
		}
		findings = append(findings, model.Finding{
			RuleID:   model.RuleDscMissingSection,
			Severity: model.SeverityWarning,
			Message:  fmt.Sprintf("The PR description is missing the `%s` section.", section),
			Action:   fmt.Sprintf("Add a `%s` section to the PR description.", section),
		})
	}

	if opts.RequireLinkedIssue && !linkedIssueRe.MatchString(opts.Text) {
		findings = append(findings, model.Finding{
			RuleID:   model.RuleDscNoLinkedIssue,
			Severity: model.SeverityWarning,
			Message:  "The PR description does not reference a linked issue.",
			Action:   "Reference the related issue (for example `#123`) in the PR description.",
		})
	}

	return summary, findings
}

// Sensitive emits one finding per fired CS-SNS rule, with the matching paths
// aggregated and sorted.
//
// Each file is classified into at most one category using the priority
// lockfile > CI/release workflow > manifest > configured-other. A file that is
// none of the built-in kinds falls into configured-other (CS-SNS-004) only when
// it is flagged IsSensitive (i.e. it matched a configured sensitive path).
func Sensitive(files []model.ChangedFile) []model.Finding {
	var lock, ci, manifest, other []string

	for _, f := range files {
		switch {
		case isLockfile(f.Path):
			lock = append(lock, f.Path)
		case isCIRelease(f.Path):
			ci = append(ci, f.Path)
		case isManifest(f.Path):
			manifest = append(manifest, f.Path)
		case f.IsSensitive:
			other = append(other, f.Path)
		}
	}

	var findings []model.Finding
	if len(lock) > 0 {
		findings = append(findings, sensitiveFinding(model.RuleSnsLockfile, "lockfile", lock))
	}
	if len(ci) > 0 {
		findings = append(findings, sensitiveFinding(model.RuleSnsCIWorkflow, "CI workflow", ci))
	}
	if len(manifest) > 0 {
		findings = append(findings, sensitiveFinding(model.RuleSnsManifest, "package manifest", manifest))
	}
	if len(other) > 0 {
		findings = append(findings, sensitiveFinding(model.RuleSnsConfigured, "sensitive path", other))
	}
	return findings
}

// sensitiveFinding builds a single CS-SNS finding for a category, with paths
// sorted, deduplicated, and rendered as backtick-quoted list in the message.
func sensitiveFinding(ruleID, kind string, paths []string) model.Finding {
	sorted := sortedUnique(paths)
	return model.Finding{
		RuleID:   ruleID,
		Severity: model.SeverityWarning,
		Message:  fmt.Sprintf("%s changed (%s).", backtickJoin(sorted), kind),
		Action:   fmt.Sprintf("Call out the %s change in the description so maintainers can verify it.", kind),
		Paths:    sorted,
	}
}

// topLevelArea returns the first path segment of p, or rootArea for a
// root-level file (no directory component).
func topLevelArea(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return rootArea
}

// basename returns the final path segment of p.
func basename(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

func isLockfile(p string) bool { return lockfileBasenames[basename(p)] }
func isManifest(p string) bool { return manifestBasenames[basename(p)] }

// isCIRelease reports whether p is part of the built-in CI/release set.
func isCIRelease(p string) bool {
	for _, g := range ciReleaseGlobs {
		if m, ok := globs.Match(g, p); ok && m {
			return true
		}
	}
	return false
}

// isDocs reports whether p is documentation: under docs/ or a Markdown file.
func isDocs(p string) bool {
	if strings.HasPrefix(p, "docs/") {
		return true
	}
	if m, ok := globs.Match("*.md", p); ok && m {
		return true
	}
	return false
}

// isConfigOrCI reports whether p is a configuration or CI file for the
// mixed-concerns rule: either part of the CI/release set or a dotfile at the
// repository root.
func isConfigOrCI(p string) bool {
	if isCIRelease(p) {
		return true
	}
	return isRootDotfile(p)
}

// isRootDotfile reports whether p is a dotfile at the repository root
// (e.g. ".eslintrc", ".gitlab-ci.yml", ".codesteward.yaml").
func isRootDotfile(p string) bool {
	return !strings.Contains(p, "/") && strings.HasPrefix(p, ".")
}

// sectionPresent reports whether text contains a required section: a Markdown
// heading line (starting with "#") or a bold line (starting with "**" or "__")
// that contains the section name, compared case-insensitively.
func sectionPresent(text, section string) bool {
	needle := strings.ToLower(strings.TrimSpace(section))
	if needle == "" {
		// An empty required-section entry cannot be missing.
		return true
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if !isHeadingOrBoldLine(trimmed) {
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), needle) {
			return true
		}
	}
	return false
}

// isHeadingOrBoldLine reports whether an already-trimmed line looks like a
// Markdown heading or a bold-emphasised line.
func isHeadingOrBoldLine(trimmed string) bool {
	if strings.HasPrefix(trimmed, "#") {
		return true
	}
	return strings.HasPrefix(trimmed, "**") || strings.HasPrefix(trimmed, "__")
}

// backtickJoin renders paths as a comma-separated list of backtick-quoted
// entries.
func backtickJoin(paths []string) string {
	parts := make([]string, len(paths))
	for i, p := range paths {
		parts[i] = "`" + p + "`"
	}
	return strings.Join(parts, ", ")
}

// sortedKeys returns the keys of set sorted ascending (nil when empty).
func sortedKeys(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedUnique returns the deduplicated, ascending-sorted elements of in
// (nil when empty).
func sortedUnique(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

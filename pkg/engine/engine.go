// Package engine orchestrates a scan: repo detection, config, diff, ownership,
// tests, and rules into a single *model.Report.
//
// Run wires the internal packages together in the fixed order mandated by the
// CodeSteward engineering contract (CONTRACTS §6.12) and is fully
// deterministic: identical repositories, refs, config, and description always
// produce a byte-identical report. Every non-fatal problem discovered along the
// way (missing config, missing CODEOWNERS, shallow-clone or ref-resolution
// notes, diff warnings, and CODEOWNERS parse warnings) is funnelled into
// Report.Warnings, sorted and de-duplicated.
package engine

import (
	"fmt"
	"sort"

	"github.com/codesteward-ai/codesteward/internal/codeowners"
	"github.com/codesteward-ai/codesteward/internal/config"
	"github.com/codesteward-ai/codesteward/internal/diff"
	"github.com/codesteward-ai/codesteward/internal/git"
	"github.com/codesteward-ai/codesteward/internal/ownership"
	"github.com/codesteward-ai/codesteward/internal/readiness"
	"github.com/codesteward-ai/codesteward/internal/rules"
	"github.com/codesteward-ai/codesteward/internal/tests"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Options configures a scan run.
type Options struct {
	RepoRoot       string
	ConfigPath     string
	Base           string
	Head           string
	Description    string
	DescriptionSet bool // true when --description/--description-file/provider supplied it
	Version        string
}

// Result is the outcome of a scan run.
type Result struct {
	Report     *model.Report
	Config     *config.Config
	ConfigLoad *config.LoadResult
}

// missingCodeownersWarning is emitted verbatim when no CODEOWNERS file is
// discovered for the configured dialect.
const missingCodeownersWarning = "no CODEOWNERS file found"

// Run executes the full scan pipeline and assembles the readiness report.
//
// Pipeline (CONTRACTS §6.12): detect repo -> load config -> resolve refs ->
// collect & classify the diff -> discover/parse CODEOWNERS -> analyze ownership
// -> analyze tests -> apply scope/description/sensitive rules -> build the
// report. All non-fatal issues flow into Report.Warnings (sorted, deduped).
func Run(opts Options) (*Result, error) {
	// 1. Repo detection (CONTRACTS §6.12 pipeline step 1). Always detect the
	// enclosing git repository so that running outside a working tree produces
	// the actionable "not inside a git repository" error (CONTRACTS §6.2), and
	// so every subsequent step uses the canonical repository root. An empty
	// RepoRoot means "detect from the current working directory"; a non-empty
	// RepoRoot (the CLI always passes one, defaulting to ".") is the directory
	// to detect from, not a value to trust blindly.
	detectDir := opts.RepoRoot
	if detectDir == "" {
		detectDir = "."
	}
	info, err := git.DetectRepo(detectDir)
	if err != nil {
		return nil, err
	}
	root := info.Root

	// 2. Configuration. Load merges any discovered file over the defaults and
	// records non-fatal load warnings (missing file, unknown keys, bad globs).
	cfg, loadRes, err := config.Load(root, opts.ConfigPath)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if loadRes != nil {
		warnings = append(warnings, loadRes.Warnings...)
	}

	// 3. Ref resolution. The resolved refs are the ones reported and diffed.
	resolvedBase, resolvedHead, refWarnings, err := git.ResolveRefs(root, opts.Base, opts.Head)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, refWarnings...)

	// 4. Diff collection and classification.
	changed, diffWarnings, err := diff.Collect(root, resolvedBase, resolvedHead)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, diffWarnings...)

	changed = diff.Classify(changed, diff.ClassifyOptions{
		ProductionPaths: cfg.Ownership.ProductionPaths,
		IgnorePaths:     cfg.Ownership.IgnorePaths,
		TestPaths:       cfg.Tests.TestPaths,
		SensitivePaths:  cfg.SensitivePaths,
	})

	// 5. CODEOWNERS discovery and parsing. A missing file is not an error: the
	// matcher stays nil and a warning is recorded.
	dialect := dialectFor(cfg.Ownership.Dialect)
	ownersPath, err := codeowners.Discover(root, dialect)
	if err != nil {
		return nil, err
	}

	var matcher model.OwnerMatcher
	codeownersFound := false
	if ownersPath != "" {
		f, err := codeowners.ParseFile(ownersPath, dialect)
		if err != nil {
			return nil, err
		}
		if f != nil {
			matcher = f
			codeownersFound = true
			for _, w := range f.Warnings {
				warnings = append(warnings, formatParseWarning(w))
			}
		}
	}
	if !codeownersFound {
		warnings = append(warnings, missingCodeownersWarning)
	}

	// 6. Ownership analysis. Enabled only when CODEOWNERS is both configured for
	// use and present on disk.
	ownershipSummary, ownershipFindings := ownership.Analyze(changed, matcher, ownership.Options{
		MaxAreas: cfg.ReviewReadiness.MaxOwnershipAreas,
		Enabled:  cfg.Ownership.UseCodeowners && codeownersFound,
	})

	// 7. Test-expectation analysis.
	testsSummary, testFindings := tests.Analyze(changed, tests.Options{
		RequireFor: cfg.Tests.RequireFor,
		TestPaths:  cfg.Tests.TestPaths,
		Mappings:   cfg.Tests.PathMappings,
		Root:       root,
		Enabled:    true,
	})

	// 8. Scope, description, and sensitive-path rules.
	scopeSummary, scopeFindings := rules.Scope(changed, rules.ScopeOptions{
		MaxFiles: cfg.ReviewReadiness.MaxFilesChanged,
		MaxLines: cfg.ReviewReadiness.MaxLinesChanged,
	})
	descriptionSummary, descriptionFindings := rules.Description(rules.DescriptionOptions{
		Text:               opts.Description,
		Evaluated:          opts.DescriptionSet,
		WarnIfEmpty:        cfg.PRDescription.WarnIfEmpty,
		MinLength:          cfg.PRDescription.MinLength,
		RequiredSections:   cfg.PRDescription.RequiredSections,
		RequireLinkedIssue: cfg.PRDescription.RequireLinkedIssue,
	})
	sensitiveFindings := rules.Sensitive(changed)

	// 9. Aggregate findings. Order here is irrelevant: BuildReport applies the
	// canonical §7.6 sort.
	findings := make([]model.Finding, 0,
		len(ownershipFindings)+len(testFindings)+len(scopeFindings)+
			len(descriptionFindings)+len(sensitiveFindings))
	findings = append(findings, ownershipFindings...)
	findings = append(findings, testFindings...)
	findings = append(findings, scopeFindings...)
	findings = append(findings, descriptionFindings...)
	findings = append(findings, sensitiveFindings...)

	// 10. Assemble the report.
	report := readiness.BuildReport(readiness.BuildInput{
		Version:     opts.Version,
		Base:        resolvedBase,
		Head:        resolvedHead,
		CommentOnly: cfg.Mode.CommentOnly,
		Ownership:   ownershipSummary,
		Tests:       testsSummary,
		Scope:       scopeSummary,
		Description: descriptionSummary,
		Findings:    findings,
		Warnings:    sortDedup(warnings),
	})

	return &Result{
		Report:     report,
		Config:     cfg,
		ConfigLoad: loadRes,
	}, nil
}

// dialectFor maps a config dialect string to a codeowners.Dialect, defaulting
// to auto for unrecognized (or empty) values.
func dialectFor(s string) codeowners.Dialect {
	switch s {
	case "github":
		return codeowners.DialectGitHub
	case "gitlab":
		return codeowners.DialectGitLab
	default:
		return codeowners.DialectAuto
	}
}

// formatParseWarning renders a CODEOWNERS parse warning as a single stable
// warning line, e.g. `CODEOWNERS line 4: invalid pattern "["`.
func formatParseWarning(w codeowners.ParseWarning) string {
	return fmt.Sprintf("CODEOWNERS line %d: %s", w.Line, w.Text)
}

// sortDedup returns the input warnings sorted ascending with duplicates
// removed, so Report.Warnings is deterministic and free of repeats. It returns
// nil when there are no warnings so the JSON omitempty tag drops the field.
func sortDedup(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	sorted := make([]string, len(in))
	copy(sorted, in)
	sort.Strings(sorted)
	out := sorted[:0]
	var last string
	for i, s := range sorted {
		if i > 0 && s == last {
			continue
		}
		out = append(out, s)
		last = s
	}
	return out
}

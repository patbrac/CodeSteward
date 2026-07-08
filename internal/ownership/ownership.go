// Package ownership analyzes CODEOWNERS coverage for changed files and audits
// repository-wide ownership.
package ownership

import (
	"fmt"
	"io/fs"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/internal/globs"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Options configures ownership analysis.
type Options struct {
	MaxAreas int
	Enabled  bool // config ownership.use_codeowners && CODEOWNERS file found
}

// Analyze evaluates ownership coverage across the changed files.
//
// It considers only files where IsProduction && !IsIgnored (production files),
// plus IsSensitive && !IsIgnored files that are not production files for the
// CS-OWN-004 sensitive-no-owner rule. matcher may be nil when no CODEOWNERS
// file exists; in that case every relevant file is treated as unowned.
//
// When ownership is disabled (Enabled=false) or there are no relevant files at
// all, the summary state is not_evaluated and no findings are produced.
//
// Ownership areas are the distinct matched CODEOWNERS patterns among production
// files; each unowned production file contributes the synthetic area
// "unowned:<top-level-dir>". If the number of areas exceeds MaxAreas the
// analysis emits CS-OWN-003.
//
// State precedence: missing (any production file unowned) > partial (any
// production file covered only by fallback ownership) > complete (specific and
// broad ownership both count as covered).
func Analyze(files []model.ChangedFile, matcher model.OwnerMatcher, opts Options) (model.OwnershipSummary, []model.Finding) {
	prod := filterProduction(files)
	sens := filterSensitive(files)

	if !opts.Enabled || (len(prod) == 0 && len(sens) == 0) {
		return model.OwnershipSummary{
			State:    model.OwnershipNotEvaluated,
			MaxAreas: opts.MaxAreas,
		}, nil
	}

	var (
		own001 []model.Finding // CS-OWN-001: production file with no owner
		own002 []model.Finding // CS-OWN-002: production file with fallback-only owner
		own004 []model.Finding // CS-OWN-004: sensitive file with no owner

		fileOwnerships  []model.FileOwnership
		areas           = map[string]struct{}{}
		anyUnowned      bool
		anyFallbackOnly bool
	)

	for _, f := range prod {
		m := matchPath(matcher, f.Path)
		fileOwnerships = append(fileOwnerships, model.FileOwnership{
			Path:    f.Path,
			Owners:  m.Owners,
			Pattern: m.Pattern,
			Class:   m.Class,
		})
		if !m.Found {
			anyUnowned = true
			areas["unowned:"+topLevel(f.Path)] = struct{}{}
			own001 = append(own001, finding001(f.Path))
			continue
		}
		areas[m.Pattern] = struct{}{}
		if m.Class == model.MatchFallback {
			anyFallbackOnly = true
			own002 = append(own002, finding002(f.Path, m.Pattern, m.Owners))
		}
	}

	var own003 []model.Finding
	if len(areas) > opts.MaxAreas {
		own003 = append(own003, finding003(len(areas), opts.MaxAreas))
	}

	for _, f := range sens {
		m := matchPath(matcher, f.Path)
		if !m.Found {
			own004 = append(own004, finding004(f.Path))
		}
	}

	state := ownershipState(len(prod), anyUnowned, anyFallbackOnly)

	findings := make([]model.Finding, 0, len(own001)+len(own002)+len(own003)+len(own004))
	findings = append(findings, own001...)
	findings = append(findings, own002...)
	findings = append(findings, own003...)
	findings = append(findings, own004...)
	if len(findings) == 0 {
		findings = nil
	}

	return model.OwnershipSummary{
		State:        state,
		AreasTouched: len(areas),
		MaxAreas:     opts.MaxAreas,
		Files:        fileOwnerships,
	}, findings
}

// ownershipState maps production-file coverage to a summary state. With no
// production files the state is not_evaluated even when sensitive findings were
// produced, because there is no production coverage to describe.
func ownershipState(prodCount int, anyUnowned, anyFallbackOnly bool) model.OwnershipState {
	switch {
	case prodCount == 0:
		return model.OwnershipNotEvaluated
	case anyUnowned:
		return model.OwnershipMissing
	case anyFallbackOnly:
		return model.OwnershipPartial
	default:
		return model.OwnershipComplete
	}
}

// filterProduction returns the production files (IsProduction && !IsIgnored)
// sorted by path.
func filterProduction(files []model.ChangedFile) []model.ChangedFile {
	var out []model.ChangedFile
	for _, f := range files {
		if f.IsProduction && !f.IsIgnored {
			out = append(out, f)
		}
	}
	sortByPath(out)
	return out
}

// filterSensitive returns the sensitive files relevant to CS-OWN-004: sensitive,
// not ignored, and not already handled as production files (production files
// with no owner are reported by CS-OWN-001 instead). Sorted by path.
func filterSensitive(files []model.ChangedFile) []model.ChangedFile {
	var out []model.ChangedFile
	for _, f := range files {
		if f.IsSensitive && !f.IsIgnored && !f.IsProduction {
			out = append(out, f)
		}
	}
	sortByPath(out)
	return out
}

func sortByPath(files []model.ChangedFile) {
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
}

// matchPath resolves ownership for a single path, treating a nil matcher as
// "no CODEOWNERS present" so every path is unowned.
func matchPath(matcher model.OwnerMatcher, p string) model.OwnershipMatch {
	if matcher == nil {
		return model.OwnershipMatch{Found: false, Class: model.MatchMissing}
	}
	return matcher.Match(p)
}

func finding001(path string) model.Finding {
	return model.Finding{
		RuleID:   model.RuleOwnNoOwner,
		Severity: model.SeverityActionRequired,
		Message:  "No owner found for `" + path + "`.",
		Action:   "Add specific ownership for `" + path + "` or ask a maintainer to route this area.",
		Paths:    []string{path},
	}
}

func finding002(path, pattern string, owners []string) model.Finding {
	coverage := pattern
	if len(owners) > 0 {
		coverage = pattern + " " + strings.Join(owners, " ")
	}
	return model.Finding{
		RuleID:   model.RuleOwnFallbackOnly,
		Severity: model.SeverityWarning,
		Message:  "`" + path + "` is covered only by fallback ownership: `" + coverage + "`.",
		Action:   "Add specific ownership for `" + dirGlob(path) + "` or ask a maintainer to route this area.",
		Paths:    []string{path},
	}
}

func finding003(n, max int) model.Finding {
	return model.Finding{
		RuleID:   model.RuleOwnTooManyAreas,
		Severity: model.SeverityWarning,
		Message:  fmt.Sprintf("This change touches %d ownership areas, above the configured limit of %d.", n, max),
		Action:   "Consider splitting this change so each PR touches fewer ownership areas.",
	}
}

func finding004(path string) model.Finding {
	return model.Finding{
		RuleID:   model.RuleOwnSensitiveNoOwn,
		Severity: model.SeverityActionRequired,
		Message:  "No owner found for sensitive file `" + path + "`.",
		Action:   "Add specific ownership for `" + path + "` or ask a maintainer to route this area.",
		Paths:    []string{path},
	}
}

// topLevel returns the top-level area of a repo-relative path: the first path
// segment, or "(root)" for a file at the repository root (§7.3/§7.5).
func topLevel(p string) string {
	if i := strings.IndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return "(root)"
}

// dirGlob returns the immediate parent directory of p as a recursive glob, used
// in the CS-OWN-002 action suggestion (e.g. "src/runtime/cache.ts" ->
// "src/runtime/**").
func dirGlob(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i] + "/**"
	}
	return "**"
}

// AuditEntry is one production file's ownership classification.
type AuditEntry struct {
	Path    string
	Owners  []string
	Pattern string
	Class   model.OwnershipMatchClass
}

// AuditResult is the repository-wide ownership audit.
type AuditResult struct {
	Total    int
	Specific int
	Broad    int
	Fallback int
	Unowned  int
	Entries  []AuditEntry // sorted by Path; only production files
}

// Audit walks the repository work tree and classifies every production file.
//
// It prefers `git ls-files` (run in root) to enumerate tracked files and falls
// back to a filesystem walk (skipping .git) when git is unavailable or root is
// not a git repository. Every file matching productionPaths and not matching
// ignorePaths is classified via matcher; entries are sorted by path and the
// per-class counts are aggregated.
func Audit(root string, matcher model.OwnerMatcher, productionPaths, ignorePaths []string) (*AuditResult, error) {
	files, ok := gitFiles(root)
	if !ok {
		var err error
		files, err = walkFiles(root)
		if err != nil {
			return nil, fmt.Errorf("codesteward: ownership audit walk failed: %w", err)
		}
	}

	result := &AuditResult{}
	for _, p := range files {
		if _, matched := globs.MatchAny(productionPaths, p); !matched {
			continue
		}
		if _, matched := globs.MatchAny(ignorePaths, p); matched {
			continue
		}
		m := matchPath(matcher, p)
		entry := AuditEntry{Path: p, Owners: m.Owners, Pattern: m.Pattern, Class: m.Class}
		switch {
		case !m.Found:
			result.Unowned++
			entry.Class = model.MatchMissing
			entry.Owners = nil
		case m.Class == model.MatchSpecific:
			result.Specific++
		case m.Class == model.MatchBroad:
			result.Broad++
		case m.Class == model.MatchFallback:
			result.Fallback++
		default:
			result.Unowned++
			entry.Class = model.MatchMissing
			entry.Owners = nil
		}
		result.Entries = append(result.Entries, entry)
	}

	sort.Slice(result.Entries, func(i, j int) bool {
		return result.Entries[i].Path < result.Entries[j].Path
	})
	result.Total = len(result.Entries)
	return result, nil
}

// gitFiles lists tracked files relative to root via `git ls-files -z`. The
// boolean is false (and the caller should fall back to a filesystem walk) when
// git is unavailable or root is not a git repository.
func gitFiles(root string) ([]string, bool) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	var files []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p == "" {
			continue
		}
		files = append(files, filepath.ToSlash(p))
	}
	return files, true
}

// walkFiles walks root recursively, skipping any .git directory, returning
// repo-relative slash-separated paths for every regular file.
func walkFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// Package diff collects and classifies changed files between two refs. It
// shells out to git with NUL-separated (-z) output so paths with spaces or
// other awkward characters survive intact, and it merges the --numstat and
// --name-status views into a single, deterministically sorted result.
package diff

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/codesteward-ai/codesteward/internal/globs"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Collect gathers the changed files between base and head using the three-dot
// range base...head (changes on the head side since the merge base). It runs
// `git diff --find-renames --find-copies -z` for both --name-status (statuses
// and rename/copy path pairs) and --numstat (line counts and binary markers),
// then merges the two views keyed by new path. Results are sorted by Path and
// all paths are slash-normalized.
func Collect(root, base, head string) ([]model.ChangedFile, []string, error) {
	var warnings []string
	spec := base + "..." + head

	nsOut, err := runGitRaw(root, "diff", "-z", "--find-renames", "--find-copies", "--name-status", spec)
	if err != nil {
		return nil, warnings, wrapDiffErr(err)
	}
	numOut, err := runGitRaw(root, "diff", "-z", "--find-renames", "--find-copies", "--numstat", spec)
	if err != nil {
		return nil, warnings, wrapDiffErr(err)
	}

	files := map[string]*model.ChangedFile{}
	if err := parseNameStatus(nsOut, files); err != nil {
		return nil, warnings, err
	}
	if err := parseNumstat(numOut, files); err != nil {
		return nil, warnings, err
	}

	paths := make([]string, 0, len(files))
	for p := range files {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	result := make([]model.ChangedFile, 0, len(paths))
	for _, p := range paths {
		result = append(result, *files[p])
	}
	return result, warnings, nil
}

// ClassifyOptions configures file classification.
type ClassifyOptions struct {
	ProductionPaths []string
	IgnorePaths     []string
	TestPaths       []string
	SensitivePaths  []string
}

// Classify sets IsTest, IsProduction, IsSensitive, and IsIgnored on each file
// and returns a new slice in the same order. Precedence: IsIgnored is
// independent; a file matching TestPaths is IsTest and never IsProduction;
// production = matches ProductionPaths and is neither test nor ignored;
// sensitive = a built-in sensitive set (§4) OR a configured SensitivePaths
// glob. Classification uses the file's new Path.
func Classify(files []model.ChangedFile, opts ClassifyOptions) []model.ChangedFile {
	out := make([]model.ChangedFile, len(files))
	for i, f := range files {
		p := f.Path
		f.IsIgnored = matchesAny(opts.IgnorePaths, p)
		f.IsTest = matchesAny(opts.TestPaths, p)
		f.IsProduction = matchesAny(opts.ProductionPaths, p) && !f.IsTest && !f.IsIgnored
		f.IsSensitive = builtinSensitive(p) || matchesAny(opts.SensitivePaths, p)
		out[i] = f
	}
	return out
}

// matchesAny reports whether any pattern matches path.
func matchesAny(patterns []string, path string) bool {
	_, ok := globs.MatchAny(patterns, path)
	return ok
}

// Built-in sensitive sets from CONTRACTS §4 (case-sensitive basenames and
// path globs). The finer per-rule classification (lockfile vs manifest vs CI)
// lives in internal/rules; here only the IsSensitive boolean is derived.
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

// builtinSensitive reports whether path matches a built-in sensitive set.
func builtinSensitive(p string) bool {
	base := p
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		base = p[i+1:]
	}
	if lockfileBasenames[base] || manifestBasenames[base] {
		return true
	}
	for _, g := range ciReleaseGlobs {
		if m, ok := globs.Match(g, p); ok && m {
			return true
		}
	}
	return false
}

// parseNameStatus parses `git diff -z --name-status` output into files, keyed
// by new path. Each record is a NUL-terminated status field followed by one
// path field, or (for renames/copies) two path fields (old, then new).
func parseNameStatus(out []byte, files map[string]*model.ChangedFile) error {
	toks := splitNUL(out)
	for i := 0; i < len(toks); {
		status := toks[i]
		if status == "" {
			break
		}
		i++
		if len(status) == 0 {
			return fmt.Errorf("codesteward: malformed git name-status output")
		}
		switch status[0] {
		case 'R', 'C':
			if i+1 >= len(toks) {
				return fmt.Errorf("codesteward: truncated rename/copy record in git name-status output")
			}
			oldPath := toSlash(toks[i])
			newPath := toSlash(toks[i+1])
			i += 2
			f := ensure(files, newPath)
			f.OldPath = oldPath
			if status[0] == 'R' {
				f.Status = "renamed"
			} else {
				f.Status = "copied"
			}
		default:
			if i >= len(toks) {
				return fmt.Errorf("codesteward: truncated record in git name-status output")
			}
			path := toSlash(toks[i])
			i++
			f := ensure(files, path)
			f.Status = statusFromLetter(status[0])
		}
	}
	return nil
}

// parseNumstat parses `git diff -z --numstat` output into files, keyed by new
// path. A normal record is "<add>\t<del>\t<path>"; a rename/copy record is
// "<add>\t<del>\t" followed by two path fields (old, then new). Binary files
// use "-" for both counts.
func parseNumstat(out []byte, files map[string]*model.ChangedFile) error {
	toks := splitNUL(out)
	for i := 0; i < len(toks); {
		tok := toks[i]
		if tok == "" {
			break
		}
		fields := strings.SplitN(tok, "\t", 3)
		if len(fields) < 3 {
			return fmt.Errorf("codesteward: malformed git numstat record %q", tok)
		}
		addStr, delStr, path := fields[0], fields[1], fields[2]
		var oldPath string
		if path == "" {
			// Rename/copy form: two trailing NUL-separated path fields.
			if i+2 >= len(toks) {
				return fmt.Errorf("codesteward: truncated rename/copy record in git numstat output")
			}
			oldPath = toSlash(toks[i+1])
			path = toSlash(toks[i+2])
			i += 3
		} else {
			path = toSlash(path)
			i++
		}

		f := ensure(files, path)
		if oldPath != "" && f.OldPath == "" {
			f.OldPath = oldPath
		}
		if f.Status == "" {
			// Defensive: numstat saw a path name-status did not.
			if oldPath != "" {
				f.Status = "renamed"
			} else {
				f.Status = "modified"
			}
		}
		if addStr == "-" || delStr == "-" {
			f.IsBinary = true
			f.Additions = 0
			f.Deletions = 0
			continue
		}
		add, err := strconv.Atoi(addStr)
		if err != nil {
			return fmt.Errorf("codesteward: invalid addition count %q in git numstat output: %w", addStr, err)
		}
		del, err := strconv.Atoi(delStr)
		if err != nil {
			return fmt.Errorf("codesteward: invalid deletion count %q in git numstat output: %w", delStr, err)
		}
		f.Additions = add
		f.Deletions = del
	}
	return nil
}

// ensure returns the ChangedFile for path, creating it if absent.
func ensure(files map[string]*model.ChangedFile, path string) *model.ChangedFile {
	if f, ok := files[path]; ok {
		return f
	}
	f := &model.ChangedFile{Path: path}
	files[path] = f
	return f
}

// statusFromLetter maps a git status letter to a contracted status string.
func statusFromLetter(c byte) string {
	switch c {
	case 'A':
		return "added"
	case 'D':
		return "deleted"
	case 'M':
		return "modified"
	default:
		// T (type change), U (unmerged), and anything else are treated as
		// modifications for review-readiness purposes.
		return "modified"
	}
}

// splitNUL splits raw git -z output on NUL. The trailing empty element after
// the final NUL is retained and treated as a terminator by the parsers.
func splitNUL(out []byte) []string {
	return strings.Split(string(out), "\x00")
}

// toSlash normalizes a git-reported path to slash-separated, no leading "./".
func toSlash(p string) string {
	return strings.TrimPrefix(filepath.ToSlash(p), "./")
}

// wrapDiffErr wraps a git diff failure with actionable shallow-clone guidance.
func wrapDiffErr(err error) error {
	return fmt.Errorf("codesteward: failed to compute diff; in a shallow clone fetch full history with fetch-depth: 0 or `git fetch --unshallow`: %w", err)
}

// runGitRaw runs git in root and returns raw stdout bytes (untrimmed, so NUL
// separators are preserved) or an error including git's stderr.
func runGitRaw(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), msg, err)
		}
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.Bytes(), nil
}

// Package git detects the repository and resolves base/head refs by shelling
// out to the git binary. It never mutates the repository and produces no
// timestamps, randomness, or environment-dependent text in its results, so
// callers can rely on deterministic behavior for identical inputs.
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RepoInfo describes the detected git repository.
type RepoInfo struct {
	Root      string
	Branch    string
	IsShallow bool
}

// DetectRepo detects the repository root and current branch for dir. It runs
// `git rev-parse --show-toplevel` for the root, `--abbrev-ref HEAD` for the
// branch, and `--is-shallow-repository` for the shallow flag. It returns an
// actionable error when dir is not inside a git working tree.
func DetectRepo(dir string) (*RepoInfo, error) {
	root, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("codesteward: %q is not inside a git repository; run CodeSteward from a git working tree or pass --repo-root: %w", dir, err)
	}
	info := &RepoInfo{Root: root}
	if branch, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		info.Branch = branch
	}
	if out, err := runGit(dir, "rev-parse", "--is-shallow-repository"); err == nil {
		info.IsShallow = strings.TrimSpace(out) == "true"
	}
	return info, nil
}

// ResolveRefs resolves the base and head refs to be diffed. head defaults to
// "HEAD". The base resolution order is:
//
//	explicit base flag > env GITHUB_BASE_REF >
//	env CI_MERGE_REQUEST_TARGET_BRANCH_NAME > origin/HEAD symbolic ref >
//	"main" if it exists > "master" if it exists > error.
//
// If an explicit or environment-supplied base name X is not a valid local ref
// but origin/X is, origin/X is used. Both refs are verified with
// `git rev-parse --verify`, and a merge base is required for the three-dot
// diff. When resolution fails in a shallow clone the returned error explains
// how to fetch full history (fetch-depth: 0 / git fetch --unshallow).
func ResolveRefs(root, base, head string) (resolvedBase, resolvedHead string, warnings []string, err error) {
	return resolveRefs(root, base, head, os.Getenv)
}

// resolveRefs is the testable core of ResolveRefs with an injectable getenv.
func resolveRefs(root, base, head string, getenv func(string) string) (string, string, []string, error) {
	var warnings []string
	if head == "" {
		head = "HEAD"
	}
	shallow := isShallow(root)
	if shallow {
		warnings = append(warnings, "repository is a shallow clone; results may be incomplete. Fetch full history with fetch-depth: 0 or `git fetch --unshallow`.")
	}

	// Resolve and verify head.
	if !verifyRef(root, head) {
		return "", "", warnings, refResolveError(head, shallow)
	}
	resolvedHead := head

	// Determine the base candidate name from the contracted precedence.
	candidate := ""
	switch {
	case base != "":
		candidate = base
	case getenv("GITHUB_BASE_REF") != "":
		candidate = getenv("GITHUB_BASE_REF")
	case getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME") != "":
		candidate = getenv("CI_MERGE_REQUEST_TARGET_BRANCH_NAME")
	}

	var resolvedBase string
	if candidate != "" {
		switch {
		case verifyRef(root, candidate):
			resolvedBase = candidate
		case verifyRef(root, "origin/"+candidate):
			resolvedBase = "origin/" + candidate
			warnings = append(warnings, fmt.Sprintf("base ref %q not found locally; using %q instead", candidate, resolvedBase))
		default:
			return "", "", warnings, refResolveError(candidate, shallow)
		}
	} else {
		resolvedBase = discoverDefaultBase(root)
		if resolvedBase == "" {
			return "", "", warnings, fmt.Errorf("codesteward: could not determine a base ref; pass --base explicitly (no origin/HEAD, main, or master was found)")
		}
	}

	// The three-dot diff requires a common ancestor.
	if !hasMergeBase(root, resolvedBase, resolvedHead) {
		return "", "", warnings, mergeBaseError(resolvedBase, resolvedHead, shallow)
	}

	return resolvedBase, resolvedHead, warnings, nil
}

// discoverDefaultBase applies the no-flag/no-env portion of the base
// resolution order: origin/HEAD symbolic ref, then "main", then "master".
// It returns "" when none of them resolve to a commit.
func discoverDefaultBase(root string) string {
	if out, err := runGit(root, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD"); err == nil {
		ref := strings.TrimSpace(out) // e.g. refs/remotes/origin/main
		if name := strings.TrimPrefix(ref, "refs/remotes/"); name != ref && verifyRef(root, name) {
			return name // e.g. origin/main
		}
	}
	if verifyRef(root, "main") {
		return "main"
	}
	if verifyRef(root, "master") {
		return "master"
	}
	return ""
}

// verifyRef reports whether ref resolves to a commit in the repository at root.
func verifyRef(root, ref string) bool {
	_, err := runGit(root, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil
}

// hasMergeBase reports whether a and b share a common ancestor.
func hasMergeBase(root, a, b string) bool {
	_, err := runGit(root, "merge-base", a, b)
	return err == nil
}

// isShallow reports whether the repository at root is a shallow clone.
func isShallow(root string) bool {
	out, err := runGit(root, "rev-parse", "--is-shallow-repository")
	return err == nil && strings.TrimSpace(out) == "true"
}

// refResolveError builds an actionable error for a ref that would not resolve,
// with shallow-clone remediation when relevant.
func refResolveError(ref string, shallow bool) error {
	if shallow {
		return fmt.Errorf("codesteward: ref %q could not be resolved in a shallow clone. Fetch full history: set fetch-depth: 0 on your checkout step, or run `git fetch --unshallow`, then retry", ref)
	}
	return fmt.Errorf("codesteward: ref %q could not be resolved. Ensure it exists locally (try `git fetch origin %s`) or pass a valid --base/--head", ref, ref)
}

// mergeBaseError builds an actionable error when no common history exists,
// with shallow-clone remediation when relevant.
func mergeBaseError(base, head string, shallow bool) error {
	if shallow {
		return fmt.Errorf("codesteward: no common history between %q and %q in a shallow clone. Fetch full history: set fetch-depth: 0 on your checkout step, or run `git fetch --unshallow`, then retry", base, head)
	}
	return fmt.Errorf("codesteward: no common history (merge base) between %q and %q; verify the base and head refs share history", base, head)
}

// runGit runs git in dir, returning trimmed stdout or an error that includes
// git's stderr. It disables interactive prompts for hermetic behavior.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return "", fmt.Errorf("git %s: %s: %w", strings.Join(args, " "), msg, err)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

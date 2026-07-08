package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// requireGit skips the test when the git binary is not available.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
}

// gitCmd runs git in dir with a hermetic identity and fails the test on error.
func gitCmd(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_CONFIG_NOSYSTEM=1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, rel, data string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitFile(t *testing.T, repo, rel, data, msg string) {
	t.Helper()
	writeFile(t, repo, rel, data)
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-m", msg)
}

// initRepo creates a repo on branch defaultBranch with one commit.
func initRepo(t *testing.T, defaultBranch string) string {
	t.Helper()
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-b", defaultBranch)
	commitFile(t, repo, "a.txt", "one\n", "first")
	return repo
}

// noEnv is an empty getenv used to isolate tests from the ambient environment.
func noEnv(string) string { return "" }

func eval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDetectRepo(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "main")

	info, err := DetectRepo(repo)
	if err != nil {
		t.Fatalf("DetectRepo: %v", err)
	}
	if eval(t, info.Root) != eval(t, repo) {
		t.Errorf("Root = %q, want %q", info.Root, repo)
	}
	if info.Branch != "main" {
		t.Errorf("Branch = %q, want main", info.Branch)
	}
	if info.IsShallow {
		t.Errorf("IsShallow = true, want false for a normal repo")
	}

	// Detection from a subdirectory returns the same root.
	sub := filepath.Join(repo, "pkg", "inner")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	info2, err := DetectRepo(sub)
	if err != nil {
		t.Fatalf("DetectRepo(sub): %v", err)
	}
	if eval(t, info2.Root) != eval(t, repo) {
		t.Errorf("Root from subdir = %q, want %q", info2.Root, repo)
	}
}

func TestDetectRepoNotARepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir()
	if _, err := DetectRepo(dir); err == nil {
		t.Errorf("expected error for non-repo dir, got nil")
	}
}

func TestResolveRefsExplicitExported(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "main")
	gitCmd(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "b.txt", "two\n", "second")

	// Explicit base short-circuits before any env lookup, so the exported
	// entrypoint is safe to test regardless of the ambient environment.
	base, head, warnings, err := ResolveRefs(repo, "main", "feature")
	if err != nil {
		t.Fatalf("ResolveRefs: %v", err)
	}
	if base != "main" || head != "feature" {
		t.Errorf("got base=%q head=%q, want main/feature", base, head)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestResolveRefsHeadDefault(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "main")
	base, head, _, err := resolveRefs(repo, "main", "", noEnv)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if base != "main" || head != "HEAD" {
		t.Errorf("got base=%q head=%q, want main/HEAD", base, head)
	}
}

func TestResolveRefsEnvPrecedence(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "main")
	gitCmd(t, repo, "branch", "target")
	gitCmd(t, repo, "checkout", "-b", "feature")
	commitFile(t, repo, "c.txt", "three\n", "third")

	// GITHUB_BASE_REF is consulted when no explicit base is given.
	env := func(k string) string {
		if k == "GITHUB_BASE_REF" {
			return "main"
		}
		return ""
	}
	base, _, _, err := resolveRefs(repo, "", "feature", env)
	if err != nil {
		t.Fatalf("resolveRefs(GITHUB_BASE_REF): %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want main (from GITHUB_BASE_REF)", base)
	}

	// GITHUB_BASE_REF wins over CI_MERGE_REQUEST_TARGET_BRANCH_NAME.
	env2 := func(k string) string {
		switch k {
		case "GITHUB_BASE_REF":
			return "main"
		case "CI_MERGE_REQUEST_TARGET_BRANCH_NAME":
			return "target"
		}
		return ""
	}
	base, _, _, err = resolveRefs(repo, "", "feature", env2)
	if err != nil {
		t.Fatalf("resolveRefs(both env): %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want main (GITHUB_BASE_REF precedence)", base)
	}

	// CI variable is used when GITHUB_BASE_REF is absent.
	env3 := func(k string) string {
		if k == "CI_MERGE_REQUEST_TARGET_BRANCH_NAME" {
			return "target"
		}
		return ""
	}
	base, _, _, err = resolveRefs(repo, "", "feature", env3)
	if err != nil {
		t.Fatalf("resolveRefs(CI var): %v", err)
	}
	if base != "target" {
		t.Errorf("base = %q, want target (from CI var)", base)
	}
}

func TestResolveRefsDefaultMain(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "main")
	base, head, _, err := resolveRefs(repo, "", "", noEnv)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if base != "main" || head != "HEAD" {
		t.Errorf("got base=%q head=%q, want main/HEAD", base, head)
	}
}

func TestResolveRefsDefaultMaster(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "master")
	base, _, _, err := resolveRefs(repo, "", "", noEnv)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if base != "master" {
		t.Errorf("base = %q, want master", base)
	}
}

func TestResolveRefsNoBaseError(t *testing.T) {
	requireGit(t)
	repo := initRepo(t, "trunk") // neither main nor master, no remote
	_, _, _, err := resolveRefs(repo, "", "", noEnv)
	if err == nil {
		t.Fatalf("expected error when no base can be determined")
	}
	if !strings.Contains(err.Error(), "--base") {
		t.Errorf("error should suggest --base, got: %v", err)
	}
}

func TestResolveRefsOriginFallback(t *testing.T) {
	requireGit(t)

	// Origin with main and a divergent release branch sharing history.
	origin := t.TempDir()
	gitCmd(t, origin, "init", "-b", "main")
	commitFile(t, origin, "a.txt", "one\n", "first")
	gitCmd(t, origin, "checkout", "-b", "release")
	commitFile(t, origin, "rel.txt", "rel\n", "release work")
	gitCmd(t, origin, "checkout", "main")
	commitFile(t, origin, "m.txt", "main work\n", "main work")

	// Full clone: has remote-tracking origin/release but no local release.
	clone := t.TempDir()
	gitCmd(t, clone, "clone", "file://"+origin, ".")

	base, head, warnings, err := resolveRefs(clone, "release", "HEAD", noEnv)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	if base != "origin/release" {
		t.Errorf("base = %q, want origin/release", base)
	}
	if head != "HEAD" {
		t.Errorf("head = %q, want HEAD", head)
	}
	foundWarn := false
	for _, w := range warnings {
		if strings.Contains(w, "release") && strings.Contains(w, "origin/release") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected an origin-fallback warning, got: %v", warnings)
	}
}

func TestResolveRefsShallowError(t *testing.T) {
	requireGit(t)

	origin := t.TempDir()
	gitCmd(t, origin, "init", "-b", "main")
	commitFile(t, origin, "a.txt", "one\n", "first")
	oldSHA := gitCmd(t, origin, "rev-parse", "HEAD")
	commitFile(t, origin, "b.txt", "two\n", "second")

	// Depth-1 clone: only the tip commit is present; oldSHA is unreachable.
	clone := t.TempDir()
	gitCmd(t, clone, "clone", "--depth=1", "file://"+origin, ".")

	if !isShallow(clone) {
		t.Fatalf("expected a shallow clone")
	}

	_, _, _, err := resolveRefs(clone, oldSHA, "HEAD", noEnv)
	if err == nil {
		t.Fatalf("expected an error resolving an unreachable ref in a shallow clone")
	}
	msg := err.Error()
	if !strings.Contains(msg, "fetch-depth: 0") || !strings.Contains(msg, "git fetch --unshallow") {
		t.Errorf("shallow error is not actionable: %v", err)
	}
}

func TestResolveRefsShallowWarning(t *testing.T) {
	requireGit(t)

	origin := t.TempDir()
	gitCmd(t, origin, "init", "-b", "main")
	commitFile(t, origin, "a.txt", "one\n", "first")
	commitFile(t, origin, "b.txt", "two\n", "second")

	clone := t.TempDir()
	gitCmd(t, clone, "clone", "--depth=1", "file://"+origin, ".")

	// origin/HEAD resolves to the single fetched commit, so resolution
	// succeeds but a shallow warning is surfaced.
	_, _, warnings, err := resolveRefs(clone, "", "HEAD", noEnv)
	if err != nil {
		t.Fatalf("resolveRefs: %v", err)
	}
	shallowWarned := false
	for _, w := range warnings {
		if strings.Contains(w, "shallow") {
			shallowWarned = true
		}
	}
	if !shallowWarned {
		t.Errorf("expected a shallow-clone warning, got: %v", warnings)
	}
}

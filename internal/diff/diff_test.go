package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
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

// writeFile writes data to a slash-relative path under dir, creating parents.
func writeFile(t *testing.T, dir, rel string, data []byte) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// buildFixtureRepo constructs a repo where the head branch differs from the
// base branch in every status class (added/modified/deleted/renamed/binary,
// plus a filename with spaces), and where the base branch has a commit made
// AFTER the merge base that must not appear in a three-dot diff. It returns
// the repo root.
func buildFixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-b", "main")

	// Base commit (merge base).
	writeFile(t, repo, "a.ts", []byte("l1\nl2\nl3\nl4\nl5\nl6\n"))
	writeFile(t, repo, "b.ts", []byte("delete me\n"))
	writeFile(t, repo, "c.ts", []byte("c1\nc2\nc3\nc4\nc5\nc6\nc7\nc8\n"))
	writeFile(t, repo, "README.md", []byte("readme\n"))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-m", "base")

	// Feature branch with every kind of change.
	gitCmd(t, repo, "checkout", "-b", "feature")
	writeFile(t, repo, "a.ts", []byte("l1\nl2\nl3\nl4\nl5\nl6\nl7\n")) // modified (+1)
	gitCmd(t, repo, "rm", "b.ts")                                      // deleted
	gitCmd(t, repo, "mv", "c.ts", "d.ts")                              // renamed
	writeFile(t, repo, "d.ts", []byte("C1\nc2\nc3\nc4\nc5\nc6\nc7\nc8\n"))
	writeFile(t, repo, "added.ts", []byte("brand new\n"))   // added
	writeFile(t, repo, "with space.ts", []byte("spaced\n")) // added, spaced name
	writeFile(t, repo, "bin.png", []byte{0x89, 'P', 'N', 'G', 0x00, 0x01, 0xFF, 0xFE, 0x00, 'x'})
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-m", "feature changes")

	// A commit on base AFTER the merge base; must NOT appear in base...feature.
	gitCmd(t, repo, "checkout", "main")
	writeFile(t, repo, "only-on-main.ts", []byte("later\n"))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-m", "post-branch base change")

	return repo
}

func toMap(files []model.ChangedFile) map[string]model.ChangedFile {
	m := map[string]model.ChangedFile{}
	for _, f := range files {
		m[f.Path] = f
	}
	return m
}

func TestCollect(t *testing.T) {
	requireGit(t)
	repo := buildFixtureRepo(t)

	files, warnings, err := Collect(repo, "main", "feature")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Sorted by Path.
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	if !sort.StringsAreSorted(paths) {
		t.Errorf("files not sorted by Path: %v", paths)
	}

	m := toMap(files)

	// Three-dot semantics: the post-branch base change must be excluded.
	if _, ok := m["only-on-main.ts"]; ok {
		t.Errorf("three-dot diff wrongly included a base-side commit: only-on-main.ts")
	}
	// The rename source and unchanged base file must not appear on their own.
	if _, ok := m["c.ts"]; ok {
		t.Errorf("rename source c.ts should not appear as its own entry")
	}
	if _, ok := m["README.md"]; ok {
		t.Errorf("unchanged README.md should not appear")
	}

	wantPaths := []string{"a.ts", "added.ts", "b.ts", "bin.png", "d.ts", "with space.ts"}
	gotPaths := make([]string, 0, len(m))
	for p := range m {
		gotPaths = append(gotPaths, p)
	}
	sort.Strings(gotPaths)
	if !reflect.DeepEqual(gotPaths, wantPaths) {
		t.Fatalf("changed paths = %v, want %v", gotPaths, wantPaths)
	}

	if f := m["added.ts"]; f.Status != "added" || f.IsBinary || f.Additions == 0 {
		t.Errorf("added.ts = %+v, want added non-binary with additions>0", f)
	}
	if f := m["a.ts"]; f.Status != "modified" || f.Additions == 0 {
		t.Errorf("a.ts = %+v, want modified with additions>0", f)
	}
	if f := m["b.ts"]; f.Status != "deleted" {
		t.Errorf("b.ts = %+v, want deleted", f)
	}
	if f := m["with space.ts"]; f.Status != "added" {
		t.Errorf("with space.ts = %+v, want added", f)
	}
	if f := m["bin.png"]; f.Status != "added" || !f.IsBinary || f.Additions != 0 || f.Deletions != 0 {
		t.Errorf("bin.png = %+v, want added binary with zero counts", f)
	}
	if f := m["d.ts"]; f.Status != "renamed" || f.OldPath != "c.ts" {
		t.Errorf("d.ts = %+v, want renamed from c.ts", f)
	} else if f.Additions == 0 && f.Deletions == 0 {
		t.Errorf("d.ts renamed with edit should have nonzero counts, got %+v", f)
	}
}

func TestCollectDeterministic(t *testing.T) {
	requireGit(t)
	repo := buildFixtureRepo(t)

	a, _, err := Collect(repo, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Collect(repo, "main", "feature")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Collect is not deterministic:\n%+v\n%+v", a, b)
	}
}

func TestCollectEmptyRange(t *testing.T) {
	requireGit(t)
	repo := t.TempDir()
	gitCmd(t, repo, "init", "-b", "main")
	writeFile(t, repo, "a.ts", []byte("x\n"))
	gitCmd(t, repo, "add", "-A")
	gitCmd(t, repo, "commit", "-m", "one")

	files, _, err := Collect(repo, "HEAD", "HEAD")
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected no changes for HEAD...HEAD, got %v", files)
	}
}

func TestClassify(t *testing.T) {
	opts := ClassifyOptions{
		ProductionPaths: []string{"src/**", "lib/**"},
		IgnorePaths:     []string{"docs/**", "README.md", "src/generated/**"},
		TestPaths:       []string{"tests/**", "**/*.test.*", "**/*.spec.*"},
		SensitivePaths:  []string{"config/*.yml"},
	}

	tests := []struct {
		path                                 string
		test, production, sensitive, ignored bool
	}{
		{"src/foo.ts", false, true, false, false},
		{"lib/bar.ts", false, true, false, false},
		// test beats production
		{"src/foo.test.ts", true, false, false, false},
		{"src/foo.spec.ts", true, false, false, false},
		{"tests/foo.ts", true, false, false, false},
		// ignore is independent and suppresses production
		{"docs/guide.md", false, false, false, true},
		{"README.md", false, false, false, true},
		{"src/generated/api.ts", false, false, false, true},
		// non-production, non-test, non-ignored
		{"other/x.go", false, false, false, false},
		// built-in sensitive: manifests and lockfiles by basename anywhere
		{"package.json", false, false, true, false},
		{"go.mod", false, false, true, false},
		{"go.sum", false, false, true, false},
		{"frontend/package-lock.json", false, false, true, false},
		{"Cargo.toml", false, false, true, false},
		{"Gemfile", false, false, true, false},
		// built-in sensitive: CI/release globs
		{".github/workflows/ci.yml", false, false, true, false},
		{".gitlab-ci.yml", false, false, true, false},
		{"scripts/release/publish.sh", false, false, true, false},
		// configured sensitive
		{"config/app.yml", false, false, true, false},
		// production AND sensitive (e.g. a manifest under src is still sensitive)
		{"src/package.json", false, true, true, false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			in := []model.ChangedFile{{Path: tc.path, Status: "modified"}}
			out := Classify(in, opts)
			if len(out) != 1 {
				t.Fatalf("expected 1 file, got %d", len(out))
			}
			f := out[0]
			if f.IsTest != tc.test || f.IsProduction != tc.production ||
				f.IsSensitive != tc.sensitive || f.IsIgnored != tc.ignored {
				t.Errorf("Classify(%q) = test=%v prod=%v sens=%v ign=%v; want test=%v prod=%v sens=%v ign=%v",
					tc.path, f.IsTest, f.IsProduction, f.IsSensitive, f.IsIgnored,
					tc.test, tc.production, tc.sensitive, tc.ignored)
			}
		})
	}
}

func TestClassifyPreservesOrderAndFields(t *testing.T) {
	in := []model.ChangedFile{
		{Path: "src/b.ts", Status: "modified", Additions: 3, Deletions: 1},
		{Path: "src/a.ts", Status: "added", Additions: 5},
	}
	out := Classify(in, ClassifyOptions{ProductionPaths: []string{"src/**"}})
	if len(out) != 2 || out[0].Path != "src/b.ts" || out[1].Path != "src/a.ts" {
		t.Fatalf("Classify changed order: %+v", out)
	}
	if out[0].Additions != 3 || out[0].Deletions != 1 || out[1].Additions != 5 {
		t.Errorf("Classify mutated line counts: %+v", out)
	}
	// Input must not be mutated (Classify returns a fresh slice).
	if in[0].IsProduction {
		t.Errorf("Classify mutated the input slice")
	}
}

package engine_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/internal/report"
	"github.com/codesteward-ai/codesteward/pkg/engine"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// codeownersContent is the CONTRACTS §7.2 reference CODEOWNERS rule set.
//
// The codeowners package implements GitHub's contracted "last matching rule
// wins" semantics (locked in by internal/codeowners tests), so the `*` fallback
// is listed FIRST and the specific rules AFTER. This is the conventional GitHub
// CODEOWNERS layout: the catch-all provides default ownership that the deeper,
// specific rules override. It yields the §7.2 outcomes (src/parser is owned
// specifically; src/runtime falls back to the catch-all).
const codeownersContent = `* @maintainers
/src/parser/ @parser-maintainers
/src/public/ @api-maintainers
/docs/ @docs-maintainers
`

// longDescription is a realistic PR description comfortably over the default
// 80-character minimum used by scenario 1.
const longDescription = "Add support for normalizing a leading unary minus in the tokenizer and " +
	"extend the parser unit tests to cover the new subtraction handling behavior."

// requireGit skips the test when the git binary is unavailable.
func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available; skipping git-dependent engine test")
	}
}

// runGit runs a git command in dir with a hermetic environment (isolated from
// any host git config) and fails the test on error.
func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// newRepo creates an initialized git repository on branch main in a temp dir.
func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	return dir
}

// writeFile writes content to a slash-separated, repo-relative path under dir,
// creating parent directories as needed.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

// commitAll stages the whole work tree and commits it.
func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-q", "-m", msg)
}

// baseRepo creates a repository whose main branch contains only the reference
// CODEOWNERS file, then checks out a fresh "change" branch ready for edits.
func baseRepo(t *testing.T) string {
	t.Helper()
	dir := newRepo(t)
	writeFile(t, dir, "CODEOWNERS", codeownersContent)
	commitAll(t, dir, "initial: add CODEOWNERS")
	runGit(t, dir, "checkout", "-q", "-b", "change")
	return dir
}

// dumpOnFailure logs the rendered report JSON if the test has failed, so a
// diverging pipeline is easy to diagnose.
func dumpOnFailure(t *testing.T, r *model.Report) {
	t.Helper()
	t.Cleanup(func() {
		if t.Failed() {
			b, _ := report.RenderJSON(r)
			t.Logf("report JSON:\n%s", b)
		}
	})
}

// distinctRuleIDs returns the sorted set of distinct rule IDs present in the
// findings.
func distinctRuleIDs(findings []model.Finding) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range findings {
		if seen[f.RuleID] {
			continue
		}
		seen[f.RuleID] = true
		out = append(out, f.RuleID)
	}
	sort.Strings(out)
	return out
}

// TestScenario1GoodPR mirrors CONTRACTS §7.2 scenario 1: a production source
// change accompanied by its matching test and a real description.
func TestScenario1GoodPR(t *testing.T) {
	requireGit(t)
	dir := baseRepo(t)

	writeFile(t, dir, "src/parser/tokenize.ts",
		"export function tokenize(input: string): string[] {\n  return input.split(/\\s+/);\n}\n")
	writeFile(t, dir, "tests/parser/tokenize.test.ts",
		"import { tokenize } from \"../../src/parser/tokenize\";\n\nit(\"splits\", () => {\n  expect(tokenize(\"a b\")).toEqual([\"a\", \"b\"]);\n});\n")
	commitAll(t, dir, "parser: add tokenize and its test")

	res, err := engine.Run(engine.Options{
		RepoRoot:       dir,
		Base:           "main",
		Head:           "HEAD",
		Description:    longDescription,
		DescriptionSet: true,
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}
	r := res.Report
	dumpOnFailure(t, r)

	if r.Score != 100 {
		t.Errorf("score = %d, want 100", r.Score)
	}
	if r.Status != model.StatusReady {
		t.Errorf("status = %q, want %q", r.Status, model.StatusReady)
	}
	if r.Ownership.State != model.OwnershipComplete {
		t.Errorf("ownership = %q, want %q", r.Ownership.State, model.OwnershipComplete)
	}
	if r.Tests.State != model.TestsMatchingChanged {
		t.Errorf("tests = %q, want %q", r.Tests.State, model.TestsMatchingChanged)
	}
	if r.ReviewBurden != model.BurdenLow {
		t.Errorf("burden = %q, want %q", r.ReviewBurden, model.BurdenLow)
	}
	if len(r.Findings) != 0 {
		t.Errorf("findings = %v, want none", distinctRuleIDs(r.Findings))
	}
}

// TestScenario2WeakPR mirrors CONTRACTS §7.2 scenario 2: a fallback-owned
// production change with no test and an empty description.
func TestScenario2WeakPR(t *testing.T) {
	requireGit(t)
	dir := baseRepo(t)

	writeFile(t, dir, "src/runtime/cache.ts",
		"export class Cache<T> {\n  private store = new Map<string, T>();\n  get(k: string) { return this.store.get(k); }\n}\n")
	commitAll(t, dir, "runtime: add cache")

	res, err := engine.Run(engine.Options{
		RepoRoot:       dir,
		Base:           "main",
		Head:           "HEAD",
		Description:    "",
		DescriptionSet: true,
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}
	r := res.Report
	dumpOnFailure(t, r)

	if r.Score != 60 {
		t.Errorf("score = %d, want 60", r.Score)
	}
	if r.Status != model.StatusNeedsAction {
		t.Errorf("status = %q, want %q", r.Status, model.StatusNeedsAction)
	}
	if r.Ownership.State != model.OwnershipPartial {
		t.Errorf("ownership = %q, want %q", r.Ownership.State, model.OwnershipPartial)
	}
	if r.Tests.State != model.TestsMissingMatching {
		t.Errorf("tests = %q, want %q", r.Tests.State, model.TestsMissingMatching)
	}
	if r.ReviewBurden != model.BurdenMedium {
		t.Errorf("burden = %q, want %q", r.ReviewBurden, model.BurdenMedium)
	}
	want := []string{model.RuleOwnFallbackOnly, model.RuleTstMissing, model.RuleDscEmpty}
	sort.Strings(want)
	if got := distinctRuleIDs(r.Findings); !reflect.DeepEqual(got, want) {
		t.Errorf("rule IDs = %v, want %v", got, want)
	}
}

// TestScenario3BroadPR mirrors CONTRACTS §7.2 scenario 3: a broad, mixed-concern
// change touching source, docs, a manifest, and a CI workflow.
func TestScenario3BroadPR(t *testing.T) {
	requireGit(t)
	dir := baseRepo(t)

	writeFile(t, dir, "src/parser/parse.ts",
		"export function parse(tokens: string[]): number {\n  return tokens.length;\n}\n")
	writeFile(t, dir, "src/runtime/cache.ts",
		"export class Cache<T> {\n  private store = new Map<string, T>();\n}\n")
	writeFile(t, dir, "docs/usage.md", "# Usage\n\nRun the parser.\n")
	writeFile(t, dir, "package.json", "{\n  \"name\": \"example\",\n  \"version\": \"1.1.0\"\n}\n")
	writeFile(t, dir, ".github/workflows/release.yml",
		"name: release\non:\n  push:\n    tags: [\"v*\"]\njobs:\n  release:\n    runs-on: ubuntu-latest\n    steps:\n      - run: echo release\n")
	commitAll(t, dir, "broad change across areas")

	res, err := engine.Run(engine.Options{
		RepoRoot:       dir,
		Base:           "main",
		Head:           "HEAD",
		Description:    "",
		DescriptionSet: true,
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}
	r := res.Report
	dumpOnFailure(t, r)

	if r.Score != 15 {
		t.Errorf("score = %d, want 15", r.Score)
	}
	if r.Status != model.StatusHighBurden {
		t.Errorf("status = %q, want %q", r.Status, model.StatusHighBurden)
	}
	if r.ReviewBurden != model.BurdenHigh {
		t.Errorf("burden = %q, want %q", r.ReviewBurden, model.BurdenHigh)
	}
	want := []string{
		model.RuleOwnFallbackOnly,  // CS-OWN-002
		model.RuleTstMissing,       // CS-TST-001
		model.RuleScpSrcPlusDeps,   // CS-SCP-003
		model.RuleScpMixedConcerns, // CS-SCP-004
		model.RuleDscEmpty,         // CS-DSC-001
		model.RuleSnsCIWorkflow,    // CS-SNS-002
		model.RuleSnsManifest,      // CS-SNS-003
	}
	sort.Strings(want)
	if got := distinctRuleIDs(r.Findings); !reflect.DeepEqual(got, want) {
		t.Errorf("rule IDs = %v, want %v", got, want)
	}
}

// TestNoConfigNoCodeowners verifies the engine runs with built-in defaults when
// neither a config file nor a CODEOWNERS file is present, surfacing both as
// warnings rather than errors.
func TestNoConfigNoCodeowners(t *testing.T) {
	requireGit(t)
	dir := newRepo(t)
	writeFile(t, dir, "src/app.ts", "export const version = 1;\n")
	commitAll(t, dir, "initial")
	runGit(t, dir, "checkout", "-q", "-b", "change")
	writeFile(t, dir, "src/app.ts", "export const version = 2;\n")
	commitAll(t, dir, "bump version")

	res, err := engine.Run(engine.Options{
		RepoRoot: dir,
		Base:     "main",
		Head:     "HEAD",
		Version:  "test",
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}
	r := res.Report
	dumpOnFailure(t, r)

	if !containsWarning(r.Warnings, "no config file found; using built-in defaults") {
		t.Errorf("missing no-config warning; warnings = %v", r.Warnings)
	}
	if !containsWarning(r.Warnings, "no CODEOWNERS file found") {
		t.Errorf("missing no-CODEOWNERS warning; warnings = %v", r.Warnings)
	}
	if res.ConfigLoad == nil || res.ConfigLoad.Found {
		t.Errorf("expected ConfigLoad.Found == false, got %+v", res.ConfigLoad)
	}
	if res.Config == nil {
		t.Errorf("expected non-nil Config")
	}
}

func containsWarning(warnings []string, want string) bool {
	for _, w := range warnings {
		if w == want {
			return true
		}
	}
	return false
}

// TestRunOutsideGitRepo verifies that running the engine against a directory
// that is not inside a git working tree surfaces the contract-designed,
// actionable "not inside a git repository" error (CONTRACTS §6.2), even though
// RepoRoot is a non-empty explicit path. Repo detection (CONTRACTS §6.12 step
// 1) must run for every RepoRoot, not only the empty one, so the CLI default of
// "--repo-root ." reaches this path instead of failing later with a misleading
// ref-resolution error.
func TestRunOutsideGitRepo(t *testing.T) {
	requireGit(t)
	dir := t.TempDir() // fresh temp dir, deliberately not a git repository

	_, err := engine.Run(engine.Options{
		RepoRoot: dir,
		Base:     "main",
		Head:     "HEAD",
		Version:  "test",
	})
	if err == nil {
		t.Fatalf("engine.Run in a non-git directory: got nil error, want a git-detection error")
	}
	if !strings.Contains(err.Error(), "is not inside a git repository") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "is not inside a git repository")
	}
}

// TestRunResolvesCanonicalRoot verifies that repo detection runs even when
// RepoRoot points somewhere inside the working tree (not the toplevel), so the
// canonical repository root is used and the scan still succeeds.
func TestRunResolvesCanonicalRoot(t *testing.T) {
	requireGit(t)
	dir := baseRepo(t)
	writeFile(t, dir, "src/app.ts", "export const version = 1;\n")
	commitAll(t, dir, "add app")

	// Point RepoRoot at a subdirectory of the work tree; detection must walk up
	// to the repository toplevel rather than trusting the passed path blindly.
	sub := filepath.Join(dir, "src")
	res, err := engine.Run(engine.Options{
		RepoRoot:       sub,
		Base:           "main",
		Head:           "HEAD",
		Description:    "",
		DescriptionSet: true,
		Version:        "test",
	})
	if err != nil {
		t.Fatalf("engine.Run from subdirectory: %v", err)
	}
	if res.Report == nil {
		t.Fatalf("expected a report, got nil")
	}
}

// TestDeterminism verifies that two runs over the identical fixture produce
// deep-equal reports and byte-identical rendered JSON.
func TestDeterminism(t *testing.T) {
	requireGit(t)
	dir := baseRepo(t)
	writeFile(t, dir, "src/runtime/cache.ts",
		"export class Cache<T> {\n  private store = new Map<string, T>();\n}\n")
	writeFile(t, dir, "package.json", "{\n  \"name\": \"example\",\n  \"version\": \"2.0.0\"\n}\n")
	commitAll(t, dir, "runtime + manifest change")

	opts := engine.Options{
		RepoRoot:       dir,
		Base:           "main",
		Head:           "HEAD",
		Description:    "",
		DescriptionSet: true,
		Version:        "test",
	}

	res1, err := engine.Run(opts)
	if err != nil {
		t.Fatalf("engine.Run (1): %v", err)
	}
	res2, err := engine.Run(opts)
	if err != nil {
		t.Fatalf("engine.Run (2): %v", err)
	}

	if !reflect.DeepEqual(res1.Report, res2.Report) {
		t.Errorf("reports not deep-equal:\n%+v\n%+v", res1.Report, res2.Report)
	}

	j1, err := report.RenderJSON(res1.Report)
	if err != nil {
		t.Fatalf("RenderJSON (1): %v", err)
	}
	j2, err := report.RenderJSON(res2.Report)
	if err != nil {
		t.Fatalf("RenderJSON (2): %v", err)
	}
	if string(j1) != string(j2) {
		t.Errorf("rendered JSON not byte-identical:\n%s\n---\n%s", j1, j2)
	}
}

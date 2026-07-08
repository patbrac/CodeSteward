package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/internal/codeowners"
	"github.com/codesteward-ai/codesteward/internal/config"
	"github.com/codesteward-ai/codesteward/internal/ownership"
	"github.com/codesteward-ai/codesteward/internal/providers/github"
	"github.com/codesteward-ai/codesteward/internal/providers/gitlab"
	"github.com/codesteward-ai/codesteward/internal/report"
	"github.com/codesteward-ai/codesteward/internal/version"
	"github.com/codesteward-ai/codesteward/pkg/engine"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

// run invokes Main with the given args and a fixed getenv, capturing output.
func run(t *testing.T, args []string, getenv func(string) string) (code int, stdout, stderr string) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	code = Main(args, &outBuf, &errBuf, getenv)
	return code, outBuf.String(), errBuf.String()
}

// saveSeams captures the mutable package seams and returns a restore function
// to be deferred so tests do not leak overrides into one another.
func saveSeams(t *testing.T) {
	t.Helper()
	origEngineRun := engineRun
	origConfigLoad := configLoad
	origConfigValidate := configValidate
	origDiscover := codeownersDiscover
	origParse := codeownersParse
	origValidate := codeownersValidate
	origAudit := ownershipAudit
	origMarkdown := renderMarkdown
	origJSON := renderJSON
	origGH := githubDetectEnv
	origGL := gitlabDetectEnv
	origPostGH := postGitHubComment
	origPostGL := postGitLabNote
	t.Cleanup(func() {
		engineRun = origEngineRun
		configLoad = origConfigLoad
		configValidate = origConfigValidate
		codeownersDiscover = origDiscover
		codeownersParse = origParse
		codeownersValidate = origValidate
		ownershipAudit = origAudit
		renderMarkdown = origMarkdown
		renderJSON = origJSON
		githubDetectEnv = origGH
		gitlabDetectEnv = origGL
		postGitHubComment = origPostGH
		postGitLabNote = origPostGL
	})
}

// noProvider wires both provider detectors to report no CI environment.
func noProvider(t *testing.T) {
	t.Helper()
	githubDetectEnv = func(func(string) string) (*github.Env, error) {
		return &github.Env{IsActions: false}, nil
	}
	gitlabDetectEnv = func(func(string) string) (*gitlab.Env, error) {
		return &gitlab.Env{IsCI: false}, nil
	}
}

func TestMainNoArgs(t *testing.T) {
	code, out, errOut := run(t, nil, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr missing usage: %q", errOut)
	}
}

func TestMainTopLevelHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		code, out, errOut := run(t, []string{arg}, nil)
		if code != 0 {
			t.Errorf("%s: code = %d, want 0", arg, code)
		}
		if !strings.Contains(out, "Commands:") {
			t.Errorf("%s: stdout missing commands list: %q", arg, out)
		}
		if errOut != "" {
			t.Errorf("%s: stderr = %q, want empty", arg, errOut)
		}
	}
}

func TestMainUnknownCommand(t *testing.T) {
	code, out, errOut := run(t, []string{"frobnicate"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "unknown command") {
		t.Errorf("stderr missing 'unknown command': %q", errOut)
	}
}

func TestVersion(t *testing.T) {
	code, out, errOut := run(t, []string{"version"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	want := fmt.Sprintf("codesteward %s (commit %s, built %s, %s)\n",
		version.Version, version.Commit, version.Date, runtime.Version())
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestVersionHelp(t *testing.T) {
	code, out, errOut := run(t, []string{"version", "--help"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "codesteward version") {
		t.Errorf("stdout missing version usage: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestVersionUnexpectedArg(t *testing.T) {
	code, out, errOut := run(t, []string{"version", "extra"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "unexpected argument") {
		t.Errorf("stderr missing 'unexpected argument': %q", errOut)
	}
}

func TestHelpForEveryCommand(t *testing.T) {
	cases := [][]string{
		{"scan", "--help"},
		{"ownership", "audit", "--help"},
		{"ownership", "--help"},
		{"config", "validate", "--help"},
		{"config", "--help"},
		{"codeowners", "validate", "--help"},
		{"codeowners", "--help"},
		{"version", "-h"},
	}
	for _, args := range cases {
		code, out, errOut := run(t, args, nil)
		if code != 0 {
			t.Errorf("%v: code = %d, want 0", args, code)
		}
		if !strings.Contains(out, "Usage:") {
			t.Errorf("%v: stdout missing usage: %q", args, out)
		}
		if errOut != "" {
			t.Errorf("%v: stderr = %q, want empty", args, errOut)
		}
	}
}

func TestScanUnknownFlag(t *testing.T) {
	code, out, errOut := run(t, []string{"scan", "--nope"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "not defined") {
		t.Errorf("stderr missing flag diagnostic: %q", errOut)
	}
	if !strings.Contains(errOut, "Usage:") {
		t.Errorf("stderr missing usage: %q", errOut)
	}
}

func TestScanBadFormat(t *testing.T) {
	code, out, errOut := run(t, []string{"scan", "--format", "xml"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "invalid --format") {
		t.Errorf("stderr missing 'invalid --format': %q", errOut)
	}
}

func TestScanBothDescriptionFlags(t *testing.T) {
	code, out, errOut := run(t, []string{"scan", "--description", "x", "--description-file", "y"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "use only one") {
		t.Errorf("stderr missing exclusivity error: %q", errOut)
	}
}

func TestScanCommentNoProvider(t *testing.T) {
	saveSeams(t)
	noProvider(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		t.Fatal("engineRun should not be reached when no provider is detected")
		return nil, nil
	}
	code, out, errOut := run(t, []string{"scan", "--comment"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "--comment requires") {
		t.Errorf("stderr missing comment error: %q", errOut)
	}
	if !strings.Contains(errOut, "--dry-run") {
		t.Errorf("stderr should suggest --dry-run: %q", errOut)
	}
}

func TestScanSuccessMarkdownStdout(t *testing.T) {
	saveSeams(t)
	engineRun = func(opts engine.Options) (*engine.Result, error) {
		if opts.DescriptionSet {
			t.Errorf("DescriptionSet = true, want false for local run without description")
		}
		return &engine.Result{Report: &model.Report{Base: "main", Head: "HEAD"}}, nil
	}
	renderMarkdown = func(r *model.Report, opts report.MarkdownOptions) string {
		if opts.ShowScore {
			t.Errorf("ShowScore = true, want false")
		}
		return "MARKDOWN-BODY"
	}
	code, out, errOut := run(t, []string{"scan"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if out != "MARKDOWN-BODY" {
		t.Errorf("stdout = %q, want MARKDOWN-BODY", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestScanShowScoreAndWarnings(t *testing.T) {
	saveSeams(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		return &engine.Result{Report: &model.Report{Warnings: []string{"a", "b"}}}, nil
	}
	var gotShowScore bool
	renderMarkdown = func(r *model.Report, opts report.MarkdownOptions) string {
		gotShowScore = opts.ShowScore
		return "X"
	}
	code, out, errOut := run(t, []string{"scan", "--show-score"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if out != "X" {
		t.Errorf("stdout = %q, want X", out)
	}
	if !gotShowScore {
		t.Errorf("ShowScore not propagated to renderer")
	}
	if !strings.Contains(errOut, "warning: a\n") || !strings.Contains(errOut, "warning: b\n") {
		t.Errorf("stderr missing prefixed warnings: %q", errOut)
	}
}

func TestScanJSONToOutputFile(t *testing.T) {
	saveSeams(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderJSON = func(*model.Report) ([]byte, error) {
		return []byte("{\"ok\":true}\n"), nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string {
		t.Fatal("renderMarkdown should not be called for --format json without --comment")
		return ""
	}
	outFile := filepath.Join(t.TempDir(), "report.json")
	code, out, errOut := run(t, []string{"scan", "--format", "json", "--output", outFile}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty when --output set", out)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "{\"ok\":true}\n" {
		t.Errorf("file contents = %q", string(data))
	}
}

func TestScanEngineError(t *testing.T) {
	saveSeams(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		return nil, fmt.Errorf("boom: shallow clone")
	}
	code, out, errOut := run(t, []string{"scan"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "boom: shallow clone") {
		t.Errorf("stderr missing engine error: %q", errOut)
	}
}

func TestScanDescriptionText(t *testing.T) {
	saveSeams(t)
	var gotDesc string
	var gotSet bool
	engineRun = func(opts engine.Options) (*engine.Result, error) {
		gotDesc, gotSet = opts.Description, opts.DescriptionSet
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "X" }
	// Explicit empty description forces DescriptionSet=true.
	code, _, _ := run(t, []string{"scan", "--description", ""}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if gotDesc != "" || !gotSet {
		t.Errorf("Description=%q DescriptionSet=%v, want \"\" true", gotDesc, gotSet)
	}
}

func TestScanDescriptionFile(t *testing.T) {
	saveSeams(t)
	var gotDesc string
	var gotSet bool
	engineRun = func(opts engine.Options) (*engine.Result, error) {
		gotDesc, gotSet = opts.Description, opts.DescriptionSet
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "X" }
	descFile := filepath.Join(t.TempDir(), "desc.txt")
	if err := os.WriteFile(descFile, []byte("hello body"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := run(t, []string{"scan", "--description-file", descFile}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if gotDesc != "hello body" || !gotSet {
		t.Errorf("Description=%q DescriptionSet=%v, want %q true", gotDesc, gotSet, "hello body")
	}
}

func TestScanDescriptionFileMissing(t *testing.T) {
	saveSeams(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		t.Fatal("engineRun should not run when description file is unreadable")
		return nil, nil
	}
	code, out, errOut := run(t, []string{"scan", "--description-file", "/no/such/file/xyz"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "cannot read --description-file") {
		t.Errorf("stderr missing description-file error: %q", errOut)
	}
}

func TestScanCommentGitHubDryRun(t *testing.T) {
	saveSeams(t)
	githubDetectEnv = func(func(string) string) (*github.Env, error) {
		return &github.Env{
			IsActions:   true,
			Repo:        "owner/repo",
			PRNumber:    7,
			BaseRef:     "main",
			HeadRef:     "feature",
			APIURL:      "https://api.github.test",
			Token:       "tok",
			Description: "provider body",
		}, nil
	}
	gitlabDetectEnv = func(func(string) string) (*gitlab.Env, error) {
		return &gitlab.Env{IsCI: false}, nil
	}
	var gotBase, gotHead, gotDesc string
	var gotSet bool
	engineRun = func(opts engine.Options) (*engine.Result, error) {
		gotBase, gotHead, gotDesc, gotSet = opts.Base, opts.Head, opts.Description, opts.DescriptionSet
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "BODY" }
	var postedBody string
	var postedDryRun bool
	postGitHubComment = func(_ context.Context, env *github.Env, body string, dryRun bool) error {
		postedBody, postedDryRun = body, dryRun
		if env.Repo != "owner/repo" || env.PRNumber != 7 {
			t.Errorf("env not threaded through: %+v", env)
		}
		return nil
	}
	postGitLabNote = func(context.Context, *gitlab.Env, string, bool) error {
		t.Fatal("GitLab note should not be posted for a GitHub environment")
		return nil
	}
	code, out, errOut := run(t, []string{"scan", "--comment", "--dry-run"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if out != "BODY" {
		t.Errorf("stdout = %q, want BODY", out)
	}
	if gotBase != "main" || gotHead != "feature" {
		t.Errorf("provider refs not used as defaults: base=%q head=%q", gotBase, gotHead)
	}
	if gotDesc != "provider body" || !gotSet {
		t.Errorf("provider description not used: desc=%q set=%v", gotDesc, gotSet)
	}
	if postedBody != "BODY" || !postedDryRun {
		t.Errorf("post got body=%q dryRun=%v, want BODY true", postedBody, postedDryRun)
	}
}

func TestScanCommentGitLabExplicitRefsOverride(t *testing.T) {
	saveSeams(t)
	githubDetectEnv = func(func(string) string) (*github.Env, error) {
		return &github.Env{IsActions: false}, nil
	}
	gitlabDetectEnv = func(func(string) string) (*gitlab.Env, error) {
		return &gitlab.Env{IsCI: true, ProjectID: "42", MRIID: 3, BaseRef: "prov-base", HeadRef: "prov-head", APIURL: "u", Token: "t", Description: "d"}, nil
	}
	var gotBase, gotHead string
	engineRun = func(opts engine.Options) (*engine.Result, error) {
		gotBase, gotHead = opts.Base, opts.Head
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "B" }
	var posted bool
	postGitLabNote = func(_ context.Context, env *gitlab.Env, body string, dryRun bool) error {
		posted = true
		if env.ProjectID != "42" || env.MRIID != 3 {
			t.Errorf("env not threaded: %+v", env)
		}
		return nil
	}
	// Explicit --base overrides provider BaseRef; --head falls back to provider.
	code, _, errOut := run(t, []string{"scan", "--comment", "--base", "explicit-base"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	if gotBase != "explicit-base" {
		t.Errorf("base = %q, want explicit-base", gotBase)
	}
	if gotHead != "prov-head" {
		t.Errorf("head = %q, want prov-head", gotHead)
	}
	if !posted {
		t.Errorf("GitLab note not posted")
	}
}

func TestScanCommentPostError(t *testing.T) {
	saveSeams(t)
	githubDetectEnv = func(func(string) string) (*github.Env, error) {
		return &github.Env{IsActions: true, Repo: "o/r", PRNumber: 1, APIURL: "u", Token: "t"}, nil
	}
	gitlabDetectEnv = func(func(string) string) (*gitlab.Env, error) { return &gitlab.Env{IsCI: false}, nil }
	engineRun = func(engine.Options) (*engine.Result, error) {
		return &engine.Result{Report: &model.Report{}}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "B" }
	postGitHubComment = func(context.Context, *github.Env, string, bool) error {
		return fmt.Errorf("401 unauthorized: check token permissions")
	}
	code, _, errOut := run(t, []string{"scan", "--comment"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(errOut, "401 unauthorized") {
		t.Errorf("stderr missing post error: %q", errOut)
	}
}

func TestScanVerbose(t *testing.T) {
	saveSeams(t)
	engineRun = func(engine.Options) (*engine.Result, error) {
		return &engine.Result{
			Report:     &model.Report{Base: "b", Head: "h"},
			ConfigLoad: &config.LoadResult{Found: true, Path: ".codesteward.yaml"},
		}, nil
	}
	renderMarkdown = func(*model.Report, report.MarkdownOptions) string { return "X" }
	code, _, errOut := run(t, []string{"scan", "--verbose"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(errOut, "using config .codesteward.yaml") {
		t.Errorf("stderr missing config diagnostic: %q", errOut)
	}
	if !strings.Contains(errOut, "comparing b...h") {
		t.Errorf("stderr missing ref diagnostic: %q", errOut)
	}
}

func TestOwnershipNoSubcommand(t *testing.T) {
	code, out, errOut := run(t, []string{"ownership"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "subcommand") {
		t.Errorf("stderr missing subcommand hint: %q", errOut)
	}
}

func TestOwnershipUnknownSubcommand(t *testing.T) {
	code, _, errOut := run(t, []string{"ownership", "frob"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "unknown ownership subcommand") {
		t.Errorf("stderr missing unknown subcommand: %q", errOut)
	}
}

func TestOwnershipAuditBadFormat(t *testing.T) {
	code, _, errOut := run(t, []string{"ownership", "audit", "--format", "yaml"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "invalid --format") {
		t.Errorf("stderr missing format error: %q", errOut)
	}
}

func auditSeams(t *testing.T, res *ownership.AuditResult) {
	t.Helper()
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return config.Default(), &config.LoadResult{Found: true, Path: ".codesteward.yaml"}, nil
	}
	codeownersDiscover = func(string, codeowners.Dialect) (string, error) { return "CODEOWNERS", nil }
	codeownersParse = func(string, codeowners.Dialect) (*codeowners.File, error) {
		return &codeowners.File{}, nil
	}
	ownershipAudit = func(string, model.OwnerMatcher, []string, []string) (*ownership.AuditResult, error) {
		return res, nil
	}
}

func TestOwnershipAuditMarkdown(t *testing.T) {
	saveSeams(t)
	auditSeams(t, &ownership.AuditResult{
		Total: 2, Specific: 1, Unowned: 1,
		Entries: []ownership.AuditEntry{
			{Path: "src/a.ts", Class: model.MatchSpecific, Owners: []string{"@team"}, Pattern: "/src/**"},
			{Path: "src/b.ts", Class: model.MatchMissing},
		},
	})
	code, out, errOut := run(t, []string{"ownership", "audit"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	for _, want := range []string{"# Ownership Audit", "| Total | 2 |", "| Unowned | 1 |", "src/a.ts", "@team", "src/b.ts"} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestOwnershipAuditJSON(t *testing.T) {
	saveSeams(t)
	auditSeams(t, &ownership.AuditResult{
		Total: 1, Broad: 1,
		Entries: []ownership.AuditEntry{
			{Path: "src/a.ts", Class: model.MatchBroad, Owners: []string{"@core"}, Pattern: "/src/**"},
		},
	})
	code, out, errOut := run(t, []string{"ownership", "audit", "--format", "json"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (stderr=%q)", code, errOut)
	}
	var parsed struct {
		Total   int `json:"total"`
		Broad   int `json:"broad"`
		Entries []struct {
			Path  string `json:"path"`
			Class string `json:"class"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if parsed.Total != 1 || parsed.Broad != 1 {
		t.Errorf("counts wrong: %+v", parsed)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Path != "src/a.ts" || parsed.Entries[0].Class != "broad" {
		t.Errorf("entries wrong: %+v", parsed.Entries)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("JSON output should end with newline")
	}
}

func TestOwnershipAuditConfigError(t *testing.T) {
	saveSeams(t)
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return nil, nil, fmt.Errorf("config file not found: missing.yaml")
	}
	code, out, errOut := run(t, []string{"ownership", "audit", "--config", "missing.yaml"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "config file not found") {
		t.Errorf("stderr missing config error: %q", errOut)
	}
}

func TestConfigNoSubcommand(t *testing.T) {
	code, _, errOut := run(t, []string{"config"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "subcommand") {
		t.Errorf("stderr missing subcommand hint: %q", errOut)
	}
}

func TestConfigValidateOK(t *testing.T) {
	saveSeams(t)
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return config.Default(), &config.LoadResult{Found: true, Path: ".codesteward.yaml"}, nil
	}
	configValidate = func(*config.Config) []string { return nil }
	code, out, errOut := run(t, []string{"config", "validate"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "configuration is valid") {
		t.Errorf("stdout missing valid message: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestConfigValidateWarningsOnly(t *testing.T) {
	saveSeams(t)
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return config.Default(), &config.LoadResult{Warnings: []string{"unknown key: foo"}}, nil
	}
	configValidate = func(*config.Config) []string { return []string{"warning: min_length is small"} }
	code, out, errOut := run(t, []string{"config", "validate"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (warnings only)", code)
	}
	if !strings.Contains(out, "warning: unknown key: foo") {
		t.Errorf("stdout missing load warning: %q", out)
	}
	if !strings.Contains(out, "warning: min_length is small") {
		t.Errorf("stdout missing validate warning: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

// TestConfigValidateDeduplicatesGlobWarnings guards against the same problem
// being printed twice. config.Load surfaces invalid-glob warnings in
// loadRes.Warnings and config.Validate recomputes the identical strings; the
// command must emit each problem exactly once.
func TestConfigValidateDeduplicatesGlobWarnings(t *testing.T) {
	saveSeams(t)
	dup := "invalid glob pattern in ownership.production_paths: \"a[b\""
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return config.Default(), &config.LoadResult{Warnings: []string{dup}}, nil
	}
	configValidate = func(*config.Config) []string {
		// Same glob warning Load already reported, plus a distinct one.
		return []string{"warning: " + dup, "warning: min_length is small"}
	}
	code, out, errOut := run(t, []string{"config", "validate"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (warnings only)", code)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
	if got := strings.Count(out, dup); got != 1 {
		t.Errorf("glob warning printed %d times, want 1:\n%s", got, out)
	}
	if !strings.Contains(out, "warning: min_length is small") {
		t.Errorf("distinct validate warning dropped:\n%s", out)
	}
}

// TestConfigValidateDedupWithRealValidate exercises the real config.Load /
// config.Validate overlap via the CLI: an invalid glob in an on-disk config
// must be reported once, not once per source.
func TestConfigValidateDedupWithRealValidate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ".codesteward.yaml")
	if err := os.WriteFile(cfgPath, []byte("ownership:\n  production_paths:\n    - \"a[b\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := run(t, []string{"config", "validate", "--repo-root", dir}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0 (glob issues are warnings); stderr=%q", code, errOut)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
	const want = "invalid glob pattern in ownership.production_paths: \"a[b\""
	if got := strings.Count(out, want); got != 1 {
		t.Errorf("glob warning printed %d times, want 1:\n%s", got, out)
	}
}

func TestConfigValidateErrorsExit1(t *testing.T) {
	saveSeams(t)
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return config.Default(), &config.LoadResult{}, nil
	}
	configValidate = func(*config.Config) []string {
		return []string{"error: max_files_changed must be non-negative", "warning: dialect is unusual"}
	}
	code, out, errOut := run(t, []string{"config", "validate"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(out, "error: max_files_changed must be non-negative") {
		t.Errorf("stdout missing error problem: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty (problems go to stdout)", errOut)
	}
}

func TestConfigValidateLoadError(t *testing.T) {
	saveSeams(t)
	configLoad = func(string, string) (*config.Config, *config.LoadResult, error) {
		return nil, nil, fmt.Errorf("cannot read config: permission denied")
	}
	code, out, errOut := run(t, []string{"config", "validate", "--config", "x.yaml"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "cannot read config") {
		t.Errorf("stderr missing load error: %q", errOut)
	}
}

func TestCodeownersNoSubcommand(t *testing.T) {
	code, _, errOut := run(t, []string{"codeowners"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.Contains(errOut, "subcommand") {
		t.Errorf("stderr missing subcommand hint: %q", errOut)
	}
}

func TestCodeownersValidateBadDialect(t *testing.T) {
	code, out, errOut := run(t, []string{"codeowners", "validate", "--dialect", "svn"}, nil)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "invalid --dialect") {
		t.Errorf("stderr missing dialect error: %q", errOut)
	}
}

func TestCodeownersValidateNoFile(t *testing.T) {
	saveSeams(t)
	codeownersDiscover = func(string, codeowners.Dialect) (string, error) { return "", nil }
	code, out, errOut := run(t, []string{"codeowners", "validate"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "no CODEOWNERS file found") {
		t.Errorf("stdout missing no-file message: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestCodeownersValidateOK(t *testing.T) {
	saveSeams(t)
	codeownersDiscover = func(string, codeowners.Dialect) (string, error) { return ".github/CODEOWNERS", nil }
	codeownersParse = func(string, codeowners.Dialect) (*codeowners.File, error) {
		return &codeowners.File{}, nil
	}
	codeownersValidate = func(*codeowners.File) []string { return nil }
	code, out, errOut := run(t, []string{"codeowners", "validate", "--dialect", "github"}, nil)
	if code != 0 {
		t.Fatalf("code = %d, want 0", code)
	}
	if !strings.Contains(out, "CODEOWNERS is valid") {
		t.Errorf("stdout missing valid message: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty", errOut)
	}
}

func TestCodeownersValidateErrorsExit1(t *testing.T) {
	saveSeams(t)
	codeownersDiscover = func(string, codeowners.Dialect) (string, error) { return "CODEOWNERS", nil }
	codeownersParse = func(string, codeowners.Dialect) (*codeowners.File, error) {
		return &codeowners.File{}, nil
	}
	codeownersValidate = func(*codeowners.File) []string {
		return []string{"error: line 3: malformed owner 'bogus'", "warning: line 5: empty owner list"}
	}
	code, out, errOut := run(t, []string{"codeowners", "validate"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if !strings.Contains(out, "error: line 3") {
		t.Errorf("stdout missing error: %q", out)
	}
	if !strings.Contains(out, "warning: line 5") {
		t.Errorf("stdout missing warning: %q", out)
	}
	if errOut != "" {
		t.Errorf("stderr = %q, want empty (problems go to stdout)", errOut)
	}
}

func TestCodeownersValidateParseError(t *testing.T) {
	saveSeams(t)
	codeownersDiscover = func(string, codeowners.Dialect) (string, error) { return "CODEOWNERS", nil }
	codeownersParse = func(string, codeowners.Dialect) (*codeowners.File, error) {
		return nil, fmt.Errorf("cannot read CODEOWNERS: permission denied")
	}
	code, out, errOut := run(t, []string{"codeowners", "validate"}, nil)
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if out != "" {
		t.Errorf("stdout = %q, want empty", out)
	}
	if !strings.Contains(errOut, "cannot read CODEOWNERS") {
		t.Errorf("stderr missing parse error: %q", errOut)
	}
}

// TestDeterministicAuditRender verifies the audit renderers are byte-stable.
func TestDeterministicAuditRender(t *testing.T) {
	res := &ownership.AuditResult{
		Total: 1, Specific: 1,
		Entries: []ownership.AuditEntry{
			{Path: "src/a.ts", Class: model.MatchSpecific, Owners: []string{"@team"}, Pattern: "/src/**"},
		},
	}
	if renderAuditMarkdown(res) != renderAuditMarkdown(res) {
		t.Errorf("markdown audit render not deterministic")
	}
	a, err := renderAuditJSON(res)
	if err != nil {
		t.Fatal(err)
	}
	b, err := renderAuditJSON(res)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Errorf("json audit render not deterministic")
	}
}

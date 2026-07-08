// Package cli implements command dispatch, per-command flag parsing, usage
// text, and stable process exit codes for the codesteward binary.
//
// Exit codes are stable and documented:
//
//	0  success (also: validation ran and found only warnings)
//	1  runtime/scan error, or a validation command found errors
//	2  usage error (unknown command/flag, missing subcommand, bad flag value)
//
// A --help or -h flag on any command prints that command's usage to stdout and
// exits 0. Unknown commands, unknown flags, and invalid enum values print usage
// to stderr and exit 2. The scan command never exits non-zero because of report
// content: CodeSteward is a comment-only tool.
//
// All output is routed through the injected stdout/stderr writers and all
// environment access goes through the injected getenv, so the package makes no
// direct use of os.Stdout, os.Stderr, or os.Getenv.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

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

// Package-level indirection over collaborating packages. Production code uses
// the real functions; unit tests replace these seams so the CLI dispatch,
// output routing, and exit codes can be exercised without depending on other
// packages' (possibly stubbed) behavior.
var (
	engineRun          = engine.Run
	configLoad         = config.Load
	configValidate     = config.Validate
	codeownersDiscover = codeowners.Discover
	codeownersParse    = codeowners.ParseFile
	codeownersValidate = codeowners.Validate
	ownershipAudit     = ownership.Audit
	renderMarkdown     = report.RenderMarkdown
	renderJSON         = report.RenderJSON
	githubDetectEnv    = github.DetectEnv
	gitlabDetectEnv    = gitlab.DetectEnv

	postGitHubComment = func(ctx context.Context, env *github.Env, body string, dryRun bool) error {
		c := github.NewClient(env.APIURL, env.Token, nil)
		return c.UpsertComment(ctx, env.Repo, env.PRNumber, body, dryRun)
	}
	postGitLabNote = func(ctx context.Context, env *gitlab.Env, body string, dryRun bool) error {
		c := gitlab.NewClient(env.APIURL, env.Token, nil)
		return c.UpsertNote(ctx, env.ProjectID, env.MRIID, body, dryRun)
	}
)

// Main dispatches a CodeSteward command and returns the process exit code.
func Main(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		topUsage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		topUsage(stdout)
		return 0
	case "version":
		return runVersion(args[1:], stdout, stderr)
	case "scan":
		return runScan(args[1:], stdout, stderr, getenv)
	case "ownership":
		return runOwnership(args[1:], stdout, stderr)
	case "config":
		return runConfig(args[1:], stdout, stderr)
	case "codeowners":
		return runCodeowners(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "codesteward: unknown command %q\n", args[0])
		topUsage(stderr)
		return 2
	}
}

// parseFlags parses args with fs, suppressing the flag package's built-in usage
// so this package controls exactly where usage text goes. It returns done=true
// when the caller should return immediately with the given code: code 0 for
// -h/--help (usage printed to stdout) and code 2 for a parse error (the flag
// diagnostic and usage printed to stderr).
func parseFlags(fs *flag.FlagSet, args []string, usage func(io.Writer), stdout, stderr io.Writer) (done bool, code int) {
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	fs.Usage = func() {}
	err := fs.Parse(args)
	if err == nil {
		return false, 0
	}
	if errors.Is(err, flag.ErrHelp) {
		usage(stdout)
		return true, 0
	}
	if buf.Len() > 0 {
		_, _ = stderr.Write(buf.Bytes())
	}
	usage(stderr)
	return true, 2
}

// rejectExtraArgs reports a usage error when positional arguments remain after
// flag parsing. Returns true when the caller should return code 2.
func rejectExtraArgs(fs *flag.FlagSet, usage func(io.Writer), stderr io.Writer) bool {
	if fs.NArg() == 0 {
		return false
	}
	fmt.Fprintf(stderr, "codesteward: unexpected argument %q\n", fs.Arg(0))
	usage(stderr)
	return true
}

// dedupePreservingOrder returns s with exact-duplicate entries removed,
// keeping the first occurrence of each value so ordering stays deterministic.
func dedupePreservingOrder(s []string) []string {
	seen := make(map[string]bool, len(s))
	out := make([]string, 0, len(s))
	for _, v := range s {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func runVersion(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	if done, code := parseFlags(fs, args, versionUsage, stdout, stderr); done {
		return code
	}
	if rejectExtraArgs(fs, versionUsage, stderr) {
		return 2
	}
	fmt.Fprintf(stdout, "codesteward %s (commit %s, built %s, %s)\n",
		version.Version, version.Commit, version.Date, runtime.Version())
	return 0
}

func runScan(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	base := fs.String("base", "", "base ref to diff from")
	head := fs.String("head", "", "head ref to diff to")
	format := fs.String("format", "markdown", "output format: markdown or json")
	output := fs.String("output", "", "write the report to a file instead of stdout")
	configPath := fs.String("config", "", "path to a .codesteward.yaml config file")
	repoRoot := fs.String("repo-root", ".", "repository root directory")
	desc := fs.String("description", "", "PR/MR description text")
	descFile := fs.String("description-file", "", "read the PR/MR description from a file")
	comment := fs.Bool("comment", false, "post or update the report as a PR/MR comment")
	dryRun := fs.Bool("dry-run", false, "with --comment, log the intended action without posting")
	showScore := fs.Bool("show-score", false, "show the internal readiness score in Markdown")
	verbose := fs.Bool("verbose", false, "print extra diagnostics to stderr")

	if done, code := parseFlags(fs, args, scanUsage, stdout, stderr); done {
		return code
	}
	if rejectExtraArgs(fs, scanUsage, stderr) {
		return 2
	}
	if *format != "markdown" && *format != "json" {
		fmt.Fprintf(stderr, "codesteward: invalid --format %q (want markdown or json)\n", *format)
		scanUsage(stderr)
		return 2
	}

	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["description"] && set["description-file"] {
		fmt.Fprintln(stderr, "codesteward: use only one of --description or --description-file")
		scanUsage(stderr)
		return 2
	}

	// Provider detection happens only when a comment is requested. The detected
	// environment also supplies default refs and description.
	var (
		providerKind string
		ghEnv        *github.Env
		glEnv        *gitlab.Env
	)
	if *comment {
		kind, gh, gl, err := detectProvider(getenv)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if kind == "" {
			fmt.Fprintln(stderr, "codesteward: --comment requires a CI provider environment "+
				"(GitHub Actions or GitLab CI); none detected. Run inside CI, and use --dry-run "+
				"to preview without posting.")
			return 1
		}
		providerKind, ghEnv, glEnv = kind, gh, gl
	}

	description := ""
	descriptionSet := false
	switch {
	case set["description"]:
		description = *desc
		descriptionSet = true
	case set["description-file"]:
		data, err := os.ReadFile(*descFile)
		if err != nil {
			fmt.Fprintf(stderr, "codesteward: cannot read --description-file %q: %v\n", *descFile, err)
			return 1
		}
		description = string(data)
		descriptionSet = true
	default:
		switch providerKind {
		case "github":
			description = ghEnv.Description
			descriptionSet = true
		case "gitlab":
			description = glEnv.Description
			descriptionSet = true
		}
	}

	baseVal, headVal := *base, *head
	switch providerKind {
	case "github":
		if baseVal == "" {
			baseVal = ghEnv.BaseRef
		}
		if headVal == "" {
			headVal = ghEnv.HeadRef
		}
	case "gitlab":
		if baseVal == "" {
			baseVal = glEnv.BaseRef
		}
		if headVal == "" {
			headVal = glEnv.HeadRef
		}
	}

	res, err := engineRun(engine.Options{
		RepoRoot:       *repoRoot,
		ConfigPath:     *configPath,
		Base:           baseVal,
		Head:           headVal,
		Description:    description,
		DescriptionSet: descriptionSet,
		Version:        version.Version,
	})
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if res == nil || res.Report == nil {
		fmt.Fprintln(stderr, "codesteward: scan produced no report")
		return 1
	}

	for _, w := range res.Report.Warnings {
		fmt.Fprintf(stderr, "warning: %s\n", w)
	}

	if *verbose {
		if res.ConfigLoad != nil {
			if res.ConfigLoad.Found {
				fmt.Fprintf(stderr, "codesteward: using config %s\n", res.ConfigLoad.Path)
			} else {
				fmt.Fprintln(stderr, "codesteward: using built-in default config")
			}
		}
		fmt.Fprintf(stderr, "codesteward: comparing %s...%s\n", res.Report.Base, res.Report.Head)
	}

	var out []byte
	switch *format {
	case "json":
		b, err := renderJSON(res.Report)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		out = b
	default:
		out = []byte(renderMarkdown(res.Report, report.MarkdownOptions{ShowScore: *showScore}))
	}

	if *output != "" {
		if err := os.WriteFile(*output, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "codesteward: cannot write --output %q: %v\n", *output, err)
			return 1
		}
	} else {
		_, _ = stdout.Write(out)
	}

	if *comment {
		body := renderMarkdown(res.Report, report.MarkdownOptions{ShowScore: *showScore})
		ctx := context.Background()
		var perr error
		switch providerKind {
		case "github":
			perr = postGitHubComment(ctx, ghEnv, body, *dryRun)
		case "gitlab":
			perr = postGitLabNote(ctx, glEnv, body, *dryRun)
		}
		if perr != nil {
			fmt.Fprintln(stderr, perr)
			return 1
		}
	}

	return 0
}

// detectProvider reports the first supported CI provider whose environment is
// present. kind is "" when no provider environment is detected.
func detectProvider(getenv func(string) string) (kind string, gh *github.Env, gl *gitlab.Env, err error) {
	ghEnv, gerr := githubDetectEnv(getenv)
	if gerr != nil {
		return "", nil, nil, gerr
	}
	if ghEnv != nil && ghEnv.IsActions {
		return "github", ghEnv, nil, nil
	}
	glEnv, lerr := gitlabDetectEnv(getenv)
	if lerr != nil {
		return "", nil, nil, lerr
	}
	if glEnv != nil && glEnv.IsCI {
		return "gitlab", nil, glEnv, nil
	}
	return "", nil, nil, nil
}

func runOwnership(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "codesteward: ownership requires a subcommand (audit)")
		ownershipAuditUsage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		ownershipAuditUsage(stdout)
		return 0
	case "audit":
		return runOwnershipAudit(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "codesteward: unknown ownership subcommand %q\n", args[0])
		ownershipAuditUsage(stderr)
		return 2
	}
}

func runOwnershipAudit(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ownership audit", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to a .codesteward.yaml config file")
	repoRoot := fs.String("repo-root", ".", "repository root directory")
	format := fs.String("format", "markdown", "output format: markdown or json")
	output := fs.String("output", "", "write the audit to a file instead of stdout")

	if done, code := parseFlags(fs, args, ownershipAuditUsage, stdout, stderr); done {
		return code
	}
	if rejectExtraArgs(fs, ownershipAuditUsage, stderr) {
		return 2
	}
	if *format != "markdown" && *format != "json" {
		fmt.Fprintf(stderr, "codesteward: invalid --format %q (want markdown or json)\n", *format)
		ownershipAuditUsage(stderr)
		return 2
	}

	cfg, loadRes, err := configLoad(*repoRoot, *configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if loadRes != nil {
		for _, w := range loadRes.Warnings {
			fmt.Fprintf(stderr, "warning: %s\n", w)
		}
	}

	dialect := dialectFor(cfg.Ownership.Dialect)
	path, err := codeownersDiscover(*repoRoot, dialect)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	var matcher model.OwnerMatcher
	if path != "" {
		f, err := codeownersParse(path, dialect)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if f != nil {
			matcher = f
			for _, w := range f.Warnings {
				fmt.Fprintf(stderr, "warning: CODEOWNERS line %d: %s\n", w.Line, w.Text)
			}
		}
	}

	res, err := ownershipAudit(*repoRoot, matcher, cfg.Ownership.ProductionPaths, cfg.Ownership.IgnorePaths)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var out []byte
	switch *format {
	case "json":
		b, err := renderAuditJSON(res)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		out = b
	default:
		out = []byte(renderAuditMarkdown(res))
	}

	if *output != "" {
		if err := os.WriteFile(*output, out, 0o644); err != nil {
			fmt.Fprintf(stderr, "codesteward: cannot write --output %q: %v\n", *output, err)
			return 1
		}
	} else {
		_, _ = stdout.Write(out)
	}
	return 0
}

func runConfig(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "codesteward: config requires a subcommand (validate)")
		configValidateUsage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		configValidateUsage(stdout)
		return 0
	case "validate":
		return runConfigValidate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "codesteward: unknown config subcommand %q\n", args[0])
		configValidateUsage(stderr)
		return 2
	}
}

func runConfigValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("config validate", flag.ContinueOnError)
	configPath := fs.String("config", "", "path to a .codesteward.yaml config file")
	repoRoot := fs.String("repo-root", ".", "repository root directory")

	if done, code := parseFlags(fs, args, configValidateUsage, stdout, stderr); done {
		return code
	}
	if rejectExtraArgs(fs, configValidateUsage, stderr) {
		return 2
	}

	cfg, loadRes, err := configLoad(*repoRoot, *configPath)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	var problems []string
	if loadRes != nil {
		for _, w := range loadRes.Warnings {
			problems = append(problems, "warning: "+w)
		}
	}
	problems = append(problems, configValidate(cfg)...)

	// config.Load() already surfaces invalid-glob warnings in loadRes.Warnings,
	// and config.Validate() recomputes the identical strings, so the two sources
	// overlap. Deduplicate exact matches, keeping first occurrence, so each
	// problem is reported once while preserving deterministic order.
	problems = dedupePreservingOrder(problems)

	hasError := false
	for _, p := range problems {
		if strings.HasPrefix(p, "error: ") {
			hasError = true
		}
		fmt.Fprintln(stdout, p)
	}
	if len(problems) == 0 {
		fmt.Fprintln(stdout, "codesteward: configuration is valid")
	}
	if hasError {
		return 1
	}
	return 0
}

func runCodeowners(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "codesteward: codeowners requires a subcommand (validate)")
		codeownersValidateUsage(stderr)
		return 2
	}
	switch args[0] {
	case "-h", "--help", "help":
		codeownersValidateUsage(stdout)
		return 0
	case "validate":
		return runCodeownersValidate(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "codesteward: unknown codeowners subcommand %q\n", args[0])
		codeownersValidateUsage(stderr)
		return 2
	}
}

func runCodeownersValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("codeowners validate", flag.ContinueOnError)
	repoRoot := fs.String("repo-root", ".", "repository root directory")
	dialectFlag := fs.String("dialect", "auto", "CODEOWNERS dialect: github, gitlab, or auto")

	if done, code := parseFlags(fs, args, codeownersValidateUsage, stdout, stderr); done {
		return code
	}
	if rejectExtraArgs(fs, codeownersValidateUsage, stderr) {
		return 2
	}
	if *dialectFlag != "github" && *dialectFlag != "gitlab" && *dialectFlag != "auto" {
		fmt.Fprintf(stderr, "codesteward: invalid --dialect %q (want github, gitlab, or auto)\n", *dialectFlag)
		codeownersValidateUsage(stderr)
		return 2
	}

	dialect := dialectFor(*dialectFlag)
	path, err := codeownersDiscover(*repoRoot, dialect)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	if path == "" {
		fmt.Fprintln(stdout, "codesteward: no CODEOWNERS file found")
		return 0
	}

	f, err := codeownersParse(path, dialect)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	problems := codeownersValidate(f)
	hasError := false
	for _, p := range problems {
		if strings.HasPrefix(p, "error: ") {
			hasError = true
		}
		fmt.Fprintln(stdout, p)
	}
	if len(problems) == 0 {
		fmt.Fprintln(stdout, "codesteward: CODEOWNERS is valid")
	}
	if hasError {
		return 1
	}
	return 0
}

// dialectFor maps a config/flag dialect string to a codeowners.Dialect,
// defaulting to auto for unrecognized values.
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

// renderAuditJSON renders an ownership audit as stable, indented JSON with a
// trailing newline. Entries are already sorted by path by ownership.Audit.
func renderAuditJSON(res *ownership.AuditResult) ([]byte, error) {
	type entry struct {
		Path    string   `json:"path"`
		Owners  []string `json:"owners,omitempty"`
		Pattern string   `json:"pattern,omitempty"`
		Class   string   `json:"class"`
	}
	type auditOut struct {
		Total    int     `json:"total"`
		Specific int     `json:"specific"`
		Broad    int     `json:"broad"`
		Fallback int     `json:"fallback"`
		Unowned  int     `json:"unowned"`
		Entries  []entry `json:"entries"`
	}
	o := auditOut{
		Total:    res.Total,
		Specific: res.Specific,
		Broad:    res.Broad,
		Fallback: res.Fallback,
		Unowned:  res.Unowned,
		Entries:  make([]entry, 0, len(res.Entries)),
	}
	for _, e := range res.Entries {
		o.Entries = append(o.Entries, entry{
			Path:    e.Path,
			Owners:  e.Owners,
			Pattern: e.Pattern,
			Class:   string(e.Class),
		})
	}
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// renderAuditMarkdown renders an ownership audit as a compact, deterministic
// Markdown report with a summary table and a per-file table.
func renderAuditMarkdown(res *ownership.AuditResult) string {
	var b strings.Builder
	b.WriteString("# Ownership Audit\n\n")
	b.WriteString("| Metric | Count |\n")
	b.WriteString("| --- | --- |\n")
	fmt.Fprintf(&b, "| Total | %d |\n", res.Total)
	fmt.Fprintf(&b, "| Specific | %d |\n", res.Specific)
	fmt.Fprintf(&b, "| Broad | %d |\n", res.Broad)
	fmt.Fprintf(&b, "| Fallback | %d |\n", res.Fallback)
	fmt.Fprintf(&b, "| Unowned | %d |\n", res.Unowned)
	b.WriteString("\n")
	b.WriteString("| Path | Class | Owners | Pattern |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, e := range res.Entries {
		owners := "-"
		if len(e.Owners) > 0 {
			owners = strings.Join(e.Owners, " ")
		}
		pattern := e.Pattern
		if pattern == "" {
			pattern = "-"
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", e.Path, string(e.Class), owners, pattern)
	}
	return b.String()
}

// --- usage text ---

const topUsageText = `CodeSteward - deterministic PR/MR review-readiness bot.

Usage:
  codesteward <command> [flags]

Commands:
  version              Print version, commit, and build metadata.
  scan                 Analyze a change and render a readiness report.
  ownership audit      Audit repository-wide CODEOWNERS coverage.
  config validate      Validate a .codesteward.yaml configuration file.
  codeowners validate  Validate the CODEOWNERS file.

Run "codesteward <command> --help" for command-specific flags.
`

const versionUsageText = `Usage:
  codesteward version

Print the CodeSteward version, commit, build date, and Go toolchain version.
`

const scanUsageText = `Usage:
  codesteward scan [flags]

Analyze the change between base and head and render a review-readiness report.

Flags:
  --base <ref>               Base ref to diff from (default: CI provider or repo default branch).
  --head <ref>               Head ref to diff to (default: HEAD).
  --format markdown|json     Report output format (default: markdown).
  --output <file>            Write the report to a file instead of stdout.
  --config <file>            Path to a .codesteward.yaml/.codesteward.yml file.
  --repo-root <dir>          Repository root directory (default: current directory).
  --description <text>       PR/MR description text to evaluate.
  --description-file <file>  Read the PR/MR description from a file.
  --comment                  Post or update the report as a PR/MR comment (requires CI).
  --dry-run                  With --comment, log the intended action without posting.
  --show-score               Show the internal readiness score in Markdown output.
  --verbose                  Print extra diagnostics to stderr.
`

const ownershipAuditUsageText = `Usage:
  codesteward ownership audit [flags]

Audit repository-wide CODEOWNERS coverage for production files.

Flags:
  --config <file>          Path to a .codesteward.yaml/.codesteward.yml file.
  --repo-root <dir>        Repository root directory (default: current directory).
  --format markdown|json   Output format (default: markdown).
  --output <file>          Write the audit to a file instead of stdout.
`

const configValidateUsageText = `Usage:
  codesteward config validate [flags]

Validate a .codesteward.yaml/.codesteward.yml configuration file.

Flags:
  --config <file>    Path to a .codesteward.yaml/.codesteward.yml file.
  --repo-root <dir>  Repository root directory (default: current directory).
`

const codeownersValidateUsageText = `Usage:
  codesteward codeowners validate [flags]

Validate the CODEOWNERS file discovered for the given dialect.

Flags:
  --repo-root <dir>             Repository root directory (default: current directory).
  --dialect github|gitlab|auto  CODEOWNERS dialect (default: auto).
`

func topUsage(w io.Writer)                { fmt.Fprint(w, topUsageText) }
func versionUsage(w io.Writer)            { fmt.Fprint(w, versionUsageText) }
func scanUsage(w io.Writer)               { fmt.Fprint(w, scanUsageText) }
func ownershipAuditUsage(w io.Writer)     { fmt.Fprint(w, ownershipAuditUsageText) }
func configValidateUsage(w io.Writer)     { fmt.Fprint(w, configValidateUsageText) }
func codeownersValidateUsage(w io.Writer) { fmt.Fprint(w, codeownersValidateUsageText) }

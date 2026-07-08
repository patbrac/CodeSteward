# CodeSteward Engineering Contracts (v0)

This document is the **authoritative integration contract** for all packages.
Implementers MUST follow the signatures, types, rule IDs, penalties, and output
formats defined here exactly. If a contract is impossible or clearly wrong,
implement as closely as possible and report the deviation — do not silently
redesign shared surfaces. The product scope is defined in
`codesteward_phased_build_plan.md`; this file defines the engineering shape.

---

## 1. Ground rules

- Module path: `github.com/codesteward-ai/codesteward`
- Go: 1.26 toolchain, `go 1.24` directive in go.mod (be conservative).
- **Allowed third-party dependencies: `gopkg.in/yaml.v3` ONLY.** Everything
  else is stdlib. Never run `go get`; the dependency is already in go.mod.
- **Determinism is a hard product requirement.** Identical inputs must produce
  byte-identical output:
  - Never range over a map when order can affect output; sort keys first.
  - No timestamps, random values, absolute paths, or environment-dependent
    text in reports.
  - Sort all file lists, findings, owners, and action items with the rules in
    §7.6 before rendering or scoring.
- All repository paths are slash-separated, repo-root-relative, no leading
  `./`. Use `filepath.ToSlash` at boundaries.
- Errors: wrap with `fmt.Errorf("...: %w", err)`. User-facing errors must be
  actionable (say what to do), especially for shallow clones and missing refs.
- Logging: human diagnostics go to **stderr** only. Report output (markdown/
  JSON) goes to stdout or `--output`. Nothing else may print to stdout.
- Every package gets unit tests in the same package dir. Fixture data lives in
  `testdata/` inside the package. Run only your own package's tests
  (`go test ./internal/<pkg>/...`) — other packages may be mid-build.
- `gofmt` everything. Comment style: doc comments on all exported symbols.
- Ownership boundaries: **only touch the files/directories assigned to you.**
  `pkg/model` and `internal/globs` are frozen; do not edit them.

## 2. Repository layout

```
cmd/codesteward/main.go        thin main; calls internal/cli.Main(os.Args[1:])
internal/cli/                  command dispatch, flags, exit codes
internal/config/               config load/validate/defaults
internal/git/                  repo detection, ref resolution
internal/diff/                 changed-file collection + classification
internal/globs/                ** glob matching (FROZEN after foundation)
internal/codeowners/           CODEOWNERS discovery/parse/match/validate
internal/ownership/            ownership analysis + audit
internal/tests/                path-aware test expectation engine
internal/rules/                scope, description, sensitive-path rules
internal/readiness/            scoring, status/burden mapping, report assembly
internal/report/               markdown + JSON renderers
internal/providers/github/     GitHub env detection + comment posting
internal/providers/gitlab/     GitLab env detection + note posting
internal/version/              version metadata (set via ldflags)
pkg/model/                     shared types (FROZEN after foundation)
pkg/engine/                    orchestration: Options -> *model.Report
```

## 3. pkg/model (FROZEN — foundation creates this verbatim)

```go
// Package model defines the shared, stable data model for CodeSteward
// reports. It has no dependencies outside the standard library.
package model

type ReadinessStatus string

const (
	StatusReady            ReadinessStatus = "ready_for_maintainer_review"
	StatusReviewableNotes  ReadinessStatus = "reviewable_with_notes"
	StatusNeedsAction      ReadinessStatus = "needs_contributor_action"
	StatusHighBurden       ReadinessStatus = "high_review_burden"
	StatusNeedsOwnerRoute  ReadinessStatus = "needs_owner_routing"
)

type ReviewBurden string

const (
	BurdenLow    ReviewBurden = "low"
	BurdenMedium ReviewBurden = "medium"
	BurdenHigh   ReviewBurden = "high"
)

type OwnershipState string

const (
	OwnershipComplete     OwnershipState = "complete"
	OwnershipPartial      OwnershipState = "partial"
	OwnershipMissing      OwnershipState = "missing"
	OwnershipNotEvaluated OwnershipState = "not_evaluated"
)

type TestState string

const (
	TestsNotRequired          TestState = "not_required"
	TestsMatchingChanged      TestState = "matching_test_changed"
	TestsExistingNotChanged   TestState = "existing_test_found_but_not_changed"
	TestsMissingMatching      TestState = "missing_matching_test"
	TestsNotEvaluated         TestState = "not_evaluated"
)

type Severity string

const (
	SeverityInfo           Severity = "info"
	SeverityWarning        Severity = "warning"
	SeverityActionRequired Severity = "action_required"
)

// OwnershipMatchClass classifies how a CODEOWNERS rule covers a path.
type OwnershipMatchClass string

const (
	MatchSpecific OwnershipMatchClass = "specific"
	MatchBroad    OwnershipMatchClass = "broad"
	MatchFallback OwnershipMatchClass = "fallback"
	MatchMissing  OwnershipMatchClass = "missing"
)

// ChangedFile is one file touched by the diff between base and head.
type ChangedFile struct {
	Path         string `json:"path"`
	OldPath      string `json:"old_path,omitempty"` // set for renames/copies
	Status       string `json:"status"`             // added|modified|deleted|renamed|copied
	Additions    int    `json:"additions"`
	Deletions    int    `json:"deletions"`
	IsBinary     bool   `json:"is_binary"`
	IsTest       bool   `json:"is_test"`
	IsProduction bool   `json:"is_production"`
	IsSensitive  bool   `json:"is_sensitive"`
	IsIgnored    bool   `json:"is_ignored"` // matched ownership.ignore_paths
}

// OwnershipMatch is the result of matching one path against CODEOWNERS.
type OwnershipMatch struct {
	Found   bool                `json:"found"`
	Owners  []string            `json:"owners,omitempty"`
	Pattern string              `json:"pattern,omitempty"` // raw CODEOWNERS pattern that matched
	Class   OwnershipMatchClass `json:"class"`
}

// OwnerMatcher is implemented by internal/codeowners.File.
type OwnerMatcher interface {
	Match(path string) OwnershipMatch
}

type FileOwnership struct {
	Path    string              `json:"path"`
	Owners  []string            `json:"owners,omitempty"`
	Pattern string              `json:"pattern,omitempty"`
	Class   OwnershipMatchClass `json:"class"`
}

type OwnershipSummary struct {
	State        OwnershipState  `json:"state"`
	AreasTouched int             `json:"areas_touched"`
	MaxAreas     int             `json:"max_areas"`
	Files        []FileOwnership `json:"files,omitempty"`
}

type FileTestExpectation struct {
	Path        string    `json:"path"`
	State       TestState `json:"state"`
	Candidates  []string  `json:"candidates,omitempty"`
	MatchedTest string    `json:"matched_test,omitempty"`
}

type TestsSummary struct {
	State TestState             `json:"state"`
	Files []FileTestExpectation `json:"files,omitempty"`
}

type ScopeSummary struct {
	FilesChanged     int      `json:"files_changed"`
	LinesAdded       int      `json:"lines_added"`
	LinesDeleted     int      `json:"lines_deleted"`
	TopLevelAreas    []string `json:"top_level_areas,omitempty"`
	MaxFilesChanged  int      `json:"max_files_changed"`
	MaxLinesChanged  int      `json:"max_lines_changed"`
	ExceedsFileLimit bool     `json:"exceeds_file_limit"`
	ExceedsLineLimit bool     `json:"exceeds_line_limit"`
}

type DescriptionSummary struct {
	Provided bool `json:"provided"`
	Length   int  `json:"length"`
	Evaluated bool `json:"evaluated"`
}

// Finding is one deterministic observation about the change.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`            // past-tense fact, for the details section
	Action   string   `json:"action,omitempty"`   // imperative contributor action item
	Paths    []string `json:"paths,omitempty"`    // sorted
}

// ExitCriterion is a concrete condition that would clear a finding.
type ExitCriterion struct {
	RuleID      string `json:"rule_id"`
	Description string `json:"description"`
}

type ToolInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Report is the full normalized scan result. JSON field order is the
// struct order below and is part of the stable schema.
type Report struct {
	SchemaVersion string              `json:"schema_version"` // "0.1"
	Tool          ToolInfo            `json:"tool"`
	Base          string              `json:"base"`
	Head          string              `json:"head"`
	CommentOnly   bool                `json:"comment_only"`
	Status        ReadinessStatus     `json:"status"`
	Score         int                 `json:"score"` // 0..100, internal; hidden in markdown
	ReviewBurden  ReviewBurden        `json:"review_burden"`
	Ownership     OwnershipSummary    `json:"ownership"`
	Tests         TestsSummary        `json:"tests"`
	Scope         ScopeSummary        `json:"scope"`
	Description   DescriptionSummary  `json:"description"`
	Findings      []Finding           `json:"findings"`
	ExitCriteria  []ExitCriterion     `json:"exit_criteria"`
	Warnings      []string            `json:"warnings,omitempty"` // non-fatal scan warnings
}
```

Rule ID constants also live in `pkg/model` (`rules.go`):

```go
const (
	RuleOwnNoOwner        = "CS-OWN-001"
	RuleOwnFallbackOnly   = "CS-OWN-002"
	RuleOwnTooManyAreas   = "CS-OWN-003"
	RuleOwnSensitiveNoOwn = "CS-OWN-004"
	RuleTstMissing        = "CS-TST-001"
	RuleTstNotUpdated     = "CS-TST-002"
	RuleTstUnresolved     = "CS-TST-003"
	RuleScpTooManyFiles   = "CS-SCP-001"
	RuleScpTooManyLines   = "CS-SCP-002"
	RuleScpSrcPlusDeps    = "CS-SCP-003"
	RuleScpMixedConcerns  = "CS-SCP-004"
	RuleScpTooManyAreas   = "CS-SCP-005"
	RuleDscEmpty          = "CS-DSC-001"
	RuleDscTooShort       = "CS-DSC-002"
	RuleDscMissingSection = "CS-DSC-003"
	RuleDscNoLinkedIssue  = "CS-DSC-004"
	RuleSnsLockfile       = "CS-SNS-001"
	RuleSnsCIWorkflow     = "CS-SNS-002"
	RuleSnsManifest       = "CS-SNS-003"
	RuleSnsConfigured     = "CS-SNS-004"
)
```

## 4. Rule catalog (IDs, severity, penalty, emission)

Findings are **per-file** where noted (better action items), but scoring
counts **each rule ID at most once** (§7.1).

| Rule ID | Severity | Penalty | Emitted by | When |
|---|---|---|---|---|
| CS-OWN-001 | action_required | 25 | ownership | production file has no CODEOWNERS match (one finding per file) |
| CS-OWN-002 | warning | 10 | ownership | production file covered only by fallback rule (per file) |
| CS-OWN-003 | warning | 10 | ownership | distinct ownership areas > `max_ownership_areas` (one finding) |
| CS-OWN-004 | action_required | 15 | ownership | sensitive file has no CODEOWNERS match (per file) |
| CS-TST-001 | action_required | 25 | tests | production file changed, no matching test exists or changed (per file) |
| CS-TST-002 | warning | 15 | tests | matching test exists on disk but was not changed (per file) |
| CS-TST-003 | info | 10 | tests | file requires tests but no path mapping matched it (per file) |
| CS-SCP-001 | warning | 20 | rules | files changed > `max_files_changed` (one finding) |
| CS-SCP-002 | warning | 15 | rules | lines changed (additions+deletions) > `max_lines_changed` (one) |
| CS-SCP-003 | warning | 10 | rules | production source + dependency manifest/lockfile in same PR (one) |
| CS-SCP-004 | warning | 10 | rules | production source + docs + config/CI all changed together (one) |
| CS-SCP-005 | info | 0 | rules | > 4 distinct top-level areas touched (one; advisory) |
| CS-DSC-001 | warning | 5 | rules | description empty/whitespace and `warn_if_empty` (one) |
| CS-DSC-002 | warning | 10 | rules | description non-empty but shorter than `min_length` (one) |
| CS-DSC-003 | warning | 15 | rules | a configured required section is missing (one per section) |
| CS-DSC-004 | warning | 10 | rules | `require_linked_issue` and no `#123` / issue URL reference (one) |
| CS-SNS-001 | warning | 15 | rules | lockfile changed (one finding, all lockfile paths listed) |
| CS-SNS-002 | warning | 15 | rules | CI/release workflow changed (one finding, paths listed) |
| CS-SNS-003 | warning | 10 | rules | package manifest changed (one finding, paths listed) |
| CS-SNS-004 | warning | 10 | rules | other configured sensitive path changed (one finding, paths listed) |

Sensitive classification priority per file: lockfile > CI/release workflow >
manifest > configured-other. A file fires exactly one CS-SNS rule. Built-in
sets (case-sensitive basenames / path globs):

- Lockfiles: `package-lock.json`, `pnpm-lock.yaml`, `yarn.lock`, `bun.lockb`,
  `go.sum`, `Cargo.lock`, `poetry.lock`, `Gemfile.lock`, `composer.lock`.
- CI/release: `.github/workflows/**`, `.gitlab-ci.yml`, `scripts/release/**`.
- Manifests: `package.json`, `go.mod`, `Cargo.toml`, `pyproject.toml`,
  `Gemfile`, `composer.json`.
- Configured-other: anything in config `sensitive_paths` not caught above.

Message/action templates (use these shapes; keep messages factual past-tense,
actions imperative):

- CS-OWN-001 msg: "No owner found for `<path>`." action: "Add specific ownership for `<path>` or ask a maintainer to route this area."
- CS-OWN-002 msg: "`<path>` is covered only by fallback ownership: `<pattern> <owners>`." action: "Add specific ownership for `<dir>/**` or ask a maintainer to route this area."
- CS-OWN-003 msg: "This change touches <n> ownership areas, above the configured limit of <max>." action: "Consider splitting this change so each PR touches fewer ownership areas."
- CS-TST-001 msg: "`<path>` changed, but no matching test file was changed." action: "Add or update matching tests for `<path>`."
- CS-TST-002 msg: "A matching test exists for `<path>` (`<test>`), but this PR did not update it." action: "Update `<test>` to cover the changes in `<path>`."
- CS-SCP-003 msg: "Dependency files changed alongside production source files." action: "Consider splitting dependency changes from runtime changes."
- CS-DSC-001 msg: "The PR description is empty." action: "Add a short description explaining the motivation and test plan."
- CS-SNS-* msg: "`<path>` changed (<lockfile|CI workflow|package manifest|sensitive path>)." action: "Call out the <kind> change in the description so maintainers can verify it."

## 5. internal/globs (FROZEN — foundation implements)

```go
package globs

// Match reports whether path (slash-separated, repo-relative, no leading /)
// matches pattern with doublestar semantics:
//   *  matches any run of non-separator chars (may be empty)
//   ?  matches one non-separator char
//   ** as a full segment matches zero or more segments
//   a trailing /** also matches the bare directory itself is NOT implied;
//   but "dir/**" matches everything under dir (not dir itself).
//   A pattern with no slash (e.g. "*.md", "package.json") matches against
//   the path's basename AND the whole path.
//   A pattern ending in "/" matches the directory and everything under it.
// Invalid patterns return false, ok=false.
func Match(pattern, path string) (matched, ok bool)

// MatchAny returns the first pattern in patterns that matches path.
func MatchAny(patterns []string, path string) (pattern string, matched bool)
```

## 6. Package contracts

### 6.1 internal/config

```go
type Config struct { // yaml tags mirror the plan's config shape exactly
	Project         ProjectConfig
	Mode            ModeConfig            // comment_only (default true)
	ReviewReadiness ReviewReadinessConfig // max_files_changed 12, max_lines_changed 500, max_ownership_areas 2
	Ownership       OwnershipConfig       // use_codeowners true; production_paths src/**,lib/**,packages/**; ignore_paths docs/**,examples/**,README.md; dialect "auto"
	Tests           TestsConfig           // require_for = production defaults; test_paths tests/**,test/**,**/*.test.*,**/*.spec.*; default path_mappings per plan
	PRDescription   PRDescriptionConfig   // warn_if_empty true, min_length 80, required_sections [], require_linked_issue false
	SensitivePaths  []string              // plan defaults
}
type PathMapping struct { From string; Expect []string }
type LoadResult struct {
	Path     string   // config file used, "" if none
	Found    bool
	Warnings []string // unknown keys, invalid globs, etc.
}

func Default() *Config
// Load discovery order: explicitPath (error if missing/unreadable),
// else <root>/.codesteward.yaml, else <root>/.codesteward.yml, else defaults
// with warning "no config file found; using built-in defaults".
// Loaded config is MERGED over defaults: absent keys keep defaults; a present
// empty list overrides to empty. Unknown keys produce warnings, not errors.
func Load(repoRoot, explicitPath string) (*Config, *LoadResult, error)
// Validate returns problems: "error: ..." entries are fatal for
// `config validate` (exit 1); "warning: ..." entries are not.
// Checks: negative thresholds, invalid globs (via globs.Match ok=false),
// invalid path_mapping placeholders, unknown dialect.
func Validate(cfg *Config) []string
```

### 6.2 internal/git

```go
type RepoInfo struct{ Root, Branch string; IsShallow bool }
func DetectRepo(dir string) (*RepoInfo, error) // git rev-parse --show-toplevel etc.
// ResolveRefs: head defaults to "HEAD". base resolution order:
// explicit flag > env GITHUB_BASE_REF > env CI_MERGE_REQUEST_TARGET_BRANCH_NAME
// > origin/HEAD symbolic ref > "main" if exists > "master" if exists > error.
// If base name X isn't a valid local ref but origin/X is, use origin/X.
// Verify both resolve via `git rev-parse --verify`; on failure in a shallow
// repo, return an error that explains fetch-depth: 0 / git fetch --unshallow.
func ResolveRefs(root, base, head string) (resolvedBase, resolvedHead string, warnings []string, err error)
```

### 6.3 internal/diff

```go
// Collect runs `git diff --find-renames --find-copies -z` variants
// (--numstat and --name-status) between base...head (three-dot: changes on
// head side since merge-base) and merges results into []model.ChangedFile
// sorted by Path. Binary files: numstat "-" columns -> IsBinary, 0 counts.
// Statuses: added|modified|deleted|renamed|copied.
func Collect(root, base, head string) ([]model.ChangedFile, []string /*warnings*/, error)

type ClassifyOptions struct {
	ProductionPaths, IgnorePaths, TestPaths, SensitivePaths []string
}
// Classify sets IsTest, IsProduction, IsSensitive, IsIgnored on each file.
// Precedence: IsIgnored (ignore_paths) is independent; a file matching
// test_paths is IsTest and NEVER IsProduction; production = matches
// ProductionPaths and not test and not ignored. Sensitive = built-in sets
// (§4) OR SensitivePaths globs.
func Classify(files []model.ChangedFile, opts ClassifyOptions) []model.ChangedFile
```

### 6.4 internal/codeowners

```go
type Dialect string // "github" | "gitlab" | "auto"
type Rule struct {
	Pattern string; Owners []string; Line int; Section string // Section "" for github
}
type ParseWarning struct{ Line int; Text string }
type File struct {
	Path string; Dialect Dialect; Rules []Rule; Warnings []ParseWarning
}
// Discover search order — github: .github/CODEOWNERS, CODEOWNERS,
// docs/CODEOWNERS. gitlab: CODEOWNERS, docs/CODEOWNERS, .gitlab/CODEOWNERS.
// auto: union in order .github/, root, docs/, .gitlab/. Returns "" (no error)
// when none exists.
func Discover(root string, dialect Dialect) (path string, err error)
func ParseFile(path string, dialect Dialect) (*File, error)
func Parse(content []byte, sourcePath string, dialect Dialect) *File
// Match implements model.OwnerMatcher. GitHub semantics: last matching rule
// wins (rules with no owners = explicitly unowned -> Found=false but
// Pattern set). GitLab sections: last match within each section; owners are
// the union across sections (sorted, deduped); optional sections ignored for
// v0 scoring. Pattern semantics are gitignore-style: leading "/" anchors to
// repo root; no slash (other than trailing) matches basename anywhere;
// trailing "/" matches directory contents; "*" and "**" as in globs.
func (f *File) Match(path string) model.OwnershipMatch
// Classify: fallback = pattern matches everything ("*", "**", "/**", "/").
// broad = anchored single top-level segment (e.g. "/src/**", "/src/", "src/")
// or bare extension pattern ("*.md", "*.js").
// specific = anything deeper (>= 2 concrete path segments, e.g. "/src/parser/**").
func ClassifyPattern(pattern string) model.OwnershipMatchClass
// Validate returns warnings/errors for `codesteward codeowners validate`:
// invalid lines, empty owner lists (warning), malformed owners (not
// @user / @org/team / email), unsupported syntax ("!" negation etc.),
// section headers when dialect=github.
func Validate(f *File) []string
```

### 6.5 internal/ownership

```go
type Options struct {
	MaxAreas int
	Enabled  bool // config ownership.use_codeowners && CODEOWNERS file found
}
// Analyze considers only files where IsProduction && !IsIgnored, plus
// IsSensitive files for CS-OWN-004. matcher may be nil when no CODEOWNERS
// exists -> if there are relevant production files, every one is unowned
// (state missing, CS-OWN-001 per file); if Enabled=false or no relevant
// files, state not_evaluated and no findings.
// Areas: distinct matched rule Patterns among production files; each unowned
// production file contributes area "unowned:<top-level-dir>". If
// len(areas) > MaxAreas -> CS-OWN-003.
// State: missing if any relevant production file unowned; else partial if
// any covered fallback-only; else complete (specific and broad both count
// as covered).
func Analyze(files []model.ChangedFile, matcher model.OwnerMatcher, opts Options) (model.OwnershipSummary, []model.Finding)

type AuditEntry struct {
	Path string; Owners []string; Pattern string; Class model.OwnershipMatchClass
}
type AuditResult struct {
	Total, Specific, Broad, Fallback, Unowned int
	Entries []AuditEntry // sorted by Path; only production files
}
// Audit walks the repo work tree (git ls-files preferred; skip .git),
// classifies every file matching productionPaths (minus ignorePaths).
func Audit(root string, matcher model.OwnerMatcher, productionPaths, ignorePaths []string) (*AuditResult, error)
```

### 6.6 internal/tests (package `tests`)

```go
type Options struct {
	RequireFor   []string            // globs; files needing tests
	TestPaths    []string            // globs identifying test files
	Mappings     []config.PathMapping
	Root         string              // for on-disk existence checks
	Enabled      bool
}
// Mapping placeholders: {path} = the file's directory path *after* the
// literal prefix in From (may be empty; "src/{path}/{name}.{ext}" matches
// both src/a.ts with path="" and src/x/y/a.ts with path="x/y"), {name} =
// basename without final extension, {ext} = final extension without dot.
// When path=="" collapse double slashes in expansions.
// Per changed file that matches RequireFor (and !IsTest && !IsIgnored &&
// !IsBinary && Status != "deleted"):
//   candidates = union of Expect expansions for every From that matches
//   if any candidate ∈ changed test files (any status)  -> matching_test_changed
//   else if any candidate exists on disk under Root      -> existing_test_found_but_not_changed (CS-TST-002)
//   else if at least one From matched                    -> missing_matching_test (CS-TST-001)
//   else (no From matched)                               -> not_evaluated for that file (CS-TST-003)
// Aggregate state precedence: missing_matching_test > existing_not_changed >
// matching_test_changed > not_required (no files required tests) ; if
// Enabled=false or no mappings configured at all -> not_evaluated.
func Analyze(files []model.ChangedFile, opts Options) (model.TestsSummary, []model.Finding)
```

### 6.7 internal/rules

```go
type ScopeOptions struct{ MaxFiles, MaxLines int }
// Scope: counts exclude IsIgnored files for limits but TopLevelAreas lists
// every top-level segment touched ("src", "docs", "package.json" counts as
// area "package.json"? no — files at root use area "/" ... use "(root)").
// CS-SCP-003 when >=1 production file and >=1 lockfile-or-manifest file.
// CS-SCP-004 when production + docs (path under docs/ or *.md) + config/CI
// (sensitive CI set or dotfile config at root) all present.
// CS-SCP-005 info when len(TopLevelAreas) > 4.
func Scope(files []model.ChangedFile, opts ScopeOptions) (model.ScopeSummary, []model.Finding)

type DescriptionOptions struct {
	Text string; Evaluated bool // Evaluated=false when no description source (local run without flag) -> summary Evaluated=false, still warn empty if WarnIfEmpty? NO: when not Evaluated, emit no findings.
	WarnIfEmpty bool; MinLength int; RequiredSections []string; RequireLinkedIssue bool
}
// Empty = len(strings.TrimSpace(Text)) == 0 -> CS-DSC-001 only (skip length check).
// Length = len([]rune(TrimSpace)). Required section present = markdown
// heading or bold line containing the section name case-insensitively.
// Linked issue = regexp `#\d+` or /issues/\d+ URL.
func Description(opts DescriptionOptions) (model.DescriptionSummary, []model.Finding)

// Sensitive: one finding per fired CS-SNS rule (paths aggregated, sorted).
func Sensitive(files []model.ChangedFile) []model.Finding
```

Note: `--description-file` missing in a local run means Evaluated=false (no
description findings — avoids noise in local usage). Providers (GitHub/GitLab)
always pass the real PR/MR body (possibly empty string) with Evaluated=true.
`codesteward scan --description ""` also forces Evaluated=true.

### 6.8 internal/readiness

```go
// Score: start 100; subtract the penalty (§4 table) once per DISTINCT rule
// ID present in findings; clamp to [0,100].
func Score(findings []model.Finding) int
// Burden: high if score < 40 || CS-SCP-001 or CS-SCP-002 fired;
// low if score >= 85 and no CS-SCP-* or CS-SNS-* findings; else medium.
func Burden(score int, findings []model.Finding) model.ReviewBurden
// Status: 85-100 ready, 65-84 reviewable_with_notes, 40-64
// needs_contributor_action, 0-39 high_review_burden. Override to
// needs_owner_routing when ownership.State == missing AND score < 85 AND
// ownership-category penalties (CS-OWN-*) >= total penalties of all other
// categories combined.
func Status(score int, ownership model.OwnershipSummary, findings []model.Finding) model.ReadinessStatus
// ExitCriteria: one per action_required finding, deduped by (RuleID, first
// path), description = the finding's Action.
func ExitCriteria(findings []model.Finding) []model.ExitCriterion
// BuildReport assembles the full model.Report, applies canonical sort
// (§7.6) to findings, sets SchemaVersion "0.1" and ToolInfo.
func BuildReport(in BuildInput) *model.Report
type BuildInput struct {
	Version string; Base, Head string; CommentOnly bool
	Ownership model.OwnershipSummary; Tests model.TestsSummary
	Scope model.ScopeSummary; Description model.DescriptionSummary
	Findings []model.Finding; Warnings []string
}
```

### 6.9 internal/report

```go
type MarkdownOptions struct{ ShowScore bool } // default false
func RenderMarkdown(r *model.Report, opts MarkdownOptions) string
func RenderJSON(r *model.Report) ([]byte, error) // json.MarshalIndent "", "  ", trailing "\n"
```

Markdown template (exact structure; `<!-- codesteward-report -->` marker
constant exported as `report.Marker`):

```markdown
<!-- codesteward-report -->

## CodeSteward: <StatusDisplay>

**Review burden:** <Low|Medium|High>  
**Ownership:** <Complete|Partial|Missing|Not evaluated>  
**Tests:** <TestsDisplay>

<intro line>

### Before maintainer review

- <up to 5 action items>

<details>
<summary>Why CodeSteward flagged this</summary>

- <every finding Message, in canonical order>

</details>

_Comment-only mode. CodeSteward is not blocking this PR._
```

- StatusDisplay: ready → "Ready for maintainer review"; reviewable_with_notes
  → "Reviewable with notes"; needs_contributor_action → "Needs contributor
  action"; high_review_burden → "High review burden"; needs_owner_routing →
  "Needs owner routing".
- TestsDisplay: matching_test_changed → "Present"; missing_matching_test →
  "Missing matching updates"; existing_test_found_but_not_changed →
  "Existing tests not updated"; not_required → "Not required";
  not_evaluated → "Not evaluated".
- Intro line: if no action items -> "Thanks for the contribution. This looks
  ready for maintainer review." else "Thanks for the contribution. A few
  changes would make this easier for maintainers to review."
- Action items: findings' non-empty Action fields in canonical finding order,
  deduplicated (exact string match), max 5. Omit the entire "### Before
  maintainer review" section when there are none.
- Omit the `<details>` block entirely when there are no findings.
- If ShowScore: add line `**Internal score:** <n>/100` after the Tests line.
- Header lines end with two trailing spaces (markdown hard break), as shown.
- The two static header stat labels always render even when not_evaluated.
- Final line always present exactly as shown (comment-only disclaimer).

### 6.10 internal/providers/github and /gitlab

```go
// github
type Env struct{ IsActions bool; Repo string; PRNumber int; BaseRef, HeadRef, EventPath, APIURL, Token, Description string }
func DetectEnv(getenv func(string) string) (*Env, error) // GITHUB_ACTIONS, GITHUB_REPOSITORY, GITHUB_EVENT_PATH (parse JSON for PR number, body, base/head), GITHUB_API_URL, GITHUB_TOKEN
type Client struct{ ... } // NewClient(apiURL, token string, hc *http.Client)
// UpsertComment: GET /repos/{repo}/issues/{pr}/comments (paginate, per_page=100),
// find first whose body contains report.Marker; PATCH it if found else POST.
// dryRun: log intended action to stderr, do nothing.
func (c *Client) UpsertComment(ctx context.Context, repo string, pr int, body string, dryRun bool) error

// gitlab (same shape)
type Env struct{ IsCI bool; ProjectID string; MRIID int; BaseRef, HeadRef, APIURL, Token, Description string }
// env: GITLAB_CI, CI_PROJECT_ID, CI_MERGE_REQUEST_IID, CI_API_V4_URL,
// CI_MERGE_REQUEST_TARGET_BRANCH_NAME, CI_MERGE_REQUEST_DESCRIPTION,
// token: CODESTEWARD_GITLAB_TOKEN then CI_JOB_TOKEN.
// Notes API: GET/POST/PUT /projects/{id}/merge_requests/{iid}/notes
func (c *Client) UpsertNote(ctx context.Context, projectID string, mriid int, body string, dryRun bool) error
```

Both clients: stdlib net/http only, 30s timeout, honor context, return
actionable errors on 401/403/404 (mention token permissions). Unit tests use
`httptest.Server`.

### 6.11 internal/cli + cmd + internal/version

Commands:

```
codesteward version
codesteward scan [--base R] [--head R] [--format markdown|json] [--output F]
                 [--config F] [--repo-root D] [--description S]
                 [--description-file F] [--comment] [--dry-run] [--show-score]
                 [--verbose]
codesteward ownership audit [--config F] [--repo-root D] [--format markdown|json] [--output F]
codesteward config validate [--config F] [--repo-root D]
codesteward codeowners validate [--repo-root D] [--dialect github|gitlab|auto]
```

- stdlib `flag` with a `flag.NewFlagSet` per command; no cobra.
- `func Main(args []string, stdout, stderr io.Writer, getenv func(string) string) int`
  returns the exit code; `cmd/codesteward/main.go` is
  `os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr, os.Getenv))`.
- Exit codes (stable, documented): 0 success; 1 runtime/scan error (also:
  validation found errors); 2 usage error (unknown command/flag, bad value).
  `scan` NEVER exits non-zero because of report content (comment-only tool).
- Unknown command prints usage to stderr, exit 2. `--help`/`-h` on every
  command prints usage to stdout, exit 0.
- `version` prints `codesteward <version> (commit <commit>, built <date>, <go version>)`;
  fields come from internal/version vars `Version` (default "0.1.0-dev"),
  `Commit` ("none"), `Date` ("unknown") set via
  `-ldflags "-X github.com/codesteward-ai/codesteward/internal/version.Version=..."`.
- `--comment` without a detected provider env: error to stderr, exit 1,
  suggesting --dry-run or running inside CI.
- scan flow calls `engine.Run`, renders, writes to stdout or --output, then
  posts comment if requested. Warnings print to stderr prefixed `warning: `.

### 6.12 pkg/engine

```go
type Options struct {
	RepoRoot, ConfigPath, Base, Head string
	Description     string
	DescriptionSet  bool   // true when --description/--description-file/provider supplied it
	Version         string
}
type Result struct {
	Report     *model.Report
	Config     *config.Config
	ConfigLoad *config.LoadResult
}
// Run: detect repo -> load config -> resolve refs -> diff.Collect ->
// diff.Classify -> codeowners Discover/Parse (dialect from config; "auto"
// default) -> ownership.Analyze -> tests.Analyze -> rules.Scope/Description/
// Sensitive -> readiness.BuildReport. All non-fatal issues (missing config,
// missing CODEOWNERS, shallow warnings, codeowners parse warnings) flow into
// Report.Warnings (sorted, deduped).
func Run(opts Options) (*Result, error)
```

## 7. Cross-cutting algorithms

### 7.1 Scoring
`score = clamp(100 - Σ penalty(ruleID) for each DISTINCT rule ID present, 0, 100)`.

### 7.2 Reference scenarios (MUST hold; used as acceptance tests)

Demo repo `examples/typescript-package` (CODEOWNERS lists the catch-all first
because CODEOWNERS is last-match-wins, so the specific rules below override it:
`*` @maintainers, `/src/parser/` @parser-maintainers, `/src/public/`
@api-maintainers, `/docs/` @docs-maintainers; default config; description empty,
Evaluated=true):

1. Changed: `src/parser/tokenize.ts` + `tests/parser/tokenize.test.ts`, desc
   provided (>= 80 chars) → no findings except none; score 100; status
   ready_for_maintainer_review; ownership complete; tests
   matching_test_changed; burden low.
2. Changed: `src/runtime/cache.ts`, empty desc → CS-OWN-002 (-10),
   CS-TST-001 (-25), CS-DSC-001 (-5) → score 60; status
   needs_contributor_action; ownership partial; tests missing_matching_test;
   burden medium.
3. Changed: `src/parser/parse.ts`, `src/runtime/cache.ts`, `docs/usage.md`,
   `package.json`, `.github/workflows/release.yml`, empty desc →
   CS-OWN-002, CS-TST-001, CS-SCP-003, CS-SCP-004, CS-DSC-001, CS-SNS-002,
   CS-SNS-003 → penalties 10+25+10+10+5+15+10 = 85 → score 15; status
   high_review_burden; burden high.

### 7.3 Top-level areas
Area of `a/b/c.ts` = "a"; area of root file `README.md` = "(root)".

### 7.4 Dialect auto-detection
`auto`: if found under `.github/` → github; under `.gitlab/` → gitlab; else
github semantics but parse GitLab `[Section]` headers leniently (treat as
sections, warn).

### 7.5 Ownership area of unowned files
`"unowned:" + topLevelArea(path)`.

### 7.6 Canonical finding order
Sort findings by: severity rank (action_required=0, warning=1, info=2), then
RuleID ascending, then first Paths entry ascending, then Message. Paths inside
a finding are sorted ascending. This order is applied once in
`readiness.BuildReport` and preserved by renderers.

## 8. Testing conventions

- Table-driven tests; `testdata/` fixtures; golden files for renderer
  snapshots (compare exact bytes; provide `-update` flag guarded by env var
  `UPDATE_GOLDEN=1`).
- git-dependent tests create throwaway repos in `t.TempDir()` with
  `git init -b main`, `git -c user.email=t@t -c user.name=t commit`; skip
  with `t.Skip` if git missing.
- Determinism test in report package: render the same report twice → equal.
- No test may depend on network, wall clock, or map order.

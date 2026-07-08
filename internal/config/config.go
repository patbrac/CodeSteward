// Package config loads, validates, and supplies defaults for CodeSteward
// project configuration (.codesteward.yaml / .codesteward.yml).
//
// Loading merges a discovered YAML file over the built-in defaults: keys
// absent from the file keep their default, while a key that is present with an
// empty list overrides the default to an empty list. Unknown keys and invalid
// glob patterns are surfaced as non-fatal warnings rather than errors so that
// a single typo never prevents a scan from running.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/codesteward-ai/codesteward/internal/globs"
	"gopkg.in/yaml.v3"
)

// Config is the full CodeSteward configuration tree. YAML tags mirror the
// documented .codesteward.yaml shape exactly.
type Config struct {
	Project         ProjectConfig         `yaml:"project"`
	Mode            ModeConfig            `yaml:"mode"`
	ReviewReadiness ReviewReadinessConfig `yaml:"review_readiness"`
	Ownership       OwnershipConfig       `yaml:"ownership"`
	Tests           TestsConfig           `yaml:"tests"`
	PRDescription   PRDescriptionConfig   `yaml:"pr_description"`
	SensitivePaths  []string              `yaml:"sensitive_paths"`
}

// ProjectConfig holds project identity metadata.
type ProjectConfig struct {
	Name string `yaml:"name"`
}

// ModeConfig controls operating mode.
type ModeConfig struct {
	CommentOnly bool `yaml:"comment_only"`
}

// ReviewReadinessConfig holds scope thresholds.
type ReviewReadinessConfig struct {
	MaxFilesChanged   int `yaml:"max_files_changed"`
	MaxLinesChanged   int `yaml:"max_lines_changed"`
	MaxOwnershipAreas int `yaml:"max_ownership_areas"`
}

// OwnershipConfig controls CODEOWNERS-based ownership analysis.
type OwnershipConfig struct {
	UseCodeowners   bool     `yaml:"use_codeowners"`
	ProductionPaths []string `yaml:"production_paths"`
	IgnorePaths     []string `yaml:"ignore_paths"`
	Dialect         string   `yaml:"dialect"`
}

// TestsConfig controls path-aware test expectations.
type TestsConfig struct {
	RequireFor   []string      `yaml:"require_for"`
	TestPaths    []string      `yaml:"test_paths"`
	PathMappings []PathMapping `yaml:"path_mappings"`
}

// PathMapping maps a source path template to expected test path templates.
type PathMapping struct {
	From   string   `yaml:"from"`
	Expect []string `yaml:"expect"`
}

// PRDescriptionConfig controls PR/MR description rules.
type PRDescriptionConfig struct {
	WarnIfEmpty        bool     `yaml:"warn_if_empty"`
	MinLength          int      `yaml:"min_length"`
	RequiredSections   []string `yaml:"required_sections"`
	RequireLinkedIssue bool     `yaml:"require_linked_issue"`
}

// LoadResult describes how configuration was resolved.
type LoadResult struct {
	Path     string   // config file used, "" if none
	Found    bool     // true when a config file was read
	Warnings []string // unknown keys, invalid globs, missing-config notice, etc.
}

// Default returns the built-in default configuration. This is a pure-data
// function with no I/O.
func Default() *Config {
	return &Config{
		Project: ProjectConfig{
			Name: "",
		},
		Mode: ModeConfig{
			CommentOnly: true,
		},
		ReviewReadiness: ReviewReadinessConfig{
			MaxFilesChanged:   12,
			MaxLinesChanged:   500,
			MaxOwnershipAreas: 2,
		},
		Ownership: OwnershipConfig{
			UseCodeowners:   true,
			ProductionPaths: []string{"src/**", "lib/**", "packages/**"},
			IgnorePaths:     []string{"docs/**", "examples/**", "README.md"},
			Dialect:         "auto",
		},
		Tests: TestsConfig{
			RequireFor: []string{"src/**", "lib/**", "packages/**"},
			TestPaths:  []string{"tests/**", "test/**", "**/*.test.*", "**/*.spec.*"},
			PathMappings: []PathMapping{
				{
					From: "src/{path}/{name}.{ext}",
					Expect: []string{
						"tests/{path}/{name}.test.{ext}",
						"tests/{path}/{name}.spec.{ext}",
						"src/{path}/{name}.test.{ext}",
						"src/{path}/{name}.spec.{ext}",
					},
				},
			},
		},
		PRDescription: PRDescriptionConfig{
			WarnIfEmpty:        true,
			MinLength:          80,
			RequiredSections:   []string{},
			RequireLinkedIssue: false,
		},
		SensitivePaths: []string{
			"package.json",
			"package-lock.json",
			"pnpm-lock.yaml",
			"yarn.lock",
			".github/workflows/**",
			".gitlab-ci.yml",
			"scripts/release/**",
		},
	}
}

// missingConfigWarning is emitted verbatim when no config file is discovered.
const missingConfigWarning = "no config file found; using built-in defaults"

// Load discovers and loads configuration, merging it over the defaults.
//
// Discovery order:
//   - explicitPath, when non-empty (an unreadable/missing file is an error);
//   - <repoRoot>/.codesteward.yaml;
//   - <repoRoot>/.codesteward.yml;
//   - otherwise the built-in defaults, with a warning that no file was found.
//
// The loaded file is merged over Default(): keys absent from the file keep
// their default value, while a key present with an empty list overrides the
// default to an empty list. Unknown keys and invalid glob patterns produce
// warnings in the returned LoadResult, never errors.
func Load(repoRoot, explicitPath string) (*Config, *LoadResult, error) {
	cfg := Default()
	res := &LoadResult{}

	data, usedPath, err := discover(repoRoot, explicitPath)
	if err != nil {
		return nil, nil, err
	}
	if usedPath == "" {
		res.Warnings = append(res.Warnings, missingConfigWarning)
		return cfg, res, nil
	}

	res.Found = true
	res.Path = filepath.ToSlash(usedPath)

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, nil, fmt.Errorf("parsing config file %q: %w", res.Path, err)
	}

	// An empty file (or one that is only comments) has no content node: the
	// merged result is simply the defaults.
	if len(doc.Content) == 0 {
		return cfg, res, nil
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("config file %q: top-level YAML must be a mapping", res.Path)
	}

	// Walk the mapping keys to detect unknown keys before decoding the known
	// ones over the defaults.
	res.Warnings = append(res.Warnings, unknownKeyWarnings(root)...)

	if err := root.Decode(cfg); err != nil {
		return nil, nil, fmt.Errorf("decoding config file %q: %w", res.Path, err)
	}

	// Default glob patterns are all valid, so any invalid pattern in the merged
	// config necessarily came from the user's file.
	res.Warnings = append(res.Warnings, globWarnings(cfg)...)

	return cfg, res, nil
}

// discover returns the raw bytes and path of the config file to use. usedPath
// is empty (with nil error) when no file exists and defaults should apply.
func discover(repoRoot, explicitPath string) (data []byte, usedPath string, err error) {
	if explicitPath != "" {
		b, rerr := os.ReadFile(explicitPath)
		if rerr != nil {
			return nil, "", fmt.Errorf("reading config file %q: %w", filepath.ToSlash(explicitPath), rerr)
		}
		return b, explicitPath, nil
	}

	for _, name := range []string{".codesteward.yaml", ".codesteward.yml"} {
		p := filepath.Join(repoRoot, name)
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			if errors.Is(rerr, fs.ErrNotExist) {
				continue
			}
			return nil, "", fmt.Errorf("reading config file %q: %w", filepath.ToSlash(p), rerr)
		}
		return b, p, nil
	}
	return nil, "", nil
}

// topLevelKeys is the set of recognized top-level config keys.
var topLevelKeys = map[string]bool{
	"project":          true,
	"mode":             true,
	"review_readiness": true,
	"ownership":        true,
	"tests":            true,
	"pr_description":   true,
	"sensitive_paths":  true,
}

// sectionKeys maps each mapping-valued top-level key to its recognized nested
// keys. Top-level keys whose value is a list (e.g. sensitive_paths) are absent
// here and are not walked for nested keys.
var sectionKeys = map[string]map[string]bool{
	"project": {"name": true},
	"mode":    {"comment_only": true},
	"review_readiness": {
		"max_files_changed":   true,
		"max_lines_changed":   true,
		"max_ownership_areas": true,
	},
	"ownership": {
		"use_codeowners":   true,
		"production_paths": true,
		"ignore_paths":     true,
		"dialect":          true,
	},
	"tests": {
		"require_for":   true,
		"test_paths":    true,
		"path_mappings": true,
	},
	"pr_description": {
		"warn_if_empty":        true,
		"min_length":           true,
		"required_sections":    true,
		"require_linked_issue": true,
	},
}

// unknownKeyWarnings returns a warning for every unrecognized top-level key and
// every unrecognized key directly nested under a recognized mapping section.
// Keys are visited in document order so output is deterministic for a given
// input file.
func unknownKeyWarnings(root *yaml.Node) []string {
	var out []string
	for i := 0; i+1 < len(root.Content); i += 2 {
		keyNode := root.Content[i]
		valNode := root.Content[i+1]
		key := keyNode.Value
		if !topLevelKeys[key] {
			out = append(out, fmt.Sprintf("unknown config key: %s", key))
			continue
		}
		allowed, hasNested := sectionKeys[key]
		if !hasNested || valNode.Kind != yaml.MappingNode {
			continue
		}
		for j := 0; j+1 < len(valNode.Content); j += 2 {
			nkey := valNode.Content[j].Value
			if !allowed[nkey] {
				out = append(out, fmt.Sprintf("unknown config key: %s.%s", key, nkey))
			}
		}
	}
	return out
}

// globField pairs a config field's dotted name with its glob patterns.
type globField struct {
	name     string
	patterns []string
}

// globFields returns the glob-bearing config fields in a fixed order.
func globFields(cfg *Config) []globField {
	return []globField{
		{"ownership.production_paths", cfg.Ownership.ProductionPaths},
		{"ownership.ignore_paths", cfg.Ownership.IgnorePaths},
		{"tests.require_for", cfg.Tests.RequireFor},
		{"tests.test_paths", cfg.Tests.TestPaths},
		{"sensitive_paths", cfg.SensitivePaths},
	}
}

// globWarnings returns a warning for every invalid glob pattern in cfg. A
// pattern is invalid when globs.Match reports ok=false. Fields are visited in
// a fixed order for deterministic output.
func globWarnings(cfg *Config) []string {
	var out []string
	for _, gf := range globFields(cfg) {
		for _, p := range gf.patterns {
			if _, ok := globs.Match(p, "a/b"); !ok {
				out = append(out, fmt.Sprintf("invalid glob pattern in %s: %q", gf.name, p))
			}
		}
	}
	return out
}

// validPlaceholders is the set of placeholder names allowed in path_mappings.
var validPlaceholders = map[string]bool{"path": true, "name": true, "ext": true}

// placeholderRe matches a single {token} placeholder that contains no braces.
var placeholderRe = regexp.MustCompile(`\{[^{}]*\}`)

// placeholderErrors returns "error:" entries for any unknown placeholder token
// or unbalanced braces in a path_mapping template. kind is "from" or "expect".
func placeholderErrors(kind, template string) []string {
	var out []string
	for _, tok := range placeholderRe.FindAllString(template, -1) {
		inner := tok[1 : len(tok)-1]
		if !validPlaceholders[inner] {
			out = append(out, fmt.Sprintf("error: path_mapping %s %q uses unknown placeholder %q (allowed: {path}, {name}, {ext})", kind, template, tok))
		}
	}
	if stripped := placeholderRe.ReplaceAllString(template, ""); strings.ContainsAny(stripped, "{}") {
		out = append(out, fmt.Sprintf("error: path_mapping %s %q has unbalanced braces", kind, template))
	}
	return out
}

// Validate returns configuration problems. Entries prefixed "error: " are fatal
// for `config validate` (exit 1); "warning: " entries are advisory.
//
// Errors: negative thresholds, unknown ownership dialect, and invalid
// path_mapping placeholders. Warnings: invalid glob patterns. Output ordering
// is fixed and deterministic.
func Validate(cfg *Config) []string {
	var out []string

	// Negative thresholds are errors.
	type threshold struct {
		name string
		val  int
	}
	for _, t := range []threshold{
		{"review_readiness.max_files_changed", cfg.ReviewReadiness.MaxFilesChanged},
		{"review_readiness.max_lines_changed", cfg.ReviewReadiness.MaxLinesChanged},
		{"review_readiness.max_ownership_areas", cfg.ReviewReadiness.MaxOwnershipAreas},
		{"pr_description.min_length", cfg.PRDescription.MinLength},
	} {
		if t.val < 0 {
			out = append(out, fmt.Sprintf("error: %s must be >= 0, got %d", t.name, t.val))
		}
	}

	// Unknown dialect is an error. Valid values are github, gitlab, auto.
	switch cfg.Ownership.Dialect {
	case "github", "gitlab", "auto":
	default:
		out = append(out, fmt.Sprintf("error: ownership.dialect %q is invalid (allowed: github, gitlab, auto)", cfg.Ownership.Dialect))
	}

	// Invalid path_mapping placeholders are errors.
	for _, m := range cfg.Tests.PathMappings {
		out = append(out, placeholderErrors("from", m.From)...)
		for _, e := range m.Expect {
			out = append(out, placeholderErrors("expect", e)...)
		}
	}

	// Invalid globs are warnings.
	for _, w := range globWarnings(cfg) {
		out = append(out, "warning: "+w)
	}

	return out
}

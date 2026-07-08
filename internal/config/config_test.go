package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeFile writes content to <dir>/<name> and fails the test on error.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", p, err)
	}
	return p
}

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Mode.CommentOnly != true {
		t.Errorf("Default mode.comment_only = %v, want true", cfg.Mode.CommentOnly)
	}
	if cfg.ReviewReadiness.MaxFilesChanged != 12 ||
		cfg.ReviewReadiness.MaxLinesChanged != 500 ||
		cfg.ReviewReadiness.MaxOwnershipAreas != 2 {
		t.Errorf("Default review_readiness = %+v", cfg.ReviewReadiness)
	}
	if cfg.Ownership.Dialect != "auto" {
		t.Errorf("Default ownership.dialect = %q, want auto", cfg.Ownership.Dialect)
	}
	if !reflect.DeepEqual(cfg.Ownership.ProductionPaths, []string{"src/**", "lib/**", "packages/**"}) {
		t.Errorf("Default production_paths = %v", cfg.Ownership.ProductionPaths)
	}
	if cfg.PRDescription.MinLength != 80 || !cfg.PRDescription.WarnIfEmpty {
		t.Errorf("Default pr_description = %+v", cfg.PRDescription)
	}
	// Default config must itself validate cleanly.
	if problems := Validate(cfg); len(problems) != 0 {
		t.Errorf("Validate(Default()) = %v, want none", problems)
	}
}

func TestLoadDiscoveryPrecedence(t *testing.T) {
	t.Run("yaml beats yml", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, ".codesteward.yaml", "project:\n  name: from-yaml\n")
		writeFile(t, dir, ".codesteward.yml", "project:\n  name: from-yml\n")

		cfg, res, err := Load(dir, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if !res.Found {
			t.Fatal("expected Found=true")
		}
		if got, want := res.Path, filepath.ToSlash(filepath.Join(dir, ".codesteward.yaml")); got != want {
			t.Errorf("res.Path = %q, want %q", got, want)
		}
		if cfg.Project.Name != "from-yaml" {
			t.Errorf("project.name = %q, want from-yaml (.yaml must win)", cfg.Project.Name)
		}
	})

	t.Run("yml used when yaml absent", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, ".codesteward.yml", "project:\n  name: from-yml\n")

		cfg, res, err := Load(dir, "")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got, want := res.Path, filepath.ToSlash(filepath.Join(dir, ".codesteward.yml")); got != want {
			t.Errorf("res.Path = %q, want %q", got, want)
		}
		if cfg.Project.Name != "from-yml" {
			t.Errorf("project.name = %q, want from-yml", cfg.Project.Name)
		}
	})

	t.Run("explicit path overrides discovery", func(t *testing.T) {
		dir := t.TempDir()
		// Discovery files exist but must be ignored in favor of explicit.
		writeFile(t, dir, ".codesteward.yaml", "project:\n  name: from-yaml\n")
		explicit := writeFile(t, dir, "custom.yaml", "project:\n  name: from-explicit\n")

		cfg, res, err := Load(dir, explicit)
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if got, want := res.Path, filepath.ToSlash(explicit); got != want {
			t.Errorf("res.Path = %q, want %q", got, want)
		}
		if cfg.Project.Name != "from-explicit" {
			t.Errorf("project.name = %q, want from-explicit", cfg.Project.Name)
		}
	})

	t.Run("explicit missing path errors", func(t *testing.T) {
		dir := t.TempDir()
		_, _, err := Load(dir, filepath.Join(dir, "does-not-exist.yaml"))
		if err == nil {
			t.Fatal("expected error for missing explicit config path")
		}
		if !strings.Contains(err.Error(), "does-not-exist.yaml") {
			t.Errorf("error should name the missing file, got: %v", err)
		}
	})
}

func TestLoadMissingConfigDefaults(t *testing.T) {
	dir := t.TempDir() // empty: no config files present
	cfg, res, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if res.Found {
		t.Error("expected Found=false when no config file exists")
	}
	if res.Path != "" {
		t.Errorf("res.Path = %q, want empty", res.Path)
	}
	if len(res.Warnings) != 1 || res.Warnings[0] != missingConfigWarning {
		t.Errorf("warnings = %v, want exactly [%q]", res.Warnings, missingConfigWarning)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Errorf("missing-config cfg should equal Default()")
	}
}

func TestLoadMergeSemantics(t *testing.T) {
	dir := t.TempDir()
	cfg, res, err := Load(dir, testdataPath("merge.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}

	// Present scalars override.
	if cfg.Project.Name != "my-package" {
		t.Errorf("project.name = %q, want my-package", cfg.Project.Name)
	}
	if cfg.Mode.CommentOnly != false {
		t.Errorf("mode.comment_only = %v, want false", cfg.Mode.CommentOnly)
	}
	if cfg.ReviewReadiness.MaxFilesChanged != 25 {
		t.Errorf("max_files_changed = %d, want 25", cfg.ReviewReadiness.MaxFilesChanged)
	}
	if cfg.PRDescription.MinLength != 120 {
		t.Errorf("min_length = %d, want 120", cfg.PRDescription.MinLength)
	}
	// Present non-empty list replaces (does not append to) the default.
	if !reflect.DeepEqual(cfg.Ownership.ProductionPaths, []string{"app/**", "internal/**"}) {
		t.Errorf("production_paths = %v, want [app/** internal/**]", cfg.Ownership.ProductionPaths)
	}

	// Absent keys keep their defaults.
	def := Default()
	if cfg.ReviewReadiness.MaxLinesChanged != def.ReviewReadiness.MaxLinesChanged {
		t.Errorf("max_lines_changed = %d, want default %d", cfg.ReviewReadiness.MaxLinesChanged, def.ReviewReadiness.MaxLinesChanged)
	}
	if !reflect.DeepEqual(cfg.Ownership.IgnorePaths, def.Ownership.IgnorePaths) {
		t.Errorf("ignore_paths = %v, want default %v", cfg.Ownership.IgnorePaths, def.Ownership.IgnorePaths)
	}
	if cfg.Ownership.Dialect != def.Ownership.Dialect {
		t.Errorf("dialect = %q, want default %q", cfg.Ownership.Dialect, def.Ownership.Dialect)
	}
	if !reflect.DeepEqual(cfg.Tests.TestPaths, def.Tests.TestPaths) {
		t.Errorf("test_paths = %v, want default", cfg.Tests.TestPaths)
	}
	if !reflect.DeepEqual(cfg.SensitivePaths, def.SensitivePaths) {
		t.Errorf("sensitive_paths = %v, want default", cfg.SensitivePaths)
	}
}

func TestLoadEmptyListOverride(t *testing.T) {
	dir := t.TempDir()
	cfg, res, err := Load(dir, testdataPath("empty_lists.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
	// A present empty list overrides the default to empty (len 0), not nil-kept-default.
	if len(cfg.Ownership.ProductionPaths) != 0 {
		t.Errorf("production_paths = %v, want empty", cfg.Ownership.ProductionPaths)
	}
	if len(cfg.Tests.TestPaths) != 0 {
		t.Errorf("test_paths = %v, want empty", cfg.Tests.TestPaths)
	}
	if len(cfg.SensitivePaths) != 0 {
		t.Errorf("sensitive_paths = %v, want empty", cfg.SensitivePaths)
	}
	// A list NOT mentioned keeps its default.
	def := Default()
	if !reflect.DeepEqual(cfg.Ownership.IgnorePaths, def.Ownership.IgnorePaths) {
		t.Errorf("ignore_paths = %v, want default (untouched)", cfg.Ownership.IgnorePaths)
	}
}

func TestLoadUnknownKeysWarn(t *testing.T) {
	dir := t.TempDir()
	cfg, res, err := Load(dir, testdataPath("unknown_keys.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{
		"unknown config key: mystery",
		"unknown config key: ownership.bogus_nested",
	}
	if !reflect.DeepEqual(res.Warnings, want) {
		t.Errorf("warnings = %v, want %v", res.Warnings, want)
	}
	// Known keys in the same file must still be applied.
	if cfg.Ownership.UseCodeowners != false {
		t.Errorf("ownership.use_codeowners = %v, want false", cfg.Ownership.UseCodeowners)
	}
	if !reflect.DeepEqual(cfg.Tests.RequireFor, []string{"src/**"}) {
		t.Errorf("tests.require_for = %v, want [src/**]", cfg.Tests.RequireFor)
	}
}

func TestLoadInvalidGlobsWarn(t *testing.T) {
	dir := t.TempDir()
	_, res, err := Load(dir, testdataPath("invalid_globs.yaml"))
	if err != nil {
		t.Fatalf("Load must not error on invalid globs: %v", err)
	}
	want := []string{
		`invalid glob pattern in ownership.production_paths: "a[b"`,
		`invalid glob pattern in sensitive_paths: "["`,
	}
	if !reflect.DeepEqual(res.Warnings, want) {
		t.Errorf("warnings = %v, want %v", res.Warnings, want)
	}
}

func TestLoadEmptyAndCommentsOnly(t *testing.T) {
	dir := t.TempDir()
	cfg, res, err := Load(dir, testdataPath("only_comments.yaml"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !res.Found {
		t.Error("expected Found=true for an existing (comments-only) file")
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
	if !reflect.DeepEqual(cfg, Default()) {
		t.Error("comments-only config should equal Default()")
	}
}

func TestLoadTopLevelNotMapping(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir, testdataPath("top_level_list.yaml"))
	if err == nil {
		t.Fatal("expected error when top-level YAML is not a mapping")
	}
	if !strings.Contains(err.Error(), "mapping") {
		t.Errorf("error should mention mapping, got: %v", err)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	_, _, err := Load(dir, testdataPath("malformed.yaml"))
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestLoadDeterministicWarnings(t *testing.T) {
	dir := t.TempDir()
	_, res1, err := Load(dir, testdataPath("unknown_keys.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	_, res2, err := Load(dir, testdataPath("unknown_keys.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res1.Warnings, res2.Warnings) {
		t.Errorf("warnings not deterministic: %v vs %v", res1.Warnings, res2.Warnings)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*Config)
		wantErrors []string // substrings that must appear as "error:" entries
		wantWarns  []string // substrings that must appear as "warning:" entries
	}{
		{
			name:   "clean default",
			mutate: func(*Config) {},
		},
		{
			name: "negative thresholds are errors",
			mutate: func(c *Config) {
				c.ReviewReadiness.MaxFilesChanged = -1
				c.PRDescription.MinLength = -5
			},
			wantErrors: []string{
				"review_readiness.max_files_changed",
				"pr_description.min_length",
			},
		},
		{
			name: "unknown dialect is an error",
			mutate: func(c *Config) {
				c.Ownership.Dialect = "svn"
			},
			wantErrors: []string{"ownership.dialect"},
		},
		{
			name: "empty dialect is an error",
			mutate: func(c *Config) {
				c.Ownership.Dialect = ""
			},
			wantErrors: []string{"ownership.dialect"},
		},
		{
			name: "invalid glob is a warning, not an error",
			mutate: func(c *Config) {
				c.Ownership.ProductionPaths = []string{"["}
			},
			wantWarns: []string{"invalid glob pattern in ownership.production_paths"},
		},
		{
			name: "unknown placeholder is an error",
			mutate: func(c *Config) {
				c.Tests.PathMappings = []PathMapping{
					{From: "src/{bogus}/{name}.{ext}", Expect: []string{"tests/{name}.{ext}"}},
				}
			},
			wantErrors: []string{"unknown placeholder", "{bogus}"},
		},
		{
			name: "unbalanced braces is an error",
			mutate: func(c *Config) {
				c.Tests.PathMappings = []PathMapping{
					{From: "src/{name}.{ext}", Expect: []string{"tests/{name.ext}"}},
				}
			},
			// {name.ext} is an unknown placeholder token (not unbalanced),
			// so expect an unknown-placeholder error.
			wantErrors: []string{"unknown placeholder"},
		},
		{
			name: "truly unbalanced braces is an error",
			mutate: func(c *Config) {
				c.Tests.PathMappings = []PathMapping{
					{From: "src/{name}.{ext}", Expect: []string{"tests/{name.{ext}"}},
				}
			},
			wantErrors: []string{"unbalanced braces"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Default()
			tt.mutate(cfg)
			got := Validate(cfg)

			var errs, warns []string
			for _, g := range got {
				switch {
				case strings.HasPrefix(g, "error: "):
					errs = append(errs, g)
				case strings.HasPrefix(g, "warning: "):
					warns = append(warns, g)
				default:
					t.Errorf("entry %q has neither error:/warning: prefix", g)
				}
			}

			for _, sub := range tt.wantErrors {
				if !containsSub(errs, sub) {
					t.Errorf("missing error containing %q; errors=%v", sub, errs)
				}
			}
			for _, sub := range tt.wantWarns {
				if !containsSub(warns, sub) {
					t.Errorf("missing warning containing %q; warnings=%v", sub, warns)
				}
			}
			if len(tt.wantErrors) == 0 && len(errs) != 0 {
				t.Errorf("unexpected errors: %v", errs)
			}
			if len(tt.wantWarns) == 0 && len(warns) != 0 {
				t.Errorf("unexpected warnings: %v", warns)
			}
		})
	}
}

func TestValidateDeterministic(t *testing.T) {
	cfg := Default()
	cfg.ReviewReadiness.MaxFilesChanged = -1
	cfg.Ownership.Dialect = "svn"
	cfg.SensitivePaths = []string{"[", "*.md"}
	a := Validate(cfg)
	b := Validate(cfg)
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Validate not deterministic: %v vs %v", a, b)
	}
}

func containsSub(entries []string, sub string) bool {
	for _, e := range entries {
		if strings.Contains(e, sub) {
			return true
		}
	}
	return false
}

// testdataPath returns an absolute path to a fixture under testdata/.
func testdataPath(name string) string {
	abs, err := filepath.Abs(filepath.Join("testdata", name))
	if err != nil {
		panic(err)
	}
	return abs
}

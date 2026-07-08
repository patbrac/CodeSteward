package tests

import (
	"reflect"
	"testing"

	"github.com/codesteward-ai/codesteward/internal/config"
	"github.com/codesteward-ai/codesteward/pkg/model"
)

const testRoot = "testdata/repo"

// defaultTestsCfg returns the plan's default tests configuration.
func defaultTestsCfg() config.TestsConfig {
	return config.Default().Tests
}

// baseOpts builds Options from the default config with the testdata root and
// analysis enabled.
func baseOpts() Options {
	c := defaultTestsCfg()
	return Options{
		RequireFor: c.RequireFor,
		TestPaths:  c.TestPaths,
		Mappings:   c.PathMappings,
		Root:       testRoot,
		Enabled:    true,
	}
}

func prod(path string) model.ChangedFile {
	return model.ChangedFile{Path: path, Status: "modified", IsProduction: true}
}

func testFile(path string) model.ChangedFile {
	return model.ChangedFile{Path: path, Status: "modified", IsTest: true}
}

// ruleIDs returns the as-emitted rule IDs of the findings, or nil if there are
// none.
func ruleIDs(fs []model.Finding) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.RuleID)
	}
	return out
}

// findingFor returns the first finding with the given rule id.
func findingFor(fs []model.Finding, id string) (model.Finding, bool) {
	for _, f := range fs {
		if f.RuleID == id {
			return f, true
		}
	}
	return model.Finding{}, false
}

// fileState returns the per-file state for path within a summary.
func fileState(s model.TestsSummary, path string) (model.FileTestExpectation, bool) {
	for _, e := range s.Files {
		if e.Path == path {
			return e, true
		}
	}
	return model.FileTestExpectation{}, false
}

func TestAnalyze(t *testing.T) {
	type check struct {
		wantState    model.TestState
		wantRuleIDs  []string        // exact set of finding rule IDs, in emitted order
		wantFileErr  string          // path whose file expectation to inspect ("" to skip)
		wantFileSt   model.TestState // expected state for wantFileErr
		wantMatched  string          // expected MatchedTest for wantFileErr ("" to skip check)
		wantCandidat []string        // expected Candidates for wantFileErr (nil to skip)
		wantNoFiles  bool            // summary.Files must be empty
	}

	tests := []struct {
		name  string
		files []model.ChangedFile
		opts  func() Options
		check check
	}{
		{
			name:  "disabled",
			files: []model.ChangedFile{prod("src/parser/tokenize.ts")},
			opts: func() Options {
				o := baseOpts()
				o.Enabled = false
				return o
			},
			check: check{wantState: model.TestsNotEvaluated, wantNoFiles: true},
		},
		{
			name:  "no_mappings",
			files: []model.ChangedFile{prod("src/parser/tokenize.ts")},
			opts: func() Options {
				o := baseOpts()
				o.Mappings = nil
				return o
			},
			check: check{wantState: model.TestsNotEvaluated, wantNoFiles: true},
		},
		{
			name:  "not_required_unmatched_path",
			files: []model.ChangedFile{{Path: "docs/guide.md", Status: "modified"}},
			opts:  baseOpts,
			check: check{wantState: model.TestsNotRequired, wantNoFiles: true},
		},
		{
			name: "not_required_all_filtered",
			files: []model.ChangedFile{
				testFile("src/a.ts"),                                    // IsTest -> skipped
				{Path: "src/b.ts", Status: "deleted"},                   // deleted -> skipped
				{Path: "src/c.ts", Status: "modified", IsBinary: true},  // binary -> skipped
				{Path: "src/d.ts", Status: "modified", IsIgnored: true}, // ignored -> skipped
			},
			opts:  baseOpts,
			check: check{wantState: model.TestsNotRequired, wantNoFiles: true},
		},
		{
			name: "matching_test_changed",
			files: []model.ChangedFile{
				prod("src/parser/tokenize.ts"),
				testFile("tests/parser/tokenize.test.ts"),
			},
			opts: baseOpts,
			check: check{
				wantState:   model.TestsMatchingChanged,
				wantRuleIDs: nil,
				wantFileErr: "src/parser/tokenize.ts",
				wantFileSt:  model.TestsMatchingChanged,
				wantMatched: "tests/parser/tokenize.test.ts",
			},
		},
		{
			name:  "existing_test_not_changed",
			files: []model.ChangedFile{prod("src/parser/tokenize.ts")},
			opts:  baseOpts,
			check: check{
				wantState:   model.TestsExistingNotChanged,
				wantRuleIDs: []string{model.RuleTstNotUpdated},
				wantFileErr: "src/parser/tokenize.ts",
				wantFileSt:  model.TestsExistingNotChanged,
				wantMatched: "tests/parser/tokenize.test.ts",
			},
		},
		{
			name:  "missing_matching_test",
			files: []model.ChangedFile{prod("src/runtime/cache.ts")},
			opts:  baseOpts,
			check: check{
				wantState:   model.TestsMissingMatching,
				wantRuleIDs: []string{model.RuleTstMissing},
				wantFileErr: "src/runtime/cache.ts",
				wantFileSt:  model.TestsMissingMatching,
			},
		},
		{
			name:  "nested_path",
			files: []model.ChangedFile{prod("src/a/b/c/widget.ts")},
			opts:  baseOpts,
			check: check{
				wantState:   model.TestsMissingMatching,
				wantRuleIDs: []string{model.RuleTstMissing},
				wantFileErr: "src/a/b/c/widget.ts",
				wantFileSt:  model.TestsMissingMatching,
				wantCandidat: []string{
					"src/a/b/c/widget.spec.ts",
					"src/a/b/c/widget.test.ts",
					"tests/a/b/c/widget.spec.ts",
					"tests/a/b/c/widget.test.ts",
				},
			},
		},
		{
			name:  "root_path_empty_collapses_slashes",
			files: []model.ChangedFile{prod("src/index.ts")},
			opts:  baseOpts,
			check: check{
				wantState:   model.TestsMissingMatching,
				wantRuleIDs: []string{model.RuleTstMissing},
				wantFileErr: "src/index.ts",
				wantFileSt:  model.TestsMissingMatching,
				wantCandidat: []string{
					"src/index.spec.ts",
					"src/index.test.ts",
					"tests/index.spec.ts",
					"tests/index.test.ts",
				},
			},
		},
		{
			name: "multidot_filename_changed_candidate",
			files: []model.ChangedFile{
				prod("src/date.util.ts"),
				testFile("src/date.util.test.ts"),
			},
			opts: baseOpts,
			check: check{
				wantState:   model.TestsMatchingChanged,
				wantFileErr: "src/date.util.ts",
				wantFileSt:  model.TestsMatchingChanged,
				wantMatched: "src/date.util.test.ts",
				wantCandidat: []string{
					"src/date.util.spec.ts",
					"src/date.util.test.ts",
					"tests/date.util.spec.ts",
					"tests/date.util.test.ts",
				},
			},
		},
		{
			name:  "require_for_but_no_from_template",
			files: []model.ChangedFile{prod("lib/thing.ts")},
			opts:  baseOpts,
			check: check{
				wantState:   model.TestsNotEvaluated,
				wantRuleIDs: []string{model.RuleTstUnresolved},
				wantFileErr: "lib/thing.ts",
				wantFileSt:  model.TestsNotEvaluated,
			},
		},
		{
			name:  "custom_mapping_override",
			files: []model.ChangedFile{prod("lib/foo.ts")},
			opts: func() Options {
				o := baseOpts()
				o.Mappings = []config.PathMapping{
					{From: "lib/{path}/{name}.{ext}", Expect: []string{"test/{path}/{name}_test.{ext}"}},
				}
				return o
			},
			check: check{
				wantState:    model.TestsMissingMatching,
				wantRuleIDs:  []string{model.RuleTstMissing},
				wantFileErr:  "lib/foo.ts",
				wantFileSt:   model.TestsMissingMatching,
				wantCandidat: []string{"test/foo_test.ts"},
			},
		},
		{
			name:  "union_across_mappings_deduped",
			files: []model.ChangedFile{prod("src/foo.ts")},
			opts: func() Options {
				o := baseOpts()
				o.Mappings = []config.PathMapping{
					{From: "src/{path}/{name}.{ext}", Expect: []string{"tests/{name}.test.{ext}"}},
					{From: "src/{path}/{name}.{ext}", Expect: []string{"tests/{name}.test.{ext}", "tests/{name}.spec.{ext}"}},
				}
				return o
			},
			check: check{
				wantState:    model.TestsMissingMatching,
				wantRuleIDs:  []string{model.RuleTstMissing},
				wantFileErr:  "src/foo.ts",
				wantFileSt:   model.TestsMissingMatching,
				wantCandidat: []string{"tests/foo.spec.ts", "tests/foo.test.ts"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			summary, findings := Analyze(tc.files, tc.opts())

			if summary.State != tc.check.wantState {
				t.Errorf("summary state = %q, want %q", summary.State, tc.check.wantState)
			}
			if tc.check.wantNoFiles && len(summary.Files) != 0 {
				t.Errorf("summary.Files = %+v, want empty", summary.Files)
			}
			if !reflect.DeepEqual(ruleIDs(findings), tc.check.wantRuleIDs) {
				t.Errorf("finding rule IDs = %v, want %v", ruleIDs(findings), tc.check.wantRuleIDs)
			}
			if tc.check.wantFileErr != "" {
				fe, ok := fileState(summary, tc.check.wantFileErr)
				if !ok {
					t.Fatalf("no file expectation for %q; files=%+v", tc.check.wantFileErr, summary.Files)
				}
				if fe.State != tc.check.wantFileSt {
					t.Errorf("file %q state = %q, want %q", tc.check.wantFileErr, fe.State, tc.check.wantFileSt)
				}
				if tc.check.wantMatched != "" && fe.MatchedTest != tc.check.wantMatched {
					t.Errorf("file %q matched test = %q, want %q", tc.check.wantFileErr, fe.MatchedTest, tc.check.wantMatched)
				}
				if tc.check.wantCandidat != nil && !reflect.DeepEqual(fe.Candidates, tc.check.wantCandidat) {
					t.Errorf("file %q candidates = %v, want %v", tc.check.wantFileErr, fe.Candidates, tc.check.wantCandidat)
				}
			}
		})
	}
}

func TestFindingMessages(t *testing.T) {
	// CS-TST-001 message/action.
	_, findings := Analyze([]model.ChangedFile{prod("src/runtime/cache.ts")}, baseOpts())
	f, ok := findingFor(findings, model.RuleTstMissing)
	if !ok {
		t.Fatalf("expected CS-TST-001 finding, got %v", ruleIDs(findings))
	}
	if f.Severity != model.SeverityActionRequired {
		t.Errorf("CS-TST-001 severity = %q, want action_required", f.Severity)
	}
	wantMsg := "`src/runtime/cache.ts` changed, but no matching test file was changed."
	if f.Message != wantMsg {
		t.Errorf("CS-TST-001 message = %q, want %q", f.Message, wantMsg)
	}
	wantAct := "Add or update matching tests for `src/runtime/cache.ts`."
	if f.Action != wantAct {
		t.Errorf("CS-TST-001 action = %q, want %q", f.Action, wantAct)
	}
	if !reflect.DeepEqual(f.Paths, []string{"src/runtime/cache.ts"}) {
		t.Errorf("CS-TST-001 paths = %v, want [src/runtime/cache.ts]", f.Paths)
	}

	// CS-TST-002 message/action (matching test exists on disk but unchanged).
	s2, findings2 := Analyze([]model.ChangedFile{prod("src/parser/tokenize.ts")}, baseOpts())
	if s2.State != model.TestsExistingNotChanged {
		t.Fatalf("state = %q, want existing_test_found_but_not_changed", s2.State)
	}
	f2, ok := findingFor(findings2, model.RuleTstNotUpdated)
	if !ok {
		t.Fatalf("expected CS-TST-002 finding, got %v", ruleIDs(findings2))
	}
	if f2.Severity != model.SeverityWarning {
		t.Errorf("CS-TST-002 severity = %q, want warning", f2.Severity)
	}
	wantMsg2 := "A matching test exists for `src/parser/tokenize.ts` (`tests/parser/tokenize.test.ts`), but this PR did not update it."
	if f2.Message != wantMsg2 {
		t.Errorf("CS-TST-002 message = %q, want %q", f2.Message, wantMsg2)
	}
	if f2.Action != "Update `tests/parser/tokenize.test.ts` to cover the changes in `src/parser/tokenize.ts`." {
		t.Errorf("CS-TST-002 action = %q", f2.Action)
	}

	// CS-TST-003 severity is info.
	_, findings3 := Analyze([]model.ChangedFile{prod("lib/thing.ts")}, baseOpts())
	f3, ok := findingFor(findings3, model.RuleTstUnresolved)
	if !ok {
		t.Fatalf("expected CS-TST-003 finding, got %v", ruleIDs(findings3))
	}
	if f3.Severity != model.SeverityInfo {
		t.Errorf("CS-TST-003 severity = %q, want info", f3.Severity)
	}
}

func TestAggregatePrecedence(t *testing.T) {
	// A missing file and a matching file together must aggregate to missing.
	files := []model.ChangedFile{
		prod("src/runtime/cache.ts"), // missing
		prod("src/parser/tokenize.ts"),
		testFile("tests/parser/tokenize.test.ts"), // makes tokenize matching
	}
	s, _ := Analyze(files, baseOpts())
	if s.State != model.TestsMissingMatching {
		t.Errorf("aggregate state = %q, want missing_matching_test", s.State)
	}

	// existing beats matching.
	files2 := []model.ChangedFile{
		prod("src/parser/tokenize.ts"), // existing on disk, not changed
		prod("src/other.ts"),
		testFile("tests/other.test.ts"), // matching
	}
	s2, _ := Analyze(files2, baseOpts())
	if s2.State != model.TestsExistingNotChanged {
		t.Errorf("aggregate state = %q, want existing_test_found_but_not_changed", s2.State)
	}
}

func TestCompileFromExtraction(t *testing.T) {
	tests := []struct {
		from      string
		path      string
		wantMatch bool
		wantP     string
		wantName  string
		wantExt   string
	}{
		{"src/{path}/{name}.{ext}", "src/tokenize.ts", true, "", "tokenize", "ts"},
		{"src/{path}/{name}.{ext}", "src/parser/tokenize.ts", true, "parser", "tokenize", "ts"},
		{"src/{path}/{name}.{ext}", "src/a/b/c/widget.ts", true, "a/b/c", "widget", "ts"},
		{"src/{path}/{name}.{ext}", "src/date.util.ts", true, "", "date.util", "ts"},
		{"src/{path}/{name}.{ext}", "lib/thing.ts", false, "", "", ""},
		{"src/{path}/{name}.{ext}", "src/nodot", false, "", "", ""},
		{"lib/{path}/{name}.{ext}", "lib/x/y/foo.tsx", true, "x/y", "foo", "tsx"},
	}
	for _, tc := range tests {
		re, order := compileFrom(tc.from)
		if re == nil {
			t.Fatalf("compileFrom(%q) returned nil regexp", tc.from)
		}
		sub := re.FindStringSubmatch(tc.path)
		if (sub != nil) != tc.wantMatch {
			t.Errorf("%q vs %q: match=%v, want %v", tc.from, tc.path, sub != nil, tc.wantMatch)
			continue
		}
		if !tc.wantMatch {
			continue
		}
		var gotP, gotName, gotExt string
		for k, kind := range order {
			switch kind {
			case phPath:
				gotP = sub[k+1]
			case phName:
				gotName = sub[k+1]
			case phExt:
				gotExt = sub[k+1]
			}
		}
		if gotP != tc.wantP || gotName != tc.wantName || gotExt != tc.wantExt {
			t.Errorf("%q vs %q: got path=%q name=%q ext=%q; want path=%q name=%q ext=%q",
				tc.from, tc.path, gotP, gotName, gotExt, tc.wantP, tc.wantName, tc.wantExt)
		}
	}
}

func TestExpand(t *testing.T) {
	tests := []struct {
		tmpl, path, name, ext, want string
	}{
		{"tests/{path}/{name}.test.{ext}", "parser", "tokenize", "ts", "tests/parser/tokenize.test.ts"},
		{"tests/{path}/{name}.test.{ext}", "", "index", "ts", "tests/index.test.ts"},
		{"src/{path}/{name}.spec.{ext}", "", "foo", "ts", "src/foo.spec.ts"},
		{"test/{path}/{name}_test.{ext}", "", "foo", "ts", "test/foo_test.ts"},
		{"tests/{path}/{name}.test.{ext}", "a/b", "c", "tsx", "tests/a/b/c.test.tsx"},
	}
	for _, tc := range tests {
		if got := expand(tc.tmpl, tc.path, tc.name, tc.ext); got != tc.want {
			t.Errorf("expand(%q,%q,%q,%q) = %q, want %q", tc.tmpl, tc.path, tc.name, tc.ext, got, tc.want)
		}
	}
}

func TestDeterminism(t *testing.T) {
	files := []model.ChangedFile{
		prod("src/runtime/cache.ts"),
		prod("src/a/b/c/widget.ts"),
		prod("src/parser/tokenize.ts"),
		prod("lib/thing.ts"),
	}
	s1, f1 := Analyze(files, baseOpts())
	s2, f2 := Analyze(files, baseOpts())
	if !reflect.DeepEqual(s1, s2) {
		t.Errorf("summary not deterministic:\n%+v\n%+v", s1, s2)
	}
	if !reflect.DeepEqual(f1, f2) {
		t.Errorf("findings not deterministic:\n%+v\n%+v", f1, f2)
	}
}

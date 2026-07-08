package codeowners

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return b
}

// findRule returns the rule with the given pattern (first match), or nil.
func findRule(f *File, pattern string) *Rule {
	for i := range f.Rules {
		if f.Rules[i].Pattern == pattern {
			return &f.Rules[i]
		}
	}
	return nil
}

func TestMatchPatternSemantics(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Leading slash anchors to root.
		{"/config.yml", "config.yml", true},
		{"/config.yml", "sub/config.yml", false},
		{"/config.yml", "config.yml/nested", true}, // owns the dir's contents too
		// Slashless matches a basename at any depth.
		{"config.yml", "sub/config.yml", true},
		{"config.yml", "config.yml", true},
		{"build", "build", true},
		{"build", "a/build/c.js", true},
		{"build", "a/rebuild/c.js", false},
		// Internal (non-trailing) slash anchors.
		{"apps/web", "apps/web", true},
		{"apps/web", "apps/web/x.ts", true},
		{"apps/web", "x/apps/web", false},
		// Extension globs.
		{"*.js", "a.js", true},
		{"*.js", "x/y/a.js", true},
		{"*.js", "a.ts", false},
		{"file?.ts", "file1.ts", true},
		{"file?.ts", "dir/file1.ts", true},
		{"file?.ts", "fileAB.ts", false},
		// Trailing slash = directory contents.
		{"foo/", "foo/x", true},
		{"foo/", "a/foo/b", true},
		{"foo/", "foo", false},
		{"/src/parser/", "src/parser/tokenize.ts", true},
		{"/src/parser/", "src/parser", false},
		{"/src/parser/", "src/other.ts", false},
		{"/docs/", "docs/a.md", true},
		{"/docs/", "a/docs/b.md", false},
		// Anchored wildcard-terminated patterns match a single level only
		// (they must NOT recurse, unlike concrete final segments which own the
		// subtree). Mirrors internal/globs and GitHub/GitLab CODEOWNERS.
		{"docs/*", "docs/getting-started.md", true},
		{"docs/*", "docs/build-app/troubleshooting.md", false},
		{"/src/*", "src/a.ts", true},
		{"/src/*", "src/nested/a.ts", false},
		{"/src/*", "src", false},
		{"src/file?.ts", "src/file1.ts", true},
		{"src/file?.ts", "src/dir/file1.ts", false},
		{"/src/a[bc].ts", "src/ab.ts", true},
		{"/src/a[bc].ts", "src/ab.ts/deeper", false},
		// A concrete final segment still owns its directory subtree.
		{"/apps/github", "apps/github", true},
		{"/apps/github", "apps/github/x.ts", true},
		{"/apps/github", "apps/github/deep/nested.ts", true},
		// A wildcard in a non-final segment is fine as long as the final
		// segment is concrete (owns the subtree of the concrete leaf).
		{"src/*/foo.ts", "src/x/foo.ts", true},
		{"src/*/foo.ts", "src/x/foo.ts/gen.ts", true},
		{"src/*/foo.ts", "src/x/y/foo.ts", false},
		// Doublestar.
		{"docs/**", "docs/a/b.md", true},
		{"docs/**", "docs", false},
		{"docs/**", "docsx/a", false},
		{"/src/**/test.ts", "src/a/b/test.ts", true},
		{"/src/**/test.ts", "src/test.ts", true}, // ** matches zero segments
		{"/src/**/test.ts", "src/a/test.ts", true},
		{"/src/**/test.ts", "lib/test.ts", false},
		// Catch-all patterns.
		{"*", "anything/deep.ts", true},
		{"**", "anything/deep.ts", true},
		{"/", "x/y", true},
		{"/**", "x/y", true},
	}
	for _, tt := range tests {
		if got := matchPattern(tt.pattern, tt.path); got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
		}
	}
}

// TestWildcardTerminatedAnchoredSingleLevel guards against the regression where
// an anchored pattern whose final segment is a wildcard ("docs/*") over-matched
// files nested more than one directory level deep, mis-routing ownership. A
// nested file must fall through to the catch-all rule instead of the wildcard
// rule, in both dialects (the matcher is shared).
func TestWildcardTerminatedAnchoredSingleLevel(t *testing.T) {
	for _, dialect := range []Dialect{DialectGitHub, DialectGitLab} {
		content := []byte("* @root\ndocs/* @docs-team\n")
		f := Parse(content, "CODEOWNERS", dialect)

		// Single-level file under docs/ is owned by @docs-team.
		assertMatch(t, f, "docs/getting-started.md", true, []string{"@docs-team"}, "docs/*")
		// Deeper file is NOT captured by docs/*; the catch-all wins.
		assertMatch(t, f, "docs/build-app/troubleshooting.md", true, []string{"@root"}, "*")
	}
}

func TestClassifyPattern(t *testing.T) {
	tests := []struct {
		pattern string
		want    model.OwnershipMatchClass
	}{
		{"*", model.MatchFallback},
		{"**", model.MatchFallback},
		{"/**", model.MatchFallback},
		{"/", model.MatchFallback},
		{"**/**", model.MatchFallback},
		{"/src/**", model.MatchBroad},
		{"/src/", model.MatchBroad},
		{"src/", model.MatchBroad},
		{"*.md", model.MatchBroad},
		{"*.js", model.MatchBroad},
		{"README.md", model.MatchBroad},
		{"/src/parser/**", model.MatchSpecific},
		{"/src/parser/", model.MatchSpecific},
		{"/src/parser/tokenize.ts", model.MatchSpecific},
		{"docs/usage.md", model.MatchSpecific},
	}
	for _, tt := range tests {
		if got := ClassifyPattern(tt.pattern); got != tt.want {
			t.Errorf("ClassifyPattern(%q) = %v, want %v", tt.pattern, got, tt.want)
		}
	}
}

func TestValidOwner(t *testing.T) {
	valid := []string{
		"@alice", "@acme/parser-team", "dev@example.com",
		"@a", "@user-name", "@org/sub.team", "first.last@sub.example.co",
	}
	invalid := []string{
		"alice", "no-at-owner", "bad@@x", "@", "@-bad", "@bad-",
		"@org/", "just text", "user@nodot",
	}
	for _, o := range valid {
		if !validOwner(o) {
			t.Errorf("validOwner(%q) = false, want true", o)
		}
	}
	for _, o := range invalid {
		if validOwner(o) {
			t.Errorf("validOwner(%q) = true, want false", o)
		}
	}
}

func TestTokenizeFields(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"/src/foo.ts @a @b", []string{"/src/foo.ts", "@a", "@b"}},
		{"  /src/foo.ts   @a", []string{"/src/foo.ts", "@a"}},
		{`/src/a\ b.ts @team`, []string{"/src/a b.ts", "@team"}},
		{"/src/foo.ts @a # trailing comment", []string{"/src/foo.ts", "@a"}},
		{"@a#b @c", []string{"@a#b", "@c"}}, // '#' inside token is literal
		{"/path @a #c", []string{"/path", "@a"}},
		{"# whole line comment", nil},
		{"", nil},
		{`a\#b @a`, []string{"a#b", "@a"}}, // escaped '#'
	}
	for _, tt := range tests {
		got := tokenizeFields(tt.in)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("tokenizeFields(%q) = %#v, want %#v", tt.in, got, tt.want)
		}
	}
}

func assertMatch(t *testing.T, f *File, path string, wantFound bool, wantOwners []string, wantPattern string) {
	t.Helper()
	m := f.Match(path)
	if m.Found != wantFound {
		t.Errorf("Match(%q).Found = %v, want %v (%+v)", path, m.Found, wantFound, m)
	}
	if wantOwners != nil && !reflect.DeepEqual(m.Owners, wantOwners) {
		t.Errorf("Match(%q).Owners = %#v, want %#v", path, m.Owners, wantOwners)
	}
	if wantPattern != "" && m.Pattern != wantPattern {
		t.Errorf("Match(%q).Pattern = %q, want %q", path, m.Pattern, wantPattern)
	}
}

func TestParseGitHubRealisticAndLastMatchWins(t *testing.T) {
	f := Parse(readFixture(t, "github_realistic.txt"), "CODEOWNERS", DialectGitHub)
	if len(f.Rules) != 7 {
		t.Fatalf("want 7 rules, got %d: %+v", len(f.Rules), f.Rules)
	}
	if len(f.Warnings) != 0 {
		t.Fatalf("want no warnings, got %+v", f.Warnings)
	}

	// The very specific rule appears AFTER the directory rule and wins.
	assertMatch(t, f, "src/parser/tokenize.ts", true, []string{"@tokenizer-guru"}, "/src/parser/tokenize.ts")
	// Directory rule wins for other files in that directory.
	assertMatch(t, f, "src/parser/parse.ts", true, []string{"@acme/parser-team", "@alice"}, "/src/parser/")
	assertMatch(t, f, "src/public/index.ts", true, []string{"@api-team"}, "/src/public/")
	// Only the catch-all matches -> fallback.
	assertMatch(t, f, "src/runtime/cache.ts", true, []string{"@acme/maintainers"}, "*")
	// The slashless *.md rule (later) wins over the catch-all.
	assertMatch(t, f, "README.md", true, []string{"@docs-team"}, "*.md")
	// Explicitly unowned: last matching rule has zero owners.
	m := f.Match("generated/out.js")
	if m.Found {
		t.Errorf("generated/out.js: want Found=false, got %+v", m)
	}
	if m.Pattern != "/generated/" {
		t.Errorf("generated/out.js: want Pattern=/generated/, got %q", m.Pattern)
	}

	// Class checks.
	if c := f.Match("src/parser/parse.ts").Class; c != model.MatchSpecific {
		t.Errorf("parse.ts class = %v, want specific", c)
	}
	if c := f.Match("src/runtime/cache.ts").Class; c != model.MatchFallback {
		t.Errorf("cache.ts class = %v, want fallback", c)
	}
}

// TestGitHubLastMatchWinsOverride shows a catch-all placed AFTER a specific
// rule overrides it (pure last-match-wins semantics).
func TestGitHubLastMatchWinsOverride(t *testing.T) {
	content := []byte("/src/parser/ @parser-team\n* @maintainers\n")
	f := Parse(content, "CODEOWNERS", DialectGitHub)
	// The catch-all is last, so it wins even for the parser directory.
	assertMatch(t, f, "src/parser/tokenize.ts", true, []string{"@maintainers"}, "*")
}

func TestParseGitLabSections(t *testing.T) {
	f := Parse(readFixture(t, "gitlab_sections.txt"), "CODEOWNERS", DialectGitLab)
	if f.Dialect != DialectGitLab {
		t.Fatalf("dialect = %v, want gitlab", f.Dialect)
	}
	if !f.optionalSections["optional reviewers"] {
		t.Errorf("optional section not recorded: %+v", f.optionalSections)
	}

	// Section default owners inherited by owner-less rules.
	if r := findRule(f, "docs/"); r == nil || !reflect.DeepEqual(r.Owners, []string{"@docs-team"}) {
		t.Errorf("docs/ rule owners = %+v, want [@docs-team]", r)
	}
	if r := findRule(f, "/src/api/"); r == nil || !reflect.DeepEqual(r.Owners, []string{"@backend-team"}) {
		t.Errorf("/src/api/ rule owners = %+v, want [@backend-team]", r)
	}

	// Union across sections (Backend payments-team + Security sec-team), sorted.
	assertMatch(t, f, "src/api/payments/charge.go", true,
		[]string{"@payments-team", "@sec-team"}, "/src/api/payments/")
	// Backend default owner only.
	assertMatch(t, f, "src/api/users.go", true, []string{"@backend-team"}, "/src/api/")
	// Documentation section (last-in-section is *.md).
	assertMatch(t, f, "docs/guide.md", true, []string{"@docs-team"}, "")
	assertMatch(t, f, "README.md", true, []string{"@docs-team"}, "*.md")
	// Only the optional section covers this -> excluded -> unowned.
	m := f.Match("src/lib/util.go")
	if m.Found {
		t.Errorf("src/lib/util.go: optional section must not count, got %+v", m)
	}
}

func TestParsePathologicalAndValidate(t *testing.T) {
	f := Parse(readFixture(t, "pathological.txt"), "CODEOWNERS", DialectGitHub)

	// Rules kept: /src/good.ts, @justowners (empty), /lib/ (empty), *.
	if len(f.Rules) != 4 {
		t.Fatalf("want 4 rules, got %d: %+v", len(f.Rules), f.Rules)
	}
	if findRule(f, "src/bad[.ts") != nil {
		t.Errorf("invalid-pattern rule should have been dropped")
	}
	if findRule(f, "!/src/secret.ts") != nil {
		t.Errorf("negation rule should have been dropped")
	}

	problems := Validate(f)
	// 6 line-level problems plus the catch-all-after-specific ordering warning
	// for "* @maintainers" on line 7 (it follows /src/good.ts and /lib/).
	if len(problems) != 7 {
		t.Fatalf("want 7 validate problems, got %d: %#v", len(problems), problems)
	}
	// There must be exactly one error (the invalid glob pattern).
	errCount := 0
	for _, p := range problems {
		if strings.HasPrefix(p, "error:") {
			errCount++
		}
	}
	if errCount != 1 {
		t.Errorf("want 1 error problem, got %d: %#v", errCount, problems)
	}
	joined := strings.Join(problems, "\n")
	for _, want := range []string{
		"line 2:", "negation", "line 4:", "invalid pattern",
		"line 5:", "no owners", "line 7:", "invalid owner", "line 8:", "section headers",
		"catch-all pattern", "the last matching rule wins",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("validate output missing %q:\n%s", want, joined)
		}
	}
	// Determinism: problems must already be line-ordered.
	if !strings.Contains(problems[0], "line 2:") || !strings.Contains(problems[len(problems)-1], "line 8:") {
		t.Errorf("validate output not line-ordered: %#v", problems)
	}
}

func TestParseEmailOwners(t *testing.T) {
	f := Parse(readFixture(t, "email_owners.txt"), "CODEOWNERS", DialectGitHub)
	if r := findRule(f, "/src/"); r == nil || !reflect.DeepEqual(r.Owners, []string{"alice@example.com", "@bob"}) {
		t.Errorf("/src/ owners = %+v, want [alice@example.com @bob]", r)
	}
	// Two malformed owners on the /docs/ line.
	malformed := 0
	for _, w := range f.Warnings {
		if w.kind == kindMalformedOwner {
			malformed++
		}
	}
	if malformed != 2 {
		t.Errorf("want 2 malformed-owner warnings, got %d: %+v", malformed, f.Warnings)
	}
	assertMatch(t, f, "src/app.ts", true, []string{"alice@example.com", "@bob"}, "/src/")
	assertMatch(t, f, "README.md", true, []string{"dev@example.com"}, "*")
}

func TestParseUnicodeAndEscapedSpace(t *testing.T) {
	f := Parse(readFixture(t, "unicode_paths.txt"), "CODEOWNERS", DialectGitHub)
	if len(f.Warnings) != 0 {
		t.Fatalf("unexpected warnings: %+v", f.Warnings)
	}
	if r := findRule(f, "/src/my file.ts"); r == nil {
		t.Fatalf("escaped-space pattern not parsed; rules: %+v", f.Rules)
	}
	assertMatch(t, f, "café/menu.md", true, []string{"@cafe-team"}, "/café/")
	assertMatch(t, f, "src/naïve.ts", true, []string{"@unicode-team"}, "/src/naïve.ts")
	assertMatch(t, f, "日本語/readme.md", true, []string{"@jp-team"}, "/日本語/")
	assertMatch(t, f, "src/my file.ts", true, []string{"@spaced-team"}, "/src/my file.ts")
}

func TestDiscover(t *testing.T) {
	write := func(dir, rel string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("* @a\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("github prefers .github", func(t *testing.T) {
		root := t.TempDir()
		write(root, ".github/CODEOWNERS")
		write(root, "CODEOWNERS")
		got, err := Discover(root, DialectGitHub)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(root, ".github", "CODEOWNERS") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("gitlab order skips .github", func(t *testing.T) {
		root := t.TempDir()
		write(root, ".github/CODEOWNERS") // ignored by gitlab dialect
		write(root, "docs/CODEOWNERS")
		got, err := Discover(root, DialectGitLab)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(root, "docs", "CODEOWNERS") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("auto union prefers .github", func(t *testing.T) {
		root := t.TempDir()
		write(root, ".gitlab/CODEOWNERS")
		write(root, ".github/CODEOWNERS")
		got, err := Discover(root, DialectAuto)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(root, ".github", "CODEOWNERS") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("auto finds gitlab-only", func(t *testing.T) {
		root := t.TempDir()
		write(root, ".gitlab/CODEOWNERS")
		got, err := Discover(root, DialectAuto)
		if err != nil {
			t.Fatal(err)
		}
		if got != filepath.Join(root, ".gitlab", "CODEOWNERS") {
			t.Errorf("got %q", got)
		}
	})

	t.Run("none returns empty and nil", func(t *testing.T) {
		got, err := Discover(t.TempDir(), DialectAuto)
		if err != nil || got != "" {
			t.Errorf("got (%q, %v), want (\"\", nil)", got, err)
		}
	})

	t.Run("unknown dialect errors", func(t *testing.T) {
		if _, err := Discover(t.TempDir(), Dialect("nope")); err == nil {
			t.Errorf("want error for unknown dialect")
		}
	})
}

func TestParseFile(t *testing.T) {
	root := t.TempDir()
	full := filepath.Join(root, "CODEOWNERS")
	if err := os.WriteFile(full, []byte("* @maintainers\n/src/ @team\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := ParseFile(full, DialectGitHub)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(f.Rules))
	}
	if _, err := ParseFile(filepath.Join(root, "missing"), DialectGitHub); err == nil {
		t.Errorf("want error for missing file")
	}
}

func TestAutoDialectResolution(t *testing.T) {
	tests := []struct {
		sourcePath string
		wantDial   Dialect
	}{
		{".github/CODEOWNERS", DialectGitHub},
		{"/repo/.gitlab/CODEOWNERS", DialectGitLab},
		{"CODEOWNERS", DialectGitHub}, // lenient github at root
		{"docs/CODEOWNERS", DialectGitHub},
	}
	for _, tt := range tests {
		f := Parse([]byte("* @a\n"), tt.sourcePath, DialectAuto)
		if f.Dialect != tt.wantDial {
			t.Errorf("Parse(auto, %q).Dialect = %v, want %v", tt.sourcePath, f.Dialect, tt.wantDial)
		}
	}

	// A root file with sections under auto uses github semantics but warns.
	f := Parse([]byte("[Backend] @team\n/src/ @a\n"), "CODEOWNERS", DialectAuto)
	if f.Dialect != DialectGitHub {
		t.Fatalf("want github (lenient), got %v", f.Dialect)
	}
	if len(f.Warnings) == 0 {
		t.Errorf("expected a lenient-section warning")
	}
}

func TestGithubDialectRejectsSections(t *testing.T) {
	f := Parse([]byte("[Backend] @team\n/src/ @a\n"), ".github/CODEOWNERS", DialectGitHub)
	// Section header is warned and skipped; the rule has no inherited section.
	if r := findRule(f, "/src/"); r == nil || r.Section != "" {
		t.Errorf("github rule should have empty section, got %+v", r)
	}
	found := false
	for _, w := range f.Warnings {
		if w.kind == kindSectionInGithub {
			found = true
		}
	}
	if !found {
		t.Errorf("expected section-in-github warning, got %+v", f.Warnings)
	}
}

func TestInlineCommentRequiresWhitespace(t *testing.T) {
	// '#' immediately following a token is literal; '#' after whitespace is a comment.
	f := Parse([]byte("/a#b @team # note\n"), "CODEOWNERS", DialectGitHub)
	if len(f.Rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(f.Rules))
	}
	if f.Rules[0].Pattern != "/a#b" {
		t.Errorf("pattern = %q, want /a#b", f.Rules[0].Pattern)
	}
	if !reflect.DeepEqual(f.Rules[0].Owners, []string{"@team"}) {
		t.Errorf("owners = %+v, want [@team]", f.Rules[0].Owners)
	}
}

func TestDeterminism(t *testing.T) {
	content := readFixture(t, "gitlab_sections.txt")
	a := Parse(content, "CODEOWNERS", DialectGitLab)
	b := Parse(content, "CODEOWNERS", DialectGitLab)
	if !reflect.DeepEqual(a.Rules, b.Rules) {
		t.Errorf("rules differ across parses")
	}
	// Match must be repeatable (map iteration order must not leak).
	for i := 0; i < 20; i++ {
		m := a.Match("src/api/payments/charge.go")
		if !reflect.DeepEqual(m.Owners, []string{"@payments-team", "@sec-team"}) {
			t.Fatalf("nondeterministic owners on iter %d: %#v", i, m.Owners)
		}
	}
}

func TestGitLabSectionCaseInsensitive(t *testing.T) {
	// The two headers name the same section despite differing case, so their
	// last-match-within-section resolves across both, and the "^[backend]"
	// optional marker applies to the whole section.
	content := []byte("[Backend] @team-a\n/src/ \n[BACKEND]\n/src/api/ @team-b\n")
	f := Parse(content, "CODEOWNERS", DialectGitLab)
	// /src/api/x.go: both rules are in the same (case-folded) section; the
	// later, more specific rule wins within the section.
	assertMatch(t, f, "src/api/x.go", true, []string{"@team-b"}, "/src/api/")
	// /src/other.go: only the first rule matches; it inherits the section default.
	assertMatch(t, f, "src/other.go", true, []string{"@team-a"}, "/src/")
}

func TestValidateNil(t *testing.T) {
	if Validate(nil) != nil {
		t.Errorf("Validate(nil) should be nil")
	}
}

// TestValidateCatchAllOrdering covers the warning that fires when a fallback
// (catch-all) rule is ordered after a more-specific rule that it would then
// silently override under last-match-wins. For github the check spans the whole
// file; for gitlab it is scoped to each section.
func TestValidateCatchAllOrdering(t *testing.T) {
	orderingWarnings := func(problems []string) []string {
		var out []string
		for _, p := range problems {
			if strings.Contains(p, "catch-all pattern") {
				out = append(out, p)
			}
		}
		return out
	}

	tests := []struct {
		name    string
		content string
		dialect Dialect
		want    int // expected number of ordering warnings
	}{
		{
			name:    "github catch-all after specific fires",
			content: "/src/parser/ @p\n* @maintainers\n",
			dialect: DialectGitHub,
			want:    1,
		},
		{
			name:    "github catch-all after broad fires",
			content: "/src/ @s\n* @maintainers\n",
			dialect: DialectGitHub,
			want:    1,
		},
		{
			name:    "github catch-all first is silent",
			content: "* @maintainers\n/src/parser/ @p\n",
			dialect: DialectGitHub,
			want:    0,
		},
		{
			name:    "github only catch-all is silent",
			content: "* @maintainers\n",
			dialect: DialectGitHub,
			want:    0,
		},
		{
			name:    "github two catch-alls after specific fire twice",
			content: "/src/ @s\n* @a\n** @b\n",
			dialect: DialectGitHub,
			want:    2,
		},
		{
			name:    "gitlab catch-all after specific in same section fires",
			content: "[Backend] @team\n/src/ @s\n* @m\n",
			dialect: DialectGitLab,
			want:    1,
		},
		{
			name:    "gitlab catch-all first in section is silent",
			content: "[Backend] @team\n* @m\n/src/ @s\n",
			dialect: DialectGitLab,
			want:    0,
		},
		{
			name:    "gitlab catch-all in a different later section is silent",
			content: "[Backend]\n/src/ @s\n[Fallback]\n* @m\n",
			dialect: DialectGitLab,
			want:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Parse([]byte(tt.content), "CODEOWNERS", tt.dialect)
			got := orderingWarnings(Validate(f))
			if len(got) != tt.want {
				t.Fatalf("ordering warnings = %d, want %d; got %#v", len(got), tt.want, got)
			}
			// Every ordering warning must follow the package prefix convention
			// and carry the corrective guidance.
			for _, w := range got {
				if !strings.HasPrefix(w, "warning: line ") {
					t.Errorf("ordering warning missing warning-prefix: %q", w)
				}
				if !strings.Contains(w, "consider moving it first") {
					t.Errorf("ordering warning missing guidance: %q", w)
				}
			}
		})
	}
}

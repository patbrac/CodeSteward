package globs

import "testing"

func TestMatch(t *testing.T) {
	cases := []struct {
		name        string
		pattern     string
		path        string
		wantMatched bool
		wantOK      bool
	}{
		// Exact paths.
		{"exact match", "src/main.go", "src/main.go", true, true},
		{"exact non-match", "src/main.go", "src/other.go", false, true},
		{"exact non-match different dir", "src/main.go", "lib/main.go", false, true},

		// "*" within a segment.
		{"star ext match", "src/*.go", "src/main.go", true, true},
		{"star does not cross separator", "src/*.go", "src/sub/main.go", false, true},
		{"star matches empty run", "src/main*", "src/main", true, true},
		{"star mid segment", "a*b", "axyzb", true, true},
		{"star mid segment no match", "a*b", "axyz", false, true},

		// "?" single non-separator char.
		{"question single char", "file?.txt", "file1.txt", true, true},
		{"question rejects two chars", "file?.txt", "file12.txt", false, true},
		{"question in path segment", "src/file?.go", "src/fileA.go", true, true},
		{"question slashless whole", "?", "a", true, true},
		{"question slashless rejects two", "?", "ab", false, true},

		// "**" at the start.
		{"doublestar start deep", "**/main.go", "src/a/main.go", true, true},
		{"doublestar start zero segments", "**/main.go", "main.go", true, true},
		{"doublestar start one segment", "**/main.go", "src/main.go", true, true},
		{"doublestar start no match", "**/main.go", "src/main.ts", false, true},

		// "**" in the middle.
		{"doublestar middle deep", "src/**/main.go", "src/a/b/main.go", true, true},
		{"doublestar middle zero", "src/**/main.go", "src/main.go", true, true},
		{"doublestar middle wrong prefix", "src/**/main.go", "lib/main.go", false, true},

		// "**" at the end.
		{"doublestar end one", "src/**", "src/a", true, true},
		{"doublestar end deep", "src/**", "src/a/b", true, true},
		{"doublestar end nested prefix", "src/parser/**", "src/parser/tokenize.ts", true, true},

		// "dir/**" does NOT match dir itself.
		{"dir doublestar not bare dir", "src/**", "src", false, true},
		{"dir doublestar wrong dir", "src/**", "lib/a", false, true},

		// Slashless patterns match basename anywhere AND whole path.
		{"slashless ext basename root", "*.md", "README.md", true, true},
		{"slashless ext basename nested", "*.md", "docs/README.md", true, true},
		{"slashless ext deep basename", "*.md", "docs/guide/intro.md", true, true},
		{"slashless ext non-match", "*.md", "docs/README.txt", false, true},
		{"slashless literal root", "package.json", "package.json", true, true},
		{"slashless literal nested", "package.json", "frontend/package.json", true, true},
		{"slashless literal non-match", "package.json", "frontend/package.jsonx", false, true},

		// Common config-style doublestar patterns.
		{"test glob dotted", "**/*.test.*", "tests/parser/tokenize.test.ts", true, true},
		{"spec glob dotted", "**/*.spec.*", "src/a.spec.js", true, true},
		{"workflows glob", ".github/workflows/**", ".github/workflows/release.yml", true, true},
		{"tests dir glob", "tests/**", "tests/parser/tokenize.test.ts", true, true},

		// Trailing "/" directory semantics.
		{"trailing slash under dir", "docs/", "docs/usage.md", true, true},
		{"trailing slash dir itself", "docs/", "docs", true, true},
		{"trailing slash deep", "docs/", "docs/a/b.md", true, true},
		{"trailing slash prefix guard", "docs/", "docsx/y.md", false, true},
		{"trailing slash nested pattern", "src/parser/", "src/parser/tokenize.ts", true, true},

		// Bare "*" and "**".
		{"bare doublestar matches deep", "**", "a/b/c", true, true},
		{"bare doublestar matches single", "**", "a", true, true},
		{"bare star matches via basename", "*", "a/b", true, true},

		// Empty pattern: valid, matches nothing.
		{"empty pattern non-empty path", "", "anything", false, true},
		{"empty pattern empty path", "", "", false, true},

		// Invalid patterns: ok=false.
		{"invalid unterminated class", "[", "file", false, false},
		{"invalid class in segment", "src/[/main.go", "src/x/main.go", false, false},
		{"invalid class slashless", "a[b", "x", false, false},

		// Case sensitivity (matching is case-sensitive).
		{"case sensitive ext", "*.MD", "readme.md", false, true},
		{"case sensitive literal", "README.md", "readme.md", false, true},
		{"case sensitive dir", "SRC/**", "src/a", false, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotMatched, gotOK := Match(tc.pattern, tc.path)
			if gotMatched != tc.wantMatched || gotOK != tc.wantOK {
				t.Errorf("Match(%q, %q) = (%v, %v), want (%v, %v)",
					tc.pattern, tc.path, gotMatched, gotOK, tc.wantMatched, tc.wantOK)
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	cases := []struct {
		name        string
		patterns    []string
		path        string
		wantPattern string
		wantMatched bool
	}{
		{
			name:        "first pattern wins",
			patterns:    []string{"*.md", "*.go"},
			path:        "main.go",
			wantPattern: "*.go",
			wantMatched: true,
		},
		{
			name:        "later production path",
			patterns:    []string{"lib/**", "src/**", "packages/**"},
			path:        "src/parser/tokenize.ts",
			wantPattern: "src/**",
			wantMatched: true,
		},
		{
			name:        "no match",
			patterns:    []string{"docs/**", "examples/**"},
			path:        "src/main.go",
			wantPattern: "",
			wantMatched: false,
		},
		{
			name:        "invalid patterns skipped",
			patterns:    []string{"[", "src/**"},
			path:        "src/main.go",
			wantPattern: "src/**",
			wantMatched: true,
		},
		{
			name:        "empty pattern list",
			patterns:    nil,
			path:        "src/main.go",
			wantPattern: "",
			wantMatched: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPattern, gotMatched := MatchAny(tc.patterns, tc.path)
			if gotPattern != tc.wantPattern || gotMatched != tc.wantMatched {
				t.Errorf("MatchAny(%v, %q) = (%q, %v), want (%q, %v)",
					tc.patterns, tc.path, gotPattern, gotMatched, tc.wantPattern, tc.wantMatched)
			}
		})
	}
}

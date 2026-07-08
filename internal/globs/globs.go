// Package globs implements the doublestar path-matching semantics used
// throughout CodeSteward. It is FROZEN: the behavior described here is a
// stable contract that other packages rely on. Matching is case-sensitive
// and uses only the standard library.
package globs

import (
	"path"
	"strings"
)

// Match reports whether path (slash-separated, repo-relative, no leading /)
// matches pattern with doublestar semantics:
//
//   - "*" matches any run of non-separator characters (may be empty).
//   - "?" matches exactly one non-separator character.
//   - "**" as a full path segment matches zero or more segments.
//   - "dir/**" matches everything under dir but NOT dir itself.
//   - A pattern with no slash (e.g. "*.md", "package.json") is matched against
//     the path's basename AND the whole path.
//   - A pattern ending in "/" matches the directory itself and everything
//     under it.
//
// The empty pattern is valid and matches nothing. Invalid patterns (for
// example an unterminated character class such as "[") return matched=false
// and ok=false.
func Match(pattern, path string) (matched, ok bool) {
	if !valid(pattern) {
		return false, false
	}
	if pattern == "" {
		return false, true
	}

	// Normalize a leading "./" that some callers may pass.
	path = strings.TrimPrefix(path, "./")

	// Trailing slash: the directory itself and everything under it.
	if strings.HasSuffix(pattern, "/") {
		dir := strings.TrimSuffix(pattern, "/")
		if dir == "" {
			// "/" matches every path.
			return true, true
		}
		if matchPattern(dir, path) {
			return true, true
		}
		return matchPattern(dir+"/**", path), true
	}

	// Slashless pattern: match against the whole path AND the basename, so
	// that e.g. "package.json" matches both "package.json" and
	// "frontend/package.json".
	if !strings.Contains(pattern, "/") {
		if matchPattern(pattern, path) {
			return true, true
		}
		base := path
		if i := strings.LastIndexByte(path, '/'); i >= 0 {
			base = path[i+1:]
		}
		return matchPattern(pattern, base), true
	}

	return matchPattern(pattern, path), true
}

// MatchAny returns the first pattern in patterns that matches path. Invalid
// patterns are skipped.
func MatchAny(patterns []string, path string) (pattern string, matched bool) {
	for _, p := range patterns {
		if m, ok := Match(p, path); ok && m {
			return p, true
		}
	}
	return "", false
}

// valid reports whether every segment of pattern is a well-formed matcher.
// The only source of invalidity in our syntax is a malformed character class
// (e.g. "["), which path.Match reports as ErrBadPattern.
func valid(pattern string) bool {
	for _, seg := range strings.Split(pattern, "/") {
		if seg == "**" {
			continue
		}
		if _, err := path.Match(seg, ""); err != nil {
			return false
		}
	}
	return true
}

// matchPattern matches an already-validated pattern against name using
// segment-aware doublestar semantics.
func matchPattern(pattern, name string) bool {
	m, err := matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
	if err != nil {
		return false
	}
	return m
}

// matchSegments matches pattern segments against name segments. A segment
// equal to "**" matches zero or more name segments, except a trailing "**"
// which matches one or more (so "dir/**" never matches "dir" itself).
func matchSegments(pat, name []string) (bool, error) {
	if len(pat) == 0 {
		return len(name) == 0, nil
	}
	if pat[0] == "**" {
		if len(pat) == 1 {
			// Trailing "**": require at least one remaining segment.
			return len(name) >= 1, nil
		}
		// "**" followed by more pattern: try consuming 0..len(name) segments.
		for i := 0; i <= len(name); i++ {
			m, err := matchSegments(pat[1:], name[i:])
			if err != nil {
				return false, err
			}
			if m {
				return true, nil
			}
		}
		return false, nil
	}
	if len(name) == 0 {
		return false, nil
	}
	m, err := path.Match(pat[0], name[0])
	if err != nil {
		return false, err
	}
	if !m {
		return false, nil
	}
	return matchSegments(pat[1:], name[1:])
}

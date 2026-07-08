package codeowners

import (
	"path"
	"sort"
	"strings"

	"github.com/codesteward-ai/codesteward/pkg/model"
)

// Match implements model.OwnerMatcher.
//
// GitHub semantics: the last matching rule wins. A matching rule with no
// owners is explicitly unowned (Found=false, but Pattern is set).
//
// GitLab semantics: within each section the last matching rule wins; the
// resulting owners are the sorted, deduplicated union across all
// (non-optional) sections. Optional "^[Section]" sections are excluded from
// matching per the v0 scoring contract.
//
// Pattern semantics are gitignore-style: a leading "/" anchors to the repo
// root; a pattern that contains a non-trailing slash is anchored; a slashless
// pattern matches a basename at any depth; a trailing "/" matches directory
// contents; "*", "?" and "**" behave as in internal/globs.
func (f *File) Match(p string) model.OwnershipMatch {
	np := normalizePath(p)
	if f.Dialect == DialectGitLab {
		return f.matchGitLab(np)
	}
	return f.matchGitHub(np)
}

// matchGitHub returns the last matching rule's ownership.
func (f *File) matchGitHub(np string) model.OwnershipMatch {
	var matched *Rule
	for i := range f.Rules {
		if matchPattern(f.Rules[i].Pattern, np) {
			matched = &f.Rules[i]
		}
	}
	if matched == nil {
		return model.OwnershipMatch{Found: false, Class: model.MatchMissing}
	}
	if len(matched.Owners) == 0 {
		return model.OwnershipMatch{Found: false, Pattern: matched.Pattern, Class: model.MatchMissing}
	}
	return model.OwnershipMatch{
		Found:   true,
		Owners:  dedupPreserve(matched.Owners),
		Pattern: matched.Pattern,
		Class:   ClassifyPattern(matched.Pattern),
	}
}

// matchGitLab returns the union of the last matching rule in each
// (non-optional) section.
func (f *File) matchGitLab(np string) model.OwnershipMatch {
	lastBySection := map[string]*Rule{}
	for i := range f.Rules {
		r := &f.Rules[i]
		key := strings.ToLower(r.Section)
		if f.optionalSections[key] {
			continue
		}
		if matchPattern(r.Pattern, np) {
			lastBySection[key] = r
		}
	}
	if len(lastBySection) == 0 {
		return model.OwnershipMatch{Found: false, Class: model.MatchMissing}
	}

	ownerSet := map[string]struct{}{}
	var repr *Rule
	for _, r := range lastBySection {
		for _, o := range r.Owners {
			ownerSet[o] = struct{}{}
		}
		repr = pickRepresentative(repr, r)
	}
	if len(ownerSet) == 0 {
		// Every matching section rule was explicitly unowned.
		return model.OwnershipMatch{Found: false, Pattern: repr.Pattern, Class: model.MatchMissing}
	}
	owners := make([]string, 0, len(ownerSet))
	for o := range ownerSet {
		owners = append(owners, o)
	}
	sort.Strings(owners)
	return model.OwnershipMatch{
		Found:   true,
		Owners:  owners,
		Pattern: repr.Pattern,
		Class:   ClassifyPattern(repr.Pattern),
	}
}

// pickRepresentative deterministically selects the rule that best represents a
// GitLab multi-section match: the most specific pattern wins, ties broken by
// the later line number.
func pickRepresentative(cur, next *Rule) *Rule {
	if cur == nil {
		return next
	}
	cr := classRank(ClassifyPattern(cur.Pattern))
	nr := classRank(ClassifyPattern(next.Pattern))
	if nr < cr {
		return next
	}
	if nr == cr && next.Line > cur.Line {
		return next
	}
	return cur
}

func classRank(c model.OwnershipMatchClass) int {
	switch c {
	case model.MatchSpecific:
		return 0
	case model.MatchBroad:
		return 1
	case model.MatchFallback:
		return 2
	default:
		return 3
	}
}

func dedupPreserve(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// normalizePath trims a leading "./" or "/" so paths are repo-relative.
func normalizePath(p string) string {
	p = strings.TrimPrefix(p, "./")
	p = strings.TrimPrefix(p, "/")
	return p
}

// matchPattern reports whether a gitignore-style CODEOWNERS pattern matches the
// repo-relative, slash-separated path p.
//
// This matcher is implemented in-package rather than reusing internal/globs
// because CODEOWNERS anchoring differs from globs.Match: a leading "/" or any
// non-trailing slash anchors the pattern to the repo root, and a slashless
// pattern matches a basename at any depth (globs.Match instead matches a
// slashless pattern against both the basename and the whole path). Single-
// segment glob metacharacters ("*", "?", character classes) are delegated to
// path.Match; "**" segment spanning is handled here.
func matchPattern(pattern, p string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}

	core := pattern
	hadLead := strings.HasPrefix(core, "/")
	if hadLead {
		core = core[1:]
	}
	dirOnly := strings.HasSuffix(core, "/")
	core = strings.TrimSuffix(core, "/")
	if core == "" {
		// "/" (or "" after trimming): matches everything.
		return true
	}

	pathSegs := strings.Split(p, "/")
	anchored := hadLead || strings.Contains(core, "/")

	if !anchored {
		// Slashless pattern: match a single segment at any depth.
		seg := core
		if dirOnly {
			// Directory pattern: the matching segment must be an ancestor
			// (i.e. not the final segment) so it truly has contents.
			for i := 0; i < len(pathSegs)-1; i++ {
				if segMatch(seg, pathSegs[i]) {
					return true
				}
			}
			return false
		}
		for i := 0; i < len(pathSegs); i++ {
			if segMatch(seg, pathSegs[i]) {
				return true
			}
		}
		return false
	}

	// Anchored pattern: match from the repo root.
	patSegs := strings.Split(core, "/")
	if dirOnly {
		// Directory contents only: pattern must match a proper prefix.
		return matchSegs(appendSeg(patSegs, "**"), pathSegs)
	}
	// A non-directory anchored pattern matches the path exactly, or — when its
	// final segment is a concrete name (no glob metacharacters) — also owns
	// that name's directory subtree (e.g. "/apps/github" owns
	// "apps/github/x.ts"). A wildcard-terminated final segment ("docs/*",
	// "/src/*") must match only a single level, per the frozen internal/globs
	// contract (§6.4) and GitHub/GitLab CODEOWNERS semantics, so the "**"
	// directory-recursion segment is appended only for concrete final segments.
	if matchSegs(patSegs, pathSegs) {
		return true
	}
	if isConcreteSeg(patSegs[len(patSegs)-1]) {
		return matchSegs(appendSeg(patSegs, "**"), pathSegs)
	}
	return false
}

// isConcreteSeg reports whether a pattern segment is a concrete literal name,
// i.e. contains no glob metacharacters ("*", "?" or a "[" character class) and
// so matches exactly one directory or file name.
func isConcreteSeg(seg string) bool {
	return !strings.ContainsAny(seg, "*?[")
}

// appendSeg returns a new slice equal to segs with extra appended, without
// mutating segs' backing array.
func appendSeg(segs []string, extra string) []string {
	out := make([]string, len(segs)+1)
	copy(out, segs)
	out[len(segs)] = extra
	return out
}

// segMatch matches a single pattern segment against a single name segment.
// "**" as a slashless pattern matches any segment.
func segMatch(pat, name string) bool {
	if pat == "**" {
		return true
	}
	m, err := path.Match(pat, name)
	return err == nil && m
}

// matchSegs matches anchored pattern segments against name segments with
// doublestar semantics. A "**" segment matches zero or more name segments,
// except a trailing "**" which requires at least one remaining segment (so
// "dir/**" never matches "dir" itself).
func matchSegs(pat, name []string) bool {
	if len(pat) == 0 {
		return len(name) == 0
	}
	if pat[0] == "**" {
		if len(pat) == 1 {
			return len(name) >= 1
		}
		for i := 0; i <= len(name); i++ {
			if matchSegs(pat[1:], name[i:]) {
				return true
			}
		}
		return false
	}
	if len(name) == 0 {
		return false
	}
	if !segMatch(pat[0], name[0]) {
		return false
	}
	return matchSegs(pat[1:], name[1:])
}

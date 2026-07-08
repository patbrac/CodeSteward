// Package model defines the shared, stable data model for CodeSteward
// reports. It has no dependencies outside the standard library.
package model

type ReadinessStatus string

const (
	StatusReady           ReadinessStatus = "ready_for_maintainer_review"
	StatusReviewableNotes ReadinessStatus = "reviewable_with_notes"
	StatusNeedsAction     ReadinessStatus = "needs_contributor_action"
	StatusHighBurden      ReadinessStatus = "high_review_burden"
	StatusNeedsOwnerRoute ReadinessStatus = "needs_owner_routing"
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
	TestsNotRequired        TestState = "not_required"
	TestsMatchingChanged    TestState = "matching_test_changed"
	TestsExistingNotChanged TestState = "existing_test_found_but_not_changed"
	TestsMissingMatching    TestState = "missing_matching_test"
	TestsNotEvaluated       TestState = "not_evaluated"
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
	Provided  bool `json:"provided"`
	Length    int  `json:"length"`
	Evaluated bool `json:"evaluated"`
}

// Finding is one deterministic observation about the change.
type Finding struct {
	RuleID   string   `json:"rule_id"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`          // past-tense fact, for the details section
	Action   string   `json:"action,omitempty"` // imperative contributor action item
	Paths    []string `json:"paths,omitempty"`  // sorted
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
	SchemaVersion string             `json:"schema_version"` // "0.1"
	Tool          ToolInfo           `json:"tool"`
	Base          string             `json:"base"`
	Head          string             `json:"head"`
	CommentOnly   bool               `json:"comment_only"`
	Status        ReadinessStatus    `json:"status"`
	Score         int                `json:"score"` // 0..100, internal; hidden in markdown
	ReviewBurden  ReviewBurden       `json:"review_burden"`
	Ownership     OwnershipSummary   `json:"ownership"`
	Tests         TestsSummary       `json:"tests"`
	Scope         ScopeSummary       `json:"scope"`
	Description   DescriptionSummary `json:"description"`
	Findings      []Finding          `json:"findings"`
	ExitCriteria  []ExitCriterion    `json:"exit_criteria"`
	Warnings      []string           `json:"warnings,omitempty"` // non-fatal scan warnings
}

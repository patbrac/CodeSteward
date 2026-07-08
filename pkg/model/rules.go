package model

// Rule ID constants. These IDs are contractual and appear in report output.
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

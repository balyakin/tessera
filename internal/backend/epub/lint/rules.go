package lint

const (
	RuleFontSize      = "t-lint-001"
	RuleOPFRequired   = "t-lint-002"
	RuleModified      = "t-lint-003"
	RuleNoteref       = "t-lint-004"
	RuleXHTMLXML      = "t-lint-005"
	RuleMediaType     = "t-lint-006"
	RuleImageAlt      = "t-lint-007"
	RulePossessive    = "t-lint-008"
	RuleHeadingType   = "t-lint-009"
	RuleStraightQuote = "t-lint-010"
	RuleXMLLang       = "t-lint-011"
	RuleWordCount     = "t-lint-012"
)

type Finding struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

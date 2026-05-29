package tessera

import "github.com/balyakin/tessera/pkg/tessera/ir"

type Issue = ir.Issue

type InputFormat string

const (
	FormatODT  InputFormat = "odt"
	FormatDOCX InputFormat = "docx"
)

type OutputKind string

const (
	OutputPDF  OutputKind = "pdf"
	OutputEPUB OutputKind = "epub"
	OutputTEX  OutputKind = "tex"
)

type BuildOptions struct {
	InputPath    string
	OutputDir    string
	ConfigPath   string
	Metadata     map[string]string
	Targets      []OutputKind
	Engine       string
	KeepTex      bool
	Lint         bool
	Reproducible bool
	StrictStyles bool
	DumpIRPath   string
	OnProgress   func(phase string, detail string, percent float64)
}

type Artifact struct {
	Kind OutputKind `json:"kind"`
	Path string     `json:"path"`
}

type BuildResult struct {
	InputPath     string        `json:"input_path"`
	InputFormat   InputFormat   `json:"input_format"`
	Artifacts     []Artifact    `json:"artifacts"`
	Issues        []Issue       `json:"issues"`
	LintFindings  []LintFinding `json:"lint_findings"`
	Stats         DocumentStats `json:"stats"`
	ElapsedMillis int64         `json:"elapsed_ms"`
}

type DocumentStats struct {
	Words                  int `json:"words"`
	Characters             int `json:"characters"`
	Chapters               int `json:"chapters"`
	Footnotes              int `json:"footnotes"`
	Images                 int `json:"images"`
	Tables                 int `json:"tables"`
	ParagraphStylesTotal   int `json:"paragraph_styles_total"`
	ParagraphStylesMapped  int `json:"paragraph_styles_mapped"`
	ParagraphStylesUnknown int `json:"paragraph_styles_unknown"`
	CharacterStylesTotal   int `json:"character_styles_total"`
	CharacterStylesMapped  int `json:"character_styles_mapped"`
	CharacterStylesUnknown int `json:"character_styles_unknown"`
}

type InspectResult struct {
	InputPath        string                 `json:"input_path"`
	InputFormat      InputFormat            `json:"input_format"`
	Metadata         ir.Metadata            `json:"metadata"`
	Stats            DocumentStats          `json:"stats"`
	ParagraphStyles  []InspectStyle         `json:"paragraph_styles"`
	CharacterStyles  []InspectStyle         `json:"character_styles"`
	DirectFormatting []DirectFormattingStat `json:"direct_formatting"`
	Issues           []Issue                `json:"issues"`
}

type InspectStyle struct {
	Name          string `json:"name"`
	Family        string `json:"family"`
	Status        string `json:"status"`
	Role          string `json:"role"`
	MatchedName   string `json:"matched_name"`
	MatchKind     string `json:"match_kind"`
	SuggestedTOML string `json:"suggested_toml"`
}

type DirectFormattingStat struct {
	Role  string `json:"role"`
	Count int    `json:"count"`
}

type LintFinding struct {
	RuleID   string `json:"rule_id"`
	Severity string `json:"severity"`
	File     string `json:"file"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

type Options struct {
	ConfigPath   string
	Metadata     map[string]string
	Reproducible bool
	StrictStyles bool
}

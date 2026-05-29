package parser

import (
	"io"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type Parser interface {
	Parse(reader io.ReaderAt, size int64, cfg *config.Config, opts ParseOptions) (*ParseResult, error)
}

type ParseOptions struct {
	InputPath    string
	Metadata     map[string]string
	StrictStyles bool
}

type ParseResult struct {
	Document    *ir.Document
	Issues      []ir.Issue
	StyleReport StyleReport
}

type StyleReport struct {
	ParagraphStyles  []StyleUsage
	CharacterStyles  []StyleUsage
	DirectFormatting []DirectFormattingStat
}

type StyleUsage struct {
	Name          string
	Family        string
	Status        string
	Role          string
	MatchedName   string
	MatchKind     string
	SuggestedTOML string
}

type DirectFormattingStat struct {
	Role  string
	Count int
}

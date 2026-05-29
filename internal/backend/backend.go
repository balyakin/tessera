package backend

import (
	"time"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type OutputKind string

const (
	OutputPDF  OutputKind = "pdf"
	OutputEPUB OutputKind = "epub"
	OutputTEX  OutputKind = "tex"
)

type Artifact struct {
	Kind OutputKind
	Path string
}

type RenderOptions struct {
	OutputDir     string
	Reproducible  bool
	SourceDateUTC time.Time
	Basename      string
}

type Backend interface {
	Render(doc *ir.Document, cfg *config.Config, opts RenderOptions) ([]Artifact, []ir.Issue, error)
}

type RenderedImage struct {
	Name string
	Data []byte
}

type LatexResult struct {
	TexSource string
	Images    []RenderedImage
	Issues    []ir.Issue
}

type EPUBResult struct {
	Bytes  []byte
	Issues []ir.Issue
}

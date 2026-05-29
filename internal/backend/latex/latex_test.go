package latex

import (
	"os"
	"strings"
	"testing"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func TestRenderSemanticLaTeX(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Thought, Children: []ir.Inline{ir.Text{Value: "inner"}}}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Foreign, Lang: "la", Children: []ir.Inline{ir.Text{Value: "veritas"}}}}},
		},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.TexSource, `\semThought{inner}`) || !strings.Contains(result.TexSource, `\textlatin{veritas}`) {
		t.Fatalf("semantic LaTeX missing:\n%s", result.TexSource)
	}
	if !strings.Contains(result.TexSource, `\usepackage{unicode-math}`) || !strings.Contains(result.TexSource, `\setotherlanguage{latin}`) {
		t.Fatalf("language/package setup missing:\n%s", result.TexSource)
	}
}

func TestRenderLaTeXRepeatedFootnoteAndEpigraphSource(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Epigraph{Children: []ir.Block{
				ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "First"}}},
				ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Second"}}},
			}, Source: []ir.Inline{ir.Text{Value: "Source"}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.FootnoteRef{ID: "fn-1"}, ir.Text{Value: " again "}, ir.FootnoteRef{ID: "fn-1"}}},
		},
		Footnotes: []ir.FootnoteDef{{ID: "fn-1", Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Strong, Children: []ir.Inline{ir.Text{Value: "Note"}}}}}}}},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.TexSource, `\attrib{Source}`) {
		t.Fatalf("epigraph source attribution missing:\n%s", result.TexSource)
	}
	if !strings.Contains(result.TexSource, `\epigraph{First\par{} Second}{\attrib{Source}}`) {
		t.Fatalf("epigraph paragraphs should be safe in a LaTeX argument:\n%s", result.TexSource)
	}
	if !strings.Contains(result.TexSource, `\footnote{\semStrong{Note}}`) {
		t.Fatalf("footnote inline semantics not preserved:\n%s", result.TexSource)
	}
	found := false
	for _, issue := range result.Issues {
		if issue.Code == "t-warn-footnote-repeated" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected repeated footnote warning, got %#v", result.Issues)
	}
}

func TestRenderLaTeXDeduplicatesImagesByHash(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	image := ir.Image{Name: "a.png", Data: []byte{137, 80, 78, 71}, MediaType: "image/png", Alt: "image"}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Figure{Image: image},
			ir.Figure{Image: ir.Image{Name: "b.png", Data: []byte{137, 80, 78, 71}, MediaType: "image/png", Alt: "image"}},
		},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Images) != 1 {
		t.Fatalf("expected duplicate image data to be written once, got %#v", result.Images)
	}
}

func TestRenderLaTeXTablePreservesInlineSemantics(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Table{Rows: []ir.TableRow{{Cells: []ir.TableCell{{Children: []ir.Block{
				ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{
					ir.Styled{Role: ir.Strong, Children: []ir.Inline{ir.Text{Value: "Head"}}},
					ir.Text{Value: " and "},
					ir.Styled{Role: ir.Foreign, Lang: "la", Children: []ir.Inline{ir.Text{Value: "veritas"}}},
				}},
			}}}}}},
		},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.TexSource, `\semStrong{Head} and \textlatin{veritas}`) {
		t.Fatalf("table cell semantics not preserved:\n%s", result.TexSource)
	}
}

func TestFileBackendWritesTexArtifact(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body:      []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Body"}}}},
	}
	dir := t.TempDir()
	artifacts, _, err := FileBackend{Kind: backend.OutputTEX}.Render(doc, cfg, backend.RenderOptions{OutputDir: dir, Basename: "book"})
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", artifacts)
	}
	data, err := os.ReadFile(artifacts[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `\begin{document}`) {
		t.Fatalf("tex artifact was not written correctly:\n%s", data)
	}
}

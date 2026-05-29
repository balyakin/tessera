package epub

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func TestRenderEPUBArchive(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en", Identifier: "urn:test"},
		Body: []ir.Block{
			ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Chapter"}}},
			ir.Verse{Stanzas: [][]ir.Line{{{Inlines: []ir.Inline{ir.Text{Value: "A line"}}}}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Thought, Children: []ir.Inline{ir.Text{Value: "inner"}}}}},
		},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(result.Bytes), int64(len(result.Bytes)))
	if err != nil {
		t.Fatal(err)
	}
	if zr.File[0].Name != "mimetype" || zr.File[0].Method != zip.Store {
		t.Fatalf("bad mimetype entry")
	}
}

func TestRenderEPUBTOCDepthZeroMatterAndCustomCSS(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	cfg.TOC.Depth = 0
	dir := t.TempDir()
	cfg.EPUB.CustomCSS = filepath.Join(dir, "custom.css")
	if err := os.WriteFile(cfg.EPUB.CustomCSS, []byte("p { margin: 1em; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "ru", Identifier: "urn:test"},
		Cover:     &ir.Image{Name: "cover.png", Data: []byte{137, 80, 78, 71, 13, 10, 26, 10}, MediaType: "image/png", Alt: "cover"},
		Body: []ir.Block{
			ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Chapter"}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Body"}, ir.FootnoteRef{ID: "fn-1"}}},
		},
		Footnotes: []ir.FootnoteDef{{ID: "fn-1", Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Note"}}}}}},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	files := unzipEPUB(t, result.Bytes)
	nav := string(files["epub/toc.xhtml"])
	first := string(files["epub/text/0001.xhtml"])
	if strings.Contains(nav, "<li>") {
		t.Fatalf("toc.depth=0 should suppress visible nav entries: %s", nav)
	}
	if !strings.Contains(nav, "Содержание") || !strings.Contains(nav, `href="css/custom.css"`) {
		t.Fatalf("nav missing localized title or custom CSS: %s", nav)
	}
	if !strings.Contains(first, `body epub:type="bodymatter"`) || !strings.Contains(first, `section epub:type="chapter"`) {
		t.Fatalf("first heading section should be bodymatter chapter: %s", first)
	}
	if !strings.Contains(first, `href="../css/custom.css"`) ||
		!strings.Contains(string(files["epub/text/cover.xhtml"]), `href="../css/custom.css"`) ||
		!strings.Contains(string(files["epub/text/endnotes.xhtml"]), `href="../css/custom.css"`) {
		t.Fatalf("custom CSS was not linked from all XHTML documents")
	}
}

func TestRenderEPUBStableHeadingIDsBacklinksAndTables(t *testing.T) {
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en", Identifier: "urn:test"},
		Body: []ir.Block{
			ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Repeat"}}},
			ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Repeat"}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "See note"}, ir.FootnoteRef{ID: "fn-1"}}},
			ir.Table{Rows: []ir.TableRow{
				{Header: true, Cells: []ir.TableCell{{Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Strong, Children: []ir.Inline{ir.Text{Value: "Head"}}}}}}}}},
				{Cells: []ir.TableCell{{Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Styled{Role: ir.Emphasis, Children: []ir.Inline{ir.Text{Value: "Cell"}}}}}}}}},
			}},
		},
		Footnotes: []ir.FootnoteDef{{ID: "fn-1", Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Footnote"}}}}}},
	}
	result, err := Render(doc, cfg, backend.RenderOptions{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	files := unzipEPUB(t, result.Bytes)
	nav := string(files["epub/toc.xhtml"])
	second := string(files["epub/text/0002.xhtml"])
	endnotes := string(files["epub/text/endnotes.xhtml"])
	if !strings.Contains(nav, `0002.xhtml#h-repeat-2`) || !strings.Contains(second, `id="h-repeat-2"`) {
		t.Fatalf("nav/content heading IDs diverged\nnav=%s\nsecond=%s", nav, second)
	}
	if !strings.Contains(endnotes, `href="0002.xhtml#ref-fn-1-1"`) {
		t.Fatalf("endnote backlink does not point to actual section: %s", endnotes)
	}
	if !strings.Contains(second, "<thead>") || !strings.Contains(second, "<strong>Head</strong>") || !strings.Contains(second, "<em>Cell</em>") {
		t.Fatalf("table semantics not preserved: %s", second)
	}
}

func unzipEPUB(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{}
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[file.Name] = content
	}
	return files
}

package odt

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func TestParseMinimalODT(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZip(t, zw, "content.xml", `<?xml version="1.0"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0">
<office:body><office:text>
<text:h text:outline-level="1" text:style-name="Heading 1">Chapter</text:h>
<text:p text:style-name="Poem">A line<text:line-break/>Another line</text:p>
</office:text></office:body></office:document-content>`)
	writeZip(t, zw, "styles.xml", `<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0">
<office:styles>
<style:style style:name="Heading 1" style:family="paragraph"/>
<style:style style:name="Poem" style:family="paragraph"/>
</office:styles></office:document-styles>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	result, err := (Parser{}).Parse(bytes.NewReader(buf.Bytes()), int64(buf.Len()), cfg, parser.ParseOptions{InputPath: "book.odt"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Meta.Title != "book" || len(result.Document.Body) != 2 {
		t.Fatalf("unexpected parse result: %#v", result.Document)
	}
}

func TestParseODTFigureCaptionStyle(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZip(t, zw, "content.xml", `<?xml version="1.0"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink">
<office:body><office:text>
<text:p><draw:frame draw:name="Alt"><draw:image xlink:href="Pictures/pic.png"/></draw:frame></text:p>
<text:p text:style-name="Caption">A caption</text:p>
</office:text></office:body></office:document-content>`)
	writeZip(t, zw, "styles.xml", `<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0">
<office:styles><style:style style:name="Caption" style:family="paragraph"/></office:styles></office:document-styles>`)
	writeZip(t, zw, "Pictures/pic.png", string([]byte{137, 80, 78, 71, 13, 10, 26, 10}))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	result, err := (Parser{}).Parse(bytes.NewReader(buf.Bytes()), int64(buf.Len()), cfg, parser.ParseOptions{InputPath: "book.odt"})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Document.Body) != 1 {
		t.Fatalf("expected one figure block, got %#v", result.Document.Body)
	}
	fig, ok := result.Document.Body[0].(ir.Figure)
	if !ok || len(fig.Caption) != 1 {
		t.Fatalf("caption was not attached to figure: %#v", result.Document.Body[0])
	}
}

func writeZip(t *testing.T, zw *zip.Writer, name, data string) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(data)); err != nil {
		t.Fatal(err)
	}
}

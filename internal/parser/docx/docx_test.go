package docx

import (
	"archive/zip"
	"bytes"
	"testing"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
)

func TestParseMinimalDOCX(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZip(t, zw, "word/document.xml", `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Chapter</w:t></w:r></w:p>
<w:p><w:pPr><w:pStyle w:val="Poem"/></w:pPr><w:r><w:t>A line</w:t></w:r><w:r><w:br/></w:r><w:r><w:t>Another line</w:t></w:r></w:p>
</w:body></w:document>`)
	writeZip(t, zw, "word/styles.xml", `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="Heading 1"/></w:style>
<w:style w:type="paragraph" w:styleId="Poem"><w:name w:val="Poem"/></w:style>
</w:styles>`)
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	result, err := (Parser{}).Parse(bytes.NewReader(buf.Bytes()), int64(buf.Len()), cfg, parser.ParseOptions{InputPath: "book.docx"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Document.Meta.Title != "book" || len(result.Document.Body) != 2 {
		t.Fatalf("unexpected parse result: %#v", result.Document)
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

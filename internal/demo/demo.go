package demo

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	ODTName  = "semantic-demo.odt"
	DOCXName = "semantic-demo.docx"
)

func WriteDemoFiles(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	odt, err := ODT()
	if err != nil {
		return err
	}
	docx, err := DOCX()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, ODTName), odt, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, DOCXName), docx, 0o644)
}

func ODT() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct {
		name string
		data string
	}{
		{"mimetype", "application/vnd.oasis.opendocument.text"},
		{"content.xml", odtContent},
		{"styles.xml", odtStyles},
		{"meta.xml", odtMeta},
	}
	for _, file := range files {
		if err := writeEntry(zw, file.name, []byte(file.data), file.name == "mimetype"); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func DOCX() ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := []struct {
		name string
		data string
	}{
		{"[Content_Types].xml", docxContentTypes},
		{"word/document.xml", docxDocument},
		{"word/styles.xml", docxStyles},
		{"word/_rels/document.xml.rels", docxRels},
		{"word/footnotes.xml", docxFootnotes},
		{"docProps/core.xml", docxCore},
	}
	for _, file := range files {
		if err := writeEntry(zw, file.name, []byte(file.data), false); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeEntry(zw *zip.Writer, name string, data []byte, store bool) error {
	header := &zip.FileHeader{Name: name, Method: zip.Deflate}
	if store {
		header.Method = zip.Store
	}
	header.SetModTime(time.Unix(0, 0).UTC())
	w, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create %s: %w", name, err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	return nil
}

const odtContent = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-content xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:text="urn:oasis:names:tc:opendocument:xmlns:text:1.0" xmlns:draw="urn:oasis:names:tc:opendocument:xmlns:drawing:1.0" xmlns:xlink="http://www.w3.org/1999/xlink" office:version="1.3">
  <office:body>
    <office:text>
      <text:h text:style-name="Heading 1" text:outline-level="1">The Semantic Chapter</text:h>
      <text:p text:style-name="Text Body">This paragraph keeps <text:span text:style-name="Direct Thought">a private thought</text:span> and <text:span text:style-name="Foreign - Latin">veritas</text:span> as named character styles.</text:p>
      <text:p text:style-name="Poem">First semantic line<text:line-break/>Second semantic line</text:p>
      <text:p text:style-name="Letter">Dear reader, the letter role is preserved.</text:p>
      <text:p text:style-name="Epigraph">Words before the story should remain an epigraph.</text:p>
      <text:p text:style-name="Quote">A quoted block keeps its structure.</text:p>
      <text:list>
        <text:list-item><text:p text:style-name="Text Body">One item</text:p></text:list-item>
        <text:list-item><text:p text:style-name="Text Body">Another item</text:p></text:list-item>
      </text:list>
      <text:p text:style-name="Text Body">A note appears here<text:note text:note-class="footnote" text:id="1"><text:note-citation>1</text:note-citation><text:note-body><text:p text:style-name="Text Body">Synthetic footnote text.</text:p></text:note-body></text:note>.</text:p>
    </office:text>
  </office:body>
</office:document-content>`

const odtStyles = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-styles xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:style="urn:oasis:names:tc:opendocument:xmlns:style:1.0" xmlns:fo="urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0">
  <office:styles>
    <style:style style:name="Heading 1" style:family="paragraph"/>
    <style:style style:name="Text Body" style:family="paragraph"/>
    <style:style style:name="Poem" style:family="paragraph"/>
    <style:style style:name="Letter" style:family="paragraph"/>
    <style:style style:name="Epigraph" style:family="paragraph"/>
    <style:style style:name="Quote" style:family="paragraph"/>
    <style:style style:name="Direct Thought" style:family="text"/>
    <style:style style:name="Foreign - Latin" style:family="text"/>
  </office:styles>
</office:document-styles>`

const odtMeta = `<?xml version="1.0" encoding="UTF-8"?>
<office:document-meta xmlns:office="urn:oasis:names:tc:opendocument:xmlns:office:1.0" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <office:meta>
    <dc:title>Tessera Semantic Demo</dc:title>
    <dc:creator>Evgeny Balyakin</dc:creator>
    <dc:language>en</dc:language>
    <dc:description>Synthetic Tessera demo manuscript.</dc:description>
  </office:meta>
</office:document-meta>`

const docxContentTypes = `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`

const docxDocument = `<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>The Semantic Chapter</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="TextBody"/></w:pPr><w:r><w:t>This paragraph keeps </w:t></w:r><w:r><w:rPr><w:rStyle w:val="DirectThought"/></w:rPr><w:t>a private thought</w:t></w:r><w:r><w:t> and </w:t></w:r><w:r><w:rPr><w:rStyle w:val="ForeignLatin"/></w:rPr><w:t>veritas</w:t></w:r><w:r><w:t> as named character styles.</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="Poem"/></w:pPr><w:r><w:t>First semantic line</w:t></w:r><w:r><w:br/></w:r><w:r><w:t>Second semantic line</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="Letter"/></w:pPr><w:r><w:t>Dear reader, the letter role is preserved.</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="Epigraph"/></w:pPr><w:r><w:t>Words before the story should remain an epigraph.</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="Quote"/></w:pPr><w:r><w:t>A quoted block keeps its structure.</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="TextBody"/></w:pPr><w:r><w:t>A note appears here</w:t></w:r><w:r><w:footnoteReference w:id="1"/></w:r><w:r><w:t>.</w:t></w:r></w:p>
  </w:body>
</w:document>`

const docxStyles = `<?xml version="1.0" encoding="UTF-8"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Heading1"><w:name w:val="Heading 1"/></w:style>
  <w:style w:type="paragraph" w:styleId="TextBody"><w:name w:val="Text Body"/></w:style>
  <w:style w:type="paragraph" w:styleId="Poem"><w:name w:val="Poem"/></w:style>
  <w:style w:type="paragraph" w:styleId="Letter"><w:name w:val="Letter"/></w:style>
  <w:style w:type="paragraph" w:styleId="Epigraph"><w:name w:val="Epigraph"/></w:style>
  <w:style w:type="paragraph" w:styleId="Quote"><w:name w:val="Quote"/></w:style>
  <w:style w:type="character" w:styleId="DirectThought"><w:name w:val="Direct Thought"/></w:style>
  <w:style w:type="character" w:styleId="ForeignLatin"><w:name w:val="Foreign - Latin"/></w:style>
</w:styles>`

const docxRels = `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`

const docxFootnotes = `<?xml version="1.0" encoding="UTF-8"?>
<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:footnote w:id="1"><w:p><w:pPr><w:pStyle w:val="TextBody"/></w:pPr><w:r><w:t>Synthetic footnote text.</w:t></w:r></w:p></w:footnote>
</w:footnotes>`

const docxCore = `<?xml version="1.0" encoding="UTF-8"?>
<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
  <dc:title>Tessera Semantic Demo</dc:title>
  <dc:creator>Evgeny Balyakin</dc:creator>
  <dc:language>en</dc:language>
  <dc:description>Synthetic Tessera demo manuscript.</dc:description>
</cp:coreProperties>`

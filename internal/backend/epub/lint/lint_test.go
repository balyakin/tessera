package lint

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
	"time"
)

func TestLintFindsBadFontSize(t *testing.T) {
	epub := testEPUB(t, map[string]string{
		"epub/css/core.css":    "p { font-size: 12px; }",
		"epub/content.opf":     `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>A</dc:title><dc:language>en</dc:language><dc:identifier>id</dc:identifier><meta property="dcterms:modified">1970-01-01T00:00:00Z</meta></metadata><manifest></manifest></package>`,
		"epub/text/0001.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en"><body><p>A</p></body></html>`,
	})
	findings := Lint(epub)
	if len(findings) == 0 || findings[0].RuleID != RuleFontSize {
		t.Fatalf("expected font-size finding, got %#v", findings)
	}
}

func TestFixAppliesSupportedAutofixes(t *testing.T) {
	epub := testEPUB(t, map[string]string{
		"epub/css/core.css":    "p { font-size: 16px; }",
		"epub/content.opf":     `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>A</dc:title><dc:language>en</dc:language><dc:identifier>id</dc:identifier><meta property="dcterms:modified">bad</meta><meta property="schema:wordCount">999</meta></metadata><manifest></manifest></package>`,
		"epub/text/0001.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml" xml:lang="en"><body><p>"Quoted" <i epub:type="se:name.publication">Book's</i> words</p></body></html>`,
	})
	fixed, findings, modified, err := Fix(epub, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if !modified {
		t.Fatalf("expected EPUB to be modified")
	}
	for _, finding := range findings {
		if finding.RuleID == RuleFontSize || finding.RuleID == RuleModified || finding.RuleID == RulePossessive || finding.RuleID == RuleStraightQuote || finding.RuleID == RuleWordCount {
			t.Fatalf("expected fixable findings to be resolved, got %#v", findings)
		}
	}
	files := unzipTestEPUB(t, fixed)
	css := string(files["epub/css/core.css"])
	opf := string(files["epub/content.opf"])
	xhtml := string(files["epub/text/0001.xhtml"])
	if css != "p { font-size: 1em; }" {
		t.Fatalf("bad CSS fix: %s", css)
	}
	if !bytes.Contains([]byte(opf), []byte(`<meta property="dcterms:modified">1970-01-01T00:00:00Z</meta>`)) ||
		!bytes.Contains([]byte(opf), []byte(`<meta property="schema:wordCount">4</meta>`)) {
		t.Fatalf("bad OPF fix: %s", opf)
	}
	if !bytes.Contains([]byte(xhtml), []byte(`&#8220;Quoted&#8221; <i epub:type="se:name.publication">Book</i>'s words`)) {
		t.Fatalf("bad XHTML fix: %s", xhtml)
	}
}

func TestLintPossessiveUsesXMLTextOnly(t *testing.T) {
	epub := testEPUB(t, map[string]string{
		"epub/css/core.css":    "p { font-size: 1em; }",
		"epub/content.opf":     `<package><metadata xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>A</dc:title><dc:language>en</dc:language><dc:identifier>id</dc:identifier><meta property="dcterms:modified">1970-01-01T00:00:00Z</meta></metadata><manifest></manifest></package>`,
		"epub/text/0001.xhtml": `<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="en"><body><p><!-- <i epub:type="se:name.publication">Book's</i> --><i epub:type="se:name.publication">Real's</i></p></body></html>`,
	})
	findings := Lint(epub)
	count := 0
	for _, finding := range findings {
		if finding.RuleID == RulePossessive {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected one XML-text possessive finding, got %#v", findings)
	}
}

func testEPUB(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "mimetype", Method: zip.Store}
	h.SetModTime(time.Unix(0, 0))
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("application/epub+zip"))
	for name, data := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func unzipTestEPUB(t *testing.T, data []byte) map[string][]byte {
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

package tessera

import (
	"testing"

	"github.com/balyakin/tessera/internal/demo"
)

func TestPublicAPIParseAndRender(t *testing.T) {
	dir := t.TempDir()
	if err := demo.WriteDemoFiles(dir); err != nil {
		t.Fatal(err)
	}
	doc, issues, err := ParseFile(dir+"/"+demo.DOCXName, Options{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Meta.Title == "" || len(issues) != 0 {
		t.Fatalf("unexpected parse result title=%q issues=%#v", doc.Meta.Title, issues)
	}
	epubBytes, _, err := RenderEPUB(doc, Options{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(epubBytes) == 0 {
		t.Fatalf("empty EPUB")
	}
	tex, _, err := RenderLaTeX(doc, Options{Reproducible: true})
	if err != nil {
		t.Fatal(err)
	}
	if tex == "" {
		t.Fatalf("empty TeX")
	}
}

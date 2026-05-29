package epub

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

//go:embed css/core.css
var cssFS embed.FS

type FileBackend struct{}

func (FileBackend) Render(doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) ([]backend.Artifact, []ir.Issue, error) {
	result, err := Render(doc, cfg, opts)
	if err != nil {
		return nil, nil, err
	}
	name := opts.Basename
	if name == "" {
		name = "book"
	}
	path := filepath.Join(opts.OutputDir, name+".epub")
	if err := os.WriteFile(path, result.Bytes, 0o644); err != nil {
		return nil, nil, fmt.Errorf("write EPUB: %w", err)
	}
	return []backend.Artifact{{Kind: backend.OutputEPUB, Path: path}}, result.Issues, nil
}

type imageResource struct {
	Name      string
	Data      []byte
	MediaType string
}

type asset struct {
	ID        string
	Name      string
	Data      []byte
	MediaType string
}

type renderer struct {
	cfg           *config.Config
	imageRefs     map[string]string
	images        []imageResource
	noteRefs      map[string][]noteRef
	currentFile   string
	headingCounts map[string]int
	customCSS     bool
	issues        []ir.Issue
}

func Render(doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) (backend.EPUBResult, error) {
	if doc == nil {
		return backend.EPUBResult{}, fmt.Errorf("render EPUB: document is nil")
	}
	modified, err := sourceDate(opts)
	if err != nil {
		return backend.EPUBResult{}, err
	}
	coreCSS, err := cssFS.ReadFile("css/core.css")
	if err != nil {
		return backend.EPUBResult{}, fmt.Errorf("read core css: %w", err)
	}
	var customCSS []byte
	if cfg.EPUB.CustomCSS != "" {
		customCSS, err = os.ReadFile(cfg.EPUB.CustomCSS)
		if err != nil {
			return backend.EPUBResult{}, fmt.Errorf("read custom CSS: %w", err)
		}
	}
	fonts, err := readFonts(cfg.EPUB.AdditionalFonts)
	if err != nil {
		return backend.EPUBResult{}, err
	}
	r := &renderer{
		cfg:           cfg,
		imageRefs:     map[string]string{},
		noteRefs:      map[string][]noteRef{},
		headingCounts: map[string]int{},
		customCSS:     len(customCSS) > 0,
	}
	if doc.Cover != nil {
		r.imageName(*doc.Cover)
	}
	sections := splitSections(doc, cfg.TOC.Depth)
	var files []zipFile
	files = append(files,
		zipFile{Name: "mimetype", Data: []byte("application/epub+zip"), Store: true},
		zipFile{Name: "META-INF/container.xml", Data: []byte(containerXML)},
	)
	var xhtmlFiles []zipFile
	if doc.Cover != nil {
		xhtmlFiles = append(xhtmlFiles, zipFile{Name: "epub/text/cover.xhtml", Data: []byte(renderCoverXHTML(doc, r))})
	}
	for _, sec := range sections {
		xhtmlFiles = append(xhtmlFiles, zipFile{Name: "epub/text/" + sec.Name, Data: []byte(renderSectionXHTML(doc, sec, r))})
	}
	if len(doc.Footnotes) > 0 {
		xhtmlFiles = append(xhtmlFiles, zipFile{Name: "epub/text/endnotes.xhtml", Data: []byte(renderEndnotes(doc, r))})
	}
	files = append(files,
		zipFile{Name: "epub/content.opf", Data: []byte(renderOPF(doc, sections, r, modified, len(customCSS) > 0, fonts))},
		zipFile{Name: "epub/toc.xhtml", Data: []byte(renderNav(doc, sections, cfg.TOC.Title, cfg.TOC.Depth, len(customCSS) > 0))},
	)
	files = append(files, xhtmlFiles...)
	files = append(files, zipFile{Name: "epub/css/core.css", Data: coreCSS})
	if len(customCSS) > 0 {
		files = append(files, zipFile{Name: "epub/css/custom.css", Data: customCSS})
	}
	for _, font := range fonts {
		files = append(files, zipFile{Name: "epub/fonts/" + filepath.Base(font.Name), Data: font.Data})
	}
	for _, img := range r.images {
		files = append(files, zipFile{Name: "epub/images/" + img.Name, Data: img.Data})
	}
	data, err := writeZip(files, modified)
	if err != nil {
		return backend.EPUBResult{}, err
	}
	return backend.EPUBResult{Bytes: data, Issues: r.issues}, nil
}

func sourceDate(opts backend.RenderOptions) (time.Time, error) {
	if !opts.SourceDateUTC.IsZero() {
		return opts.SourceDateUTC.UTC(), nil
	}
	if raw := os.Getenv("SOURCE_DATE_EPOCH"); raw != "" {
		sec, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return time.Time{}, fmt.Errorf("config error: invalid SOURCE_DATE_EPOCH: %w", err)
		}
		return time.Unix(sec, 0).UTC(), nil
	}
	if opts.Reproducible {
		return time.Unix(0, 0).UTC(), nil
	}
	return time.Now().UTC(), nil
}

func readFonts(paths []string) ([]asset, error) {
	var fonts []asset
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read EPUB font: %w", err)
		}
		fonts = append(fonts, asset{
			ID:        fmt.Sprintf("font-%04d", i+1),
			Name:      filepath.Base(path),
			Data:      data,
			MediaType: fontMediaType(path),
		})
	}
	return fonts, nil
}

func fontMediaType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".otf":
		return "font/otf"
	case ".ttf":
		return "font/ttf"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return "application/octet-stream"
	}
}

const containerXML = `<?xml version="1.0" encoding="utf-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="epub/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

type zipFile struct {
	Name  string
	Data  []byte
	Store bool
}

func writeZip(files []zipFile, modified time.Time) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	zw.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})
	for _, file := range files {
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate}
		if file.Store {
			header.Method = zip.Store
		}
		header.SetModTime(modified)
		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, fmt.Errorf("create EPUB entry %s: %w", file.Name, err)
		}
		if _, err := w.Write(file.Data); err != nil {
			return nil, fmt.Errorf("write EPUB entry %s: %w", file.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func renderEndnotes(doc *ir.Document, r *renderer) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + attr(doc.Meta.Language) + `">` + "\n")
	b.WriteString("<head><title>Endnotes</title><link rel=\"stylesheet\" type=\"text/css\" href=\"../css/core.css\"/>")
	if r.customCSS {
		b.WriteString("<link rel=\"stylesheet\" type=\"text/css\" href=\"../css/custom.css\"/>")
	}
	b.WriteString("</head><body><section epub:type=\"endnotes\">\n")
	for _, def := range doc.Footnotes {
		fmt.Fprintf(&b, `<aside id="%s" epub:type="footnote">`, attr(def.ID))
		r.renderChildren(&b, def.Children)
		refs := r.noteRefs[def.ID]
		if len(refs) == 0 {
			refs = []noteRef{{File: "0001.xhtml", ID: "ref-" + def.ID + "-1"}}
		}
		for i, ref := range refs {
			fmt.Fprintf(&b, `<a id="back-%s-%d" epub:type="backlink" href="%s#%s">↩</a>`, attr(def.ID), i+1, attr(ref.File), attr(ref.ID))
		}
		b.WriteString("</aside>\n")
	}
	b.WriteString("</section></body></html>\n")
	return b.String()
}

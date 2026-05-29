package epub

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func renderOPF(doc *ir.Document, sections []section, r *renderer, modified time.Time, hasCustomCSS bool, fonts []asset) string {
	var b strings.Builder
	prefix := "dcterms: http://purl.org/dc/terms/"
	if doc.Meta.Extra["word_count"] != "" {
		prefix += " schema: http://schema.org/"
	}
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	fmt.Fprintf(&b, `<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" prefix="%s">`+"\n", attr(prefix))
	b.WriteString("<metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n")
	fmt.Fprintf(&b, `<dc:identifier id="bookid">%s</dc:identifier>`+"\n", text(doc.Meta.Identifier))
	fmt.Fprintf(&b, `<dc:title>%s</dc:title>`+"\n", text(doc.Meta.Title))
	fmt.Fprintf(&b, `<dc:language>%s</dc:language>`+"\n", text(doc.Meta.Language))
	if doc.Meta.Author != "" {
		fmt.Fprintf(&b, `<dc:creator>%s</dc:creator>`+"\n", text(doc.Meta.Author))
	}
	if doc.Meta.Publisher != "" {
		fmt.Fprintf(&b, `<dc:publisher>%s</dc:publisher>`+"\n", text(doc.Meta.Publisher))
	}
	if doc.Meta.Description != "" {
		fmt.Fprintf(&b, `<dc:description>%s</dc:description>`+"\n", text(doc.Meta.Description))
	}
	for _, key := range []string{"rights", "subject"} {
		if doc.Meta.Extra[key] != "" {
			fmt.Fprintf(&b, "<dc:%s>%s</dc:%s>\n", key, text(doc.Meta.Extra[key]), key)
		}
	}
	if doc.Meta.Extra["isbn"] != "" {
		fmt.Fprintf(&b, `<dc:identifier id="isbn">urn:isbn:%s</dc:identifier>`+"\n", text(doc.Meta.Extra["isbn"]))
	}
	if doc.Meta.Extra["series"] != "" {
		fmt.Fprintf(&b, `<meta property="belongs-to-collection" id="series">%s</meta>`+"\n", text(doc.Meta.Extra["series"]))
	}
	if doc.Meta.Extra["series_position"] != "" {
		fmt.Fprintf(&b, `<meta refines="#series" property="group-position">%s</meta>`+"\n", text(doc.Meta.Extra["series_position"]))
	}
	if doc.Meta.Extra["word_count"] != "" {
		fmt.Fprintf(&b, `<meta property="schema:wordCount">%s</meta>`+"\n", text(doc.Meta.Extra["word_count"]))
	}
	fmt.Fprintf(&b, `<meta property="dcterms:modified">%s</meta>`+"\n", modified.UTC().Format("2006-01-02T15:04:05Z"))
	if doc.Cover != nil {
		b.WriteString(`<meta name="cover" content="image-0001"/>` + "\n")
	}
	b.WriteString("</metadata>\n<manifest>\n")
	b.WriteString(`<item id="nav" href="toc.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	b.WriteString(`<item id="css" href="css/core.css" media-type="text/css"/>` + "\n")
	if hasCustomCSS {
		b.WriteString(`<item id="custom-css" href="css/custom.css" media-type="text/css"/>` + "\n")
	}
	for _, font := range fonts {
		fmt.Fprintf(&b, `<item id="%s" href="fonts/%s" media-type="%s"/>`+"\n", attr(font.ID), attr(filepath.Base(font.Name)), attr(font.MediaType))
	}
	if doc.Cover != nil {
		b.WriteString(`<item id="cover" href="text/cover.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	for _, sec := range sections {
		id := strings.TrimSuffix(sec.Name, ".xhtml")
		fmt.Fprintf(&b, `<item id="text-%s" href="text/%s" media-type="application/xhtml+xml"/>`+"\n", attr(id), attr(sec.Name))
	}
	if len(doc.Footnotes) > 0 {
		b.WriteString(`<item id="endnotes" href="text/endnotes.xhtml" media-type="application/xhtml+xml"/>` + "\n")
	}
	for i, img := range r.images {
		props := ""
		if doc.Cover != nil && i == 0 {
			props = ` properties="cover-image"`
		}
		fmt.Fprintf(&b, `<item id="image-%04d" href="images/%s" media-type="%s"%s/>`+"\n", i+1, attr(img.Name), attr(img.MediaType), props)
	}
	b.WriteString("</manifest>\n<spine>\n")
	if doc.Cover != nil {
		b.WriteString(`<itemref idref="cover"/>` + "\n")
	}
	for _, sec := range sections {
		id := strings.TrimSuffix(sec.Name, ".xhtml")
		fmt.Fprintf(&b, `<itemref idref="text-%s"/>`+"\n", attr(id))
	}
	if len(doc.Footnotes) > 0 {
		b.WriteString(`<itemref idref="endnotes"/>` + "\n")
	}
	b.WriteString("</spine>\n</package>\n")
	return b.String()
}

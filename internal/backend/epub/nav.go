package epub

import (
	"fmt"
	"strings"

	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func renderNav(doc *ir.Document, sections []section, title string, tocDepth int, hasCustomCSS bool) string {
	if title == "" {
		title = defaultTOCTitle(doc.Meta.Language)
	}
	custom := ""
	if hasCustomCSS {
		custom = `<link rel="stylesheet" type="text/css" href="css/custom.css"/>`
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + attr(doc.Meta.Language) + `">` + "\n")
	b.WriteString("<head><title>" + text(title) + `</title><link rel="stylesheet" type="text/css" href="css/core.css"/>` + custom + `</head>` + "\n")
	b.WriteString(`<body><nav epub:type="toc" id="toc"><h1>` + text(title) + "</h1><ol>\n")
	if tocDepth != 0 {
		for i, sec := range sections {
			label := sec.Title
			if label == "" {
				label = fmt.Sprintf("Section %d", i+1)
			}
			href := "text/" + sec.Name
			if len(sec.Headings) > 0 {
				href += "#" + sec.Headings[0].ID
			}
			fmt.Fprintf(&b, `<li><a href="%s">%s</a></li>`+"\n", attr(href), text(label))
		}
	}
	b.WriteString("</ol></nav></body></html>\n")
	return b.String()
}

func defaultTOCTitle(language string) string {
	switch strings.ToLower(language) {
	case "ru":
		return "Содержание"
	default:
		return "Contents"
	}
}

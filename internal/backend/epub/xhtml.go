package epub

import (
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type section struct {
	Name     string
	BodyType string
	SecType  string
	Blocks   []ir.Block
	Title    string
	Headings []headingRef
}

type headingRef struct {
	ID    string
	Level int
	Text  string
	File  string
}

type noteRef struct {
	File string
	ID   string
}

func splitSections(doc *ir.Document, tocDepth int) []section {
	minLevel := 7
	for _, block := range doc.Body {
		if h, ok := block.(ir.Heading); ok && h.Level < minLevel {
			minLevel = h.Level
		}
	}
	if minLevel == 7 {
		minLevel = 1
	}
	var sections []section
	current := section{BodyType: "frontmatter", SecType: "titlepage"}
	for _, block := range doc.Body {
		if h, ok := block.(ir.Heading); ok && h.Level == minLevel && len(current.Blocks) > 0 {
			sections = append(sections, current)
			current = section{BodyType: "bodymatter", SecType: "chapter"}
		} else if h, ok := block.(ir.Heading); ok && h.Level == minLevel && len(current.Blocks) == 0 {
			current.BodyType = "bodymatter"
			current.SecType = "chapter"
		}
		current.Blocks = append(current.Blocks, block)
	}
	if len(current.Blocks) > 0 {
		sections = append(sections, current)
	}
	if len(sections) == 0 {
		sections = []section{{BodyType: "bodymatter", SecType: "chapter", Blocks: doc.Body}}
	}
	slugCounts := map[string]int{}
	for i := range sections {
		sections[i].Name = fmt.Sprintf("%04d.xhtml", i+1)
		for j, block := range sections[i].Blocks {
			h, ok := block.(ir.Heading)
			if !ok {
				continue
			}
			if h.ID == "" {
				slug := slugify(inlineText(h.Inlines))
				slugCounts[slug]++
				h.ID = fmt.Sprintf("h-%s-%d", slug, slugCounts[slug])
				sections[i].Blocks[j] = h
			}
			if tocDepth != 0 && h.Level <= tocDepth {
				sections[i].Headings = append(sections[i].Headings, headingRef{ID: h.ID, Level: h.Level, Text: inlineText(h.Inlines), File: sections[i].Name})
			}
		}
		if len(sections[i].Headings) > 0 {
			sections[i].Title = sections[i].Headings[0].Text
		}
		if sections[i].Title == "" {
			sections[i].Title = fmt.Sprintf("Section %d", i+1)
		}
	}
	return sections
}

func collectHeadings(blocks []ir.Block, file string, tocDepth int, slugCounts map[string]int) []headingRef {
	var headings []headingRef
	for _, block := range blocks {
		h, ok := block.(ir.Heading)
		if !ok {
			continue
		}
		text := inlineText(h.Inlines)
		slug := slugify(text)
		slugCounts[slug]++
		id := fmt.Sprintf("h-%s-%d", slug, slugCounts[slug])
		if h.ID != "" {
			id = h.ID
		}
		if tocDepth == 0 || h.Level <= tocDepth {
			headings = append(headings, headingRef{ID: id, Level: h.Level, Text: text, File: file})
		}
	}
	return headings
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "heading"
	}
	return s
}

func renderSectionXHTML(doc *ir.Document, sec section, r *renderer) string {
	var b strings.Builder
	r.currentFile = sec.Name
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + attr(doc.Meta.Language) + `">` + "\n")
	b.WriteString("<head><title>" + text(sec.Title) + `</title><link rel="stylesheet" type="text/css" href="../css/core.css"/>`)
	if r.customCSS {
		b.WriteString(`<link rel="stylesheet" type="text/css" href="../css/custom.css"/>`)
	}
	b.WriteString("</head>\n")
	b.WriteString(`<body epub:type="` + attr(sec.BodyType) + `"><section epub:type="` + attr(sec.SecType) + `">` + "\n")
	for _, block := range sec.Blocks {
		r.renderBlock(&b, block)
	}
	b.WriteString("</section></body></html>\n")
	return b.String()
}

func renderCoverXHTML(doc *ir.Document, r *renderer) string {
	src := r.imageName(*doc.Cover)
	custom := ""
	if r.customCSS {
		custom = `<link rel="stylesheet" type="text/css" href="../css/custom.css"/>`
	}
	return fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="%s">
<head><title>Cover</title><link rel="stylesheet" type="text/css" href="../css/core.css"/>%s</head>
<body epub:type="frontmatter"><section epub:type="cover"><img src="../images/%s" alt="%s"/></section></body>
</html>
`, attr(doc.Meta.Language), custom, attr(src), attr(doc.Cover.Alt))
}

func (r *renderer) renderBlock(b *strings.Builder, block ir.Block) {
	switch v := block.(type) {
	case ir.Heading:
		level := clamp(v.Level, 1, 6)
		fmt.Fprintf(b, "<h%d id=\"%s\">%s</h%d>\n", level, attr(r.headingID(v)), r.renderInlines(v.Inlines), level)
	case ir.Paragraph:
		epubType := ""
		switch v.Role {
		case ir.RoleDedication:
			epubType = ` epub:type="dedication"`
		case ir.RoleColophon:
			epubType = ` epub:type="colophon"`
		}
		fmt.Fprintf(b, "<p%s>%s</p>\n", epubType, r.renderInlines(v.Inlines))
	case ir.Verse:
		b.WriteString(`<blockquote epub:type="z3998:poem">` + "\n")
		for _, stanza := range v.Stanzas {
			b.WriteString("<p>")
			for i, line := range stanza {
				if i > 0 {
					b.WriteString("<br/>")
				}
				b.WriteString(r.renderInlines(line.Inlines))
			}
			b.WriteString("</p>\n")
		}
		b.WriteString("</blockquote>\n")
	case ir.Letter:
		b.WriteString(`<blockquote epub:type="z3998:letter">` + "\n")
		r.renderChildren(b, v.Children)
		b.WriteString("</blockquote>\n")
	case ir.Epigraph:
		b.WriteString(`<blockquote epub:type="epigraph">` + "\n")
		r.renderChildren(b, v.Children)
		if len(v.Source) > 0 {
			b.WriteString("<cite>" + r.renderInlines(v.Source) + "</cite>\n")
		}
		b.WriteString("</blockquote>\n")
	case ir.BlockQuote:
		b.WriteString("<blockquote>\n")
		r.renderChildren(b, v.Children)
		b.WriteString("</blockquote>\n")
	case ir.List:
		tag := "ul"
		if v.Ordered {
			tag = "ol"
		}
		b.WriteString("<" + tag + ">\n")
		for _, item := range v.Items {
			b.WriteString("<li>")
			r.renderChildren(b, item.Children)
			b.WriteString("</li>\n")
		}
		b.WriteString("</" + tag + ">\n")
	case ir.Figure:
		src := r.imageName(v.Image)
		fmt.Fprintf(b, "<figure><img src=\"../images/%s\" alt=\"%s\"/>", attr(src), attr(v.Image.Alt))
		if len(v.Caption) > 0 {
			b.WriteString("<figcaption>" + r.renderInlines(v.Caption) + "</figcaption>")
		}
		b.WriteString("</figure>\n")
	case ir.Table:
		b.WriteString("<table>")
		headerOpen := false
		bodyOpen := false
		for _, row := range v.Rows {
			if row.Header && !bodyOpen {
				if !headerOpen {
					b.WriteString("<thead>\n")
					headerOpen = true
				}
			} else {
				if headerOpen {
					b.WriteString("</thead>\n")
					headerOpen = false
				}
				if !bodyOpen {
					b.WriteString("<tbody>\n")
					bodyOpen = true
				}
			}
			b.WriteString("<tr>")
			for _, cell := range row.Cells {
				tag := "td"
				if row.Header {
					tag = "th"
				}
				span := ""
				if cell.ColSpan > 1 {
					span += ` colspan="` + strconv.Itoa(cell.ColSpan) + `"`
				}
				if cell.RowSpan > 1 {
					span += ` rowspan="` + strconv.Itoa(cell.RowSpan) + `"`
				}
				var cellBody strings.Builder
				r.renderChildren(&cellBody, cell.Children)
				fmt.Fprintf(b, "<%s%s>%s</%s>", tag, span, cellBody.String(), tag)
			}
			b.WriteString("</tr>\n")
		}
		if headerOpen {
			b.WriteString("</thead>\n")
		}
		if bodyOpen {
			b.WriteString("</tbody>\n")
		}
		b.WriteString("</table>\n")
	case ir.HorizontalRule:
		b.WriteString("<hr/>\n")
	case ir.PageBreak:
		r.issues = append(r.issues, ir.Issue{Severity: "warning", Code: "t-warn-pagebreak", Message: "EPUB ignores page breaks"})
	}
}

func (r *renderer) renderChildren(b *strings.Builder, children []ir.Block) {
	for _, child := range children {
		r.renderBlock(b, child)
	}
}

func (r *renderer) renderInlines(inlines []ir.Inline) string {
	var b strings.Builder
	for _, inline := range inlines {
		switch v := inline.(type) {
		case ir.Text:
			b.WriteString(text(v.Value))
		case ir.LineBreak:
			b.WriteString("<br/>")
		case ir.Styled:
			body := r.renderInlines(v.Children)
			switch v.Role {
			case ir.Emphasis:
				b.WriteString("<em>" + body + "</em>")
			case ir.Strong:
				b.WriteString("<strong>" + body + "</strong>")
			case ir.Foreign:
				b.WriteString(`<i xml:lang="` + attr(v.Lang) + `">` + body + "</i>")
			case ir.Thought:
				b.WriteString(`<i epub:type="z3998:thought">` + body + "</i>")
			case ir.Prayer:
				b.WriteString(`<i epub:type="z3998:prayer">` + body + "</i>")
			case ir.WorkTitle:
				b.WriteString(`<i epub:type="se:name.publication">` + body + "</i>")
			default:
				b.WriteString(body)
			}
		case ir.Link:
			if safeLink(v.Href) {
				b.WriteString(`<a href="` + attr(v.Href) + `">` + r.renderInlines(v.Children) + "</a>")
			} else {
				r.issues = append(r.issues, ir.Issue{Severity: "warning", Code: "t-warn-link-scheme", Message: "unsupported link scheme", Context: map[string]string{"href": v.Href}})
				b.WriteString(r.renderInlines(v.Children))
			}
		case ir.FootnoteRef:
			n := len(r.noteRefs[v.ID]) + 1
			refID := fmt.Sprintf("ref-%s-%d", v.ID, n)
			r.noteRefs[v.ID] = append(r.noteRefs[v.ID], noteRef{File: r.currentFile, ID: refID})
			b.WriteString(`<a id="` + attr(refID) + `" epub:type="noteref" href="endnotes.xhtml#` + attr(v.ID) + `">` + strconv.Itoa(n) + `</a>`)
		case ir.InlineImage:
			src := r.imageName(v.Image)
			b.WriteString(`<img src="../images/` + attr(src) + `" alt="` + attr(v.Image.Alt) + `"/>`)
		}
	}
	return b.String()
}

func (r *renderer) headingID(h ir.Heading) string {
	if h.ID != "" {
		return h.ID
	}
	text := inlineText(h.Inlines)
	slug := slugify(text)
	r.headingCounts[slug]++
	return fmt.Sprintf("h-%s-%d", slug, r.headingCounts[slug])
}

func (r *renderer) imageName(image ir.Image) string {
	key := backend.ImageKey(image)
	if name := r.imageRefs[key]; name != "" {
		return name
	}
	ext := strings.ToLower(filepath.Ext(image.Name))
	if ext == "" {
		ext = extFromMediaType(image.MediaType)
	}
	name := fmt.Sprintf("image-%04d%s", len(r.images)+1, ext)
	r.imageRefs[key] = name
	r.images = append(r.images, imageResource{Name: name, Data: image.Data, MediaType: image.MediaType})
	return name
}

func extFromMediaType(mt string) string {
	switch mt {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/svg+xml":
		return ".svg"
	default:
		return ".bin"
	}
}

func text(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return replacer.Replace(s)
}

func attr(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return replacer.Replace(s)
}

func inlineText(inlines []ir.Inline) string {
	var out strings.Builder
	for _, inline := range inlines {
		switch v := inline.(type) {
		case ir.Text:
			out.WriteString(v.Value)
		case ir.Styled:
			out.WriteString(inlineText(v.Children))
		case ir.Link:
			out.WriteString(inlineText(v.Children))
		}
	}
	return strings.TrimSpace(out.String())
}

func blockText(blocks []ir.Block) string {
	var out strings.Builder
	for _, block := range blocks {
		switch v := block.(type) {
		case ir.Heading:
			out.WriteString(inlineText(v.Inlines))
		case ir.Paragraph:
			out.WriteString(inlineText(v.Inlines))
		case ir.Letter:
			out.WriteString(blockText(v.Children))
		case ir.Epigraph:
			out.WriteString(blockText(v.Children))
		case ir.BlockQuote:
			out.WriteString(blockText(v.Children))
		}
		out.WriteByte(' ')
	}
	return strings.TrimSpace(out.String())
}

func safeLink(href string) bool {
	lower := strings.ToLower(href)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") ||
		strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "#") {
		return true
	}
	if strings.Contains(lower, ":") || strings.HasPrefix(href, "/") || strings.Contains(href, `\`) {
		return false
	}
	clean := path.Clean(href)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

package odt

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/internal/parser/common"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type contentParser struct {
	cfg        *config.Config
	styles     styleInfo
	images     map[string][]byte
	issues     []ir.Issue
	report     parser.StyleReport
	pResolver  *common.StyleResolver
	cResolver  *common.StyleResolver
	strict     bool
	footnotes  []ir.FootnoteDef
	fnSeen     map[string]int
	fnSequence int
}

const (
	odtNSText  = "urn:oasis:names:tc:opendocument:xmlns:text:1.0"
	odtNSDraw  = "urn:oasis:names:tc:opendocument:xmlns:drawing:1.0"
	odtNSTable = "urn:oasis:names:tc:opendocument:xmlns:table:1.0"
)

func odtName(name xml.Name, space, local string) bool {
	return name.Space == space && name.Local == local
}

func parseContent(data []byte, zr *zip.Reader, cfg *config.Config, styles styleInfo, strict bool) ([]ir.Block, []ir.FootnoteDef, []ir.Issue, parser.StyleReport, error) {
	images, err := common.ReadZipPrefix(zr, "Pictures/", common.MaxImageBytes)
	if err != nil {
		return nil, nil, nil, parser.StyleReport{}, err
	}
	p := &contentParser{
		cfg:       cfg,
		styles:    styles,
		images:    images,
		pResolver: common.NewStyleResolver("paragraph", cfg.ParagraphStyles, styles.Paragraph, cfg.StyleMatching.NormalizedFallback),
		cResolver: common.NewStyleResolver("character", cfg.CharacterStyles, styles.Character, cfg.StyleMatching.NormalizedFallback),
		strict:    strict,
		fnSeen:    map[string]int{},
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var blocks []ir.Block
	for {
		token, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, nil, nil, parser.StyleReport{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case odtName(start.Name, odtNSText, "h"):
			block, err := p.parseHeading(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, block)
		case odtName(start.Name, odtNSText, "p"):
			newBlocks, err := p.parseParagraph(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, newBlocks...)
		case odtName(start.Name, odtNSText, "list"):
			list, err := p.parseList(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, list)
		case odtName(start.Name, odtNSTable, "table"):
			table, err := p.parseTable(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, table)
		}
	}
	return groupStructural(common.AttachCaptions(blocks)), p.footnotes, p.issues, p.report, nil
}

func (p *contentParser) parseHeading(dec *xml.Decoder, start xml.StartElement) (ir.Block, error) {
	level := 1
	if raw := common.Attr(start, "outline-level"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			level = parsed
		}
	}
	inlines, err := p.parseInlines(dec, start)
	if err != nil {
		return nil, err
	}
	styleName := common.Attr(start, "style-name")
	if styleName != "" {
		resolved := p.pResolver.Resolve(styleName, p.styles.Paragraph[styleName].DisplayName, p.strict)
		common.AddStyleUsage(&p.report.ParagraphStyles, common.StyleUsage(styleName, "paragraph", resolved, "heading"))
		p.issues = append(p.issues, resolved.Issues...)
	}
	return ir.Heading{Level: level, Inlines: trimInlines(inlines)}, nil
}

func (p *contentParser) parseParagraph(dec *xml.Decoder, start xml.StartElement) ([]ir.Block, error) {
	styleName := common.Attr(start, "style-name")
	resolved := p.pResolver.Resolve(styleName, p.styles.Paragraph[styleName].DisplayName, p.strict)
	if styleName != "" {
		common.AddStyleUsage(&p.report.ParagraphStyles, common.StyleUsage(styleName, "paragraph", resolved, "body"))
		p.issues = append(p.issues, resolved.Issues...)
	}
	inlines, err := p.parseInlines(dec, start)
	if err != nil {
		return nil, err
	}
	inlines = trimInlines(inlines)
	if len(inlines) == 0 {
		return nil, nil
	}
	var out []ir.Block
	if p.styles.PageBreak[styleName] {
		out = append(out, ir.PageBreak{})
	}
	if len(inlines) == 1 {
		if img, ok := inlines[0].(ir.InlineImage); ok {
			out = append(out, ir.Figure{Image: img.Image})
			return out, nil
		}
	}
	displayName := p.styles.Paragraph[styleName].DisplayName
	if styleName != "" && common.IsCaptionStyle(styleName, displayName) {
		name := displayName
		if name == "" {
			name = styleName
		}
		common.AddStyleUsage(&p.report.ParagraphStyles, common.CaptionStyleUsage(name))
		out = append(out, ir.Paragraph{Role: common.CaptionBlockRole, Inlines: inlines})
		return out, nil
	}
	if resolved.Status == "unknown" && styleName != "" {
		p.issues = append(p.issues, common.UnknownStyleIssue("paragraph", styleName))
	}
	switch resolved.Role {
	case "heading":
		level := resolved.Mapping.Level
		if level == 0 {
			level = 1
		}
		out = append(out, ir.Heading{Level: level, Inlines: inlines})
	case "verse":
		out = append(out, ir.Paragraph{Role: ir.BlockRole("verse"), Inlines: inlines})
	case "letter":
		out = append(out, ir.Paragraph{Role: ir.BlockRole("letter"), Inlines: inlines})
	case "epigraph":
		out = append(out, ir.Paragraph{Role: ir.BlockRole("epigraph"), Inlines: inlines})
	case "blockquote":
		out = append(out, ir.Paragraph{Role: ir.BlockRole("blockquote"), Inlines: inlines})
	default:
		out = append(out, ir.Paragraph{Role: common.ParagraphRole(resolved.Mapping), Inlines: inlines})
	}
	return out, nil
}

func (p *contentParser) parseInlines(dec *xml.Decoder, end xml.StartElement) ([]ir.Inline, error) {
	var inlines []ir.Inline
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.CharData:
			text := common.NormalizeText(string(t))
			if text != "" {
				inlines = append(inlines, ir.Text{Value: text})
			}
		case xml.StartElement:
			switch {
			case odtName(t.Name, odtNSText, "span"):
				children, err := p.parseInlines(dec, t)
				if err != nil {
					return nil, err
				}
				styleName := common.Attr(t, "style-name")
				inlines = append(inlines, p.styleSpan(styleName, children)...)
			case odtName(t.Name, odtNSText, "a"):
				href := common.Attr(t, "href")
				children, err := p.parseInlines(dec, t)
				if err != nil {
					return nil, err
				}
				if safeLink(href) {
					inlines = append(inlines, ir.Link{Href: href, Children: children})
				} else {
					p.issues = append(p.issues, ir.Issue{Severity: "warning", Code: "t-warn-link-scheme", Message: "unsupported link scheme", Context: map[string]string{"href": href}})
					inlines = append(inlines, children...)
				}
			case odtName(t.Name, odtNSText, "s"):
				count := 1
				if raw := common.Attr(t, "c"); raw != "" {
					if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
						count = parsed
					}
				}
				inlines = append(inlines, ir.Text{Value: strings.Repeat(" ", count)})
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			case odtName(t.Name, odtNSText, "tab"):
				inlines = append(inlines, ir.Text{Value: " "})
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			case odtName(t.Name, odtNSText, "line-break"):
				inlines = append(inlines, ir.LineBreak{})
			case odtName(t.Name, odtNSText, "note"):
				ref, err := p.parseNote(dec, t)
				if err != nil {
					return nil, err
				}
				if ref.ID != "" {
					inlines = append(inlines, ref)
				}
			case odtName(t.Name, odtNSDraw, "frame"):
				img, err := p.parseFrame(dec, t)
				if err != nil {
					return nil, err
				}
				if img.Name != "" {
					inlines = append(inlines, ir.InlineImage{Image: img})
				}
			default:
				children, err := p.parseInlines(dec, t)
				if err != nil {
					return nil, err
				}
				inlines = append(inlines, children...)
			}
		case xml.EndElement:
			if t.Name.Local == end.Name.Local && t.Name.Space == end.Name.Space {
				return inlines, nil
			}
		}
	}
}

func (p *contentParser) styleSpan(styleName string, children []ir.Inline) []ir.Inline {
	if styleName == "" {
		return children
	}
	def := p.styles.Character[styleName]
	resolved := p.cResolver.Resolve(styleName, def.DisplayName, p.strict)
	common.AddStyleUsage(&p.report.CharacterStyles, common.StyleUsage(styleName, "character", resolved, "emphasis"))
	p.issues = append(p.issues, resolved.Issues...)
	if resolved.Status == "unknown" {
		if p.styles.Bold[styleName] {
			p.addDirect("strong")
			return []ir.Inline{ir.Styled{Role: ir.Strong, Children: children}}
		}
		if p.styles.Italic[styleName] {
			p.addDirect("emphasis")
			return []ir.Inline{ir.Styled{Role: ir.Emphasis, Children: children}}
		}
		p.issues = append(p.issues, common.UnknownStyleIssue("character", styleName))
		return children
	}
	return []ir.Inline{ir.Styled{Role: common.InlineRole(resolved.Mapping), Lang: resolved.Mapping.Lang, Children: children}}
}

func (p *contentParser) parseFrame(dec *xml.Decoder, start xml.StartElement) (ir.Image, error) {
	alt := common.Attr(start, "name")
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.Image{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSDraw, "image") {
				href := strings.TrimPrefix(common.Attr(t, "href"), "./")
				data := p.images[href]
				if len(data) == 0 {
					data = p.images[path.Clean(href)]
				}
				if err := common.SkipElement(dec, t); err != nil {
					return ir.Image{}, err
				}
				return ir.Image{Name: path.Base(href), Data: data, MediaType: mediaTypeFromName(href), Alt: alt}, nil
			}
			if err := common.SkipElement(dec, t); err != nil {
				return ir.Image{}, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.Image{}, nil
			}
		}
	}
}

func (p *contentParser) parseNote(dec *xml.Decoder, start xml.StartElement) (ir.FootnoteRef, error) {
	if cls := common.Attr(start, "note-class"); cls != "" && cls != "footnote" {
		if err := common.SkipElement(dec, start); err != nil {
			return ir.FootnoteRef{}, err
		}
		return ir.FootnoteRef{}, nil
	}
	sourceID := common.Attr(start, "id")
	id := p.normalizedFootnoteID(sourceID)
	var children []ir.Block
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.FootnoteRef{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSText, "p") {
				blocks, err := p.parseParagraph(dec, t)
				if err != nil {
					return ir.FootnoteRef{}, err
				}
				children = append(children, blocks...)
			} else {
				if err := common.SkipElement(dec, t); err != nil {
					return ir.FootnoteRef{}, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				if len(children) == 0 {
					children = []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: " "}}}}
				}
				p.footnotes = append(p.footnotes, ir.FootnoteDef{ID: id, Children: children})
				return ir.FootnoteRef{ID: id}, nil
			}
		}
	}
}

func (p *contentParser) normalizedFootnoteID(source string) string {
	source = strings.TrimSpace(source)
	if source != "" {
		clean := make([]rune, 0, len(source))
		for _, r := range source {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			id := "fn-" + strings.ToLower(string(clean))
			p.fnSeen[id]++
			if p.fnSeen[id] > 1 {
				return fmt.Sprintf("%s-%d", id, p.fnSeen[id])
			}
			return id
		}
	}
	p.fnSequence++
	return fmt.Sprintf("fn-%04d", p.fnSequence)
}

func (p *contentParser) parseList(dec *xml.Decoder, start xml.StartElement) (ir.Block, error) {
	var items []ir.ListItem
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSText, "list-item") {
				children, err := p.parseListItem(dec, t)
				if err != nil {
					return nil, err
				}
				items = append(items, ir.ListItem{Children: children})
			} else if err := common.SkipElement(dec, t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.List{Items: items}, nil
			}
		}
	}
}

func (p *contentParser) parseListItem(dec *xml.Decoder, start xml.StartElement) ([]ir.Block, error) {
	var blocks []ir.Block
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case odtName(t.Name, odtNSText, "p"):
				newBlocks, err := p.parseParagraph(dec, t)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, newBlocks...)
			case odtName(t.Name, odtNSText, "list"):
				list, err := p.parseList(dec, t)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, list)
			default:
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return blocks, nil
			}
		}
	}
}

func (p *contentParser) parseTable(dec *xml.Decoder, start xml.StartElement) (ir.Block, error) {
	var rows []ir.TableRow
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSTable, "table-row") {
				row, err := p.parseTableRow(dec, t)
				if err != nil {
					return nil, err
				}
				rows = append(rows, row)
			} else if err := common.SkipElement(dec, t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.Table{Rows: rows}, nil
			}
		}
	}
}

func (p *contentParser) parseTableRow(dec *xml.Decoder, start xml.StartElement) (ir.TableRow, error) {
	var cells []ir.TableCell
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.TableRow{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSTable, "table-cell") {
				cell, err := p.parseTableCell(dec, t)
				if err != nil {
					return ir.TableRow{}, err
				}
				cells = append(cells, cell)
			} else if err := common.SkipElement(dec, t); err != nil {
				return ir.TableRow{}, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.TableRow{Cells: cells}, nil
			}
		}
	}
}

func (p *contentParser) parseTableCell(dec *xml.Decoder, start xml.StartElement) (ir.TableCell, error) {
	var blocks []ir.Block
	colSpan := 1
	if raw := common.Attr(start, "number-columns-spanned"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			colSpan = parsed
		}
	}
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.TableCell{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if odtName(t.Name, odtNSText, "p") {
				newBlocks, err := p.parseParagraph(dec, t)
				if err != nil {
					return ir.TableCell{}, err
				}
				blocks = append(blocks, newBlocks...)
			} else if err := common.SkipElement(dec, t); err != nil {
				return ir.TableCell{}, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.TableCell{Children: blocks, ColSpan: colSpan, RowSpan: 1}, nil
			}
		}
	}
}

func (p *contentParser) addDirect(role string) {
	for i := range p.report.DirectFormatting {
		if p.report.DirectFormatting[i].Role == role {
			p.report.DirectFormatting[i].Count++
			return
		}
	}
	p.report.DirectFormatting = append(p.report.DirectFormatting, parser.DirectFormattingStat{Role: role, Count: 1})
}

func trimInlines(inlines []ir.Inline) []ir.Inline {
	if len(inlines) == 0 {
		return inlines
	}
	if t, ok := inlines[0].(ir.Text); ok {
		t.Value = strings.TrimLeft(t.Value, " \t\r\n")
		if t.Value == "" {
			inlines = inlines[1:]
		} else {
			inlines[0] = t
		}
	}
	if len(inlines) > 0 {
		if t, ok := inlines[len(inlines)-1].(ir.Text); ok {
			t.Value = strings.TrimRight(t.Value, " \t\r\n")
			if t.Value == "" {
				inlines = inlines[:len(inlines)-1]
			} else {
				inlines[len(inlines)-1] = t
			}
		}
	}
	return inlines
}

func groupStructural(blocks []ir.Block) []ir.Block {
	var out []ir.Block
	for i := 0; i < len(blocks); {
		p, ok := blocks[i].(ir.Paragraph)
		if !ok {
			out = append(out, blocks[i])
			i++
			continue
		}
		role := string(p.Role)
		switch role {
		case "verse":
			var stanzas [][]ir.Line
			for i < len(blocks) {
				p, ok := blocks[i].(ir.Paragraph)
				if !ok || string(p.Role) != role {
					break
				}
				stanzas = append(stanzas, splitVerseLines(p.Inlines))
				i++
			}
			out = append(out, ir.Verse{Stanzas: stanzas})
		case "letter", "epigraph", "blockquote":
			var children []ir.Block
			for i < len(blocks) {
				p, ok := blocks[i].(ir.Paragraph)
				if !ok || string(p.Role) != role {
					break
				}
				children = append(children, ir.Paragraph{Role: ir.RoleBody, Inlines: p.Inlines})
				i++
			}
			switch role {
			case "letter":
				out = append(out, ir.Letter{Children: children})
			case "epigraph":
				out = append(out, ir.Epigraph{Children: children})
			case "blockquote":
				out = append(out, ir.BlockQuote{Children: children})
			}
		default:
			out = append(out, blocks[i])
			i++
		}
	}
	return out
}

func splitVerseLines(inlines []ir.Inline) []ir.Line {
	var lines []ir.Line
	var current []ir.Inline
	for _, inline := range inlines {
		if _, ok := inline.(ir.LineBreak); ok {
			lines = append(lines, ir.Line{Inlines: current})
			current = nil
			continue
		}
		current = append(current, inline)
	}
	lines = append(lines, ir.Line{Inlines: current})
	return lines
}

func mediaTypeFromName(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

func safeLink(href string) bool {
	if href == "" {
		return false
	}
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

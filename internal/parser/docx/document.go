package docx

import (
	"bytes"
	"encoding/xml"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/internal/parser/common"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type documentParser struct {
	cfg          *config.Config
	styles       styleInfo
	rels         map[string]string
	images       map[string][]byte
	numbering    map[string]bool
	pResolver    *common.StyleResolver
	cResolver    *common.StyleResolver
	strict       bool
	issues       []ir.Issue
	report       parser.StyleReport
	footnoteMap  map[string][]ir.Block
	footnotes    []ir.FootnoteDef
	fnBySource   map[string]string
	fnSeen       map[string]int
	fnSequence   int
	pageBreakRun bool
}

const (
	docxNSW  = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	docxNSA  = "http://schemas.openxmlformats.org/drawingml/2006/main"
	docxNSWP = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
)

func docxName(name xml.Name, space, local string) bool {
	return name.Space == space && name.Local == local
}

func newDocumentParser(cfg *config.Config, styles styleInfo, rels map[string]string, images map[string][]byte, numbering map[string]bool, strict bool) *documentParser {
	return &documentParser{
		cfg:         cfg,
		styles:      styles,
		rels:        rels,
		images:      images,
		numbering:   numbering,
		pResolver:   common.NewStyleResolver("paragraph", cfg.ParagraphStyles, styles.Paragraph, cfg.StyleMatching.NormalizedFallback),
		cResolver:   common.NewStyleResolver("character", cfg.CharacterStyles, styles.Character, cfg.StyleMatching.NormalizedFallback),
		strict:      strict,
		fnBySource:  map[string]string{},
		fnSeen:      map[string]int{},
		footnoteMap: map[string][]ir.Block{},
	}
}

func parseDocument(data []byte, cfg *config.Config, styles styleInfo, rels map[string]string, images map[string][]byte, numbering map[string]bool, footnoteMap map[string][]ir.Block, strict bool) ([]ir.Block, []ir.FootnoteDef, []ir.Issue, parser.StyleReport, error) {
	p := newDocumentParser(cfg, styles, rels, images, numbering, strict)
	p.footnoteMap = footnoteMap
	dec := xml.NewDecoder(bytes.NewReader(data))
	var blocks []ir.Block
	for {
		token, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, nil, parser.StyleReport{}, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case docxName(start.Name, docxNSW, "p"):
			newBlocks, err := p.parseParagraph(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, newBlocks...)
		case docxName(start.Name, docxNSW, "tbl"):
			table, err := p.parseTable(dec, start)
			if err != nil {
				return nil, nil, nil, parser.StyleReport{}, err
			}
			blocks = append(blocks, table)
		}
	}
	return groupStructural(groupLists(common.AttachCaptions(blocks))), p.footnotes, p.issues, p.report, nil
}

func (p *documentParser) parseParagraph(dec *xml.Decoder, start xml.StartElement) ([]ir.Block, error) {
	props := paragraphProps{}
	var inlines []ir.Inline
	p.pageBreakRun = false
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "pPr"):
				parsed, err := p.parseParagraphProps(dec, t)
				if err != nil {
					return nil, err
				}
				props = parsed
			case docxName(t.Name, docxNSW, "r"):
				run, err := p.parseRun(dec, t)
				if err != nil {
					return nil, err
				}
				inlines = append(inlines, run...)
			case docxName(t.Name, docxNSW, "hyperlink"):
				children, href, err := p.parseHyperlink(dec, t)
				if err != nil {
					return nil, err
				}
				if safeLink(href) {
					inlines = append(inlines, ir.Link{Href: href, Children: children})
				} else {
					p.issues = append(p.issues, ir.Issue{Severity: "warning", Code: "t-warn-link-scheme", Message: "unsupported link scheme", Context: map[string]string{"href": href}})
					inlines = append(inlines, children...)
				}
			default:
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return p.paragraphToBlocks(props, trimInlines(inlines)), nil
			}
		}
	}
}

type paragraphProps struct {
	StyleID string
	NumID   string
	Ordered bool
}

func (p *documentParser) parseParagraphProps(dec *xml.Decoder, start xml.StartElement) (paragraphProps, error) {
	var props paragraphProps
	for {
		token, err := dec.Token()
		if err != nil {
			return props, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "pStyle"):
				props.StyleID = common.Attr(t, "val")
			case docxName(t.Name, docxNSW, "numId"):
				props.NumID = common.Attr(t, "val")
				ordered, ok := p.numbering[props.NumID]
				if !ok {
					p.issues = append(p.issues, ir.Issue{Severity: "warning", Code: "t-warn-numbering", Message: "unknown DOCX list numbering", Context: map[string]string{"num_id": props.NumID}})
				}
				props.Ordered = ordered
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return props, nil
			}
		}
	}
}

func (p *documentParser) paragraphToBlocks(props paragraphProps, inlines []ir.Inline) []ir.Block {
	if len(inlines) == 0 && !p.pageBreakRun {
		return nil
	}
	var out []ir.Block
	if p.pageBreakRun {
		out = append(out, ir.PageBreak{})
	}
	if len(inlines) == 1 {
		if img, ok := inlines[0].(ir.InlineImage); ok {
			out = append(out, ir.Figure{Image: img.Image})
			return out
		}
	}
	if props.NumID != "" {
		role := ir.BlockRole("__list_unordered")
		if props.Ordered {
			role = ir.BlockRole("__list_ordered")
		}
		out = append(out, ir.Paragraph{Role: role, Inlines: inlines})
		return out
	}
	def := p.styles.Paragraph[props.StyleID]
	if props.StyleID != "" && common.IsCaptionStyle(props.StyleID, def.DisplayName) {
		name := def.DisplayName
		if name == "" {
			name = props.StyleID
		}
		common.AddStyleUsage(&p.report.ParagraphStyles, common.CaptionStyleUsage(name))
		out = append(out, ir.Paragraph{Role: common.CaptionBlockRole, Inlines: inlines})
		return out
	}
	resolved := p.pResolver.Resolve(props.StyleID, def.DisplayName, p.strict)
	if props.StyleID != "" {
		common.AddStyleUsage(&p.report.ParagraphStyles, common.StyleUsage(def.DisplayName, "paragraph", resolved, "body"))
		p.issues = append(p.issues, resolved.Issues...)
	}
	if resolved.Status == "unknown" && props.StyleID != "" {
		p.issues = append(p.issues, common.UnknownStyleIssue("paragraph", def.DisplayName))
	}
	switch resolved.Role {
	case "heading":
		level := resolved.Mapping.Level
		if level == 0 {
			level = headingLevel(def.DisplayName)
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
	return out
}

func (p *documentParser) parseRun(dec *xml.Decoder, start xml.StartElement) ([]ir.Inline, error) {
	props := runProps{}
	var children []ir.Inline
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "rPr"):
				parsed, err := p.parseRunProps(dec, t)
				if err != nil {
					return nil, err
				}
				props = parsed
			case docxName(t.Name, docxNSW, "t"):
				text, err := common.ReadTextElement(dec, t)
				if err != nil {
					return nil, err
				}
				if common.AttrNS(t, "http://www.w3.org/XML/1998/namespace", "space") != "preserve" {
					text = common.NormalizeText(text)
				}
				if text != "" {
					children = append(children, ir.Text{Value: text})
				}
			case docxName(t.Name, docxNSW, "tab"):
				children = append(children, ir.Text{Value: " "})
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			case docxName(t.Name, docxNSW, "br"):
				if common.Attr(t, "type") == "page" {
					p.pageBreakRun = true
				} else {
					children = append(children, ir.LineBreak{})
				}
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			case docxName(t.Name, docxNSW, "footnoteReference"):
				sourceID := common.Attr(t, "id")
				id := p.footnoteID(sourceID)
				children = append(children, ir.FootnoteRef{ID: id})
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			case docxName(t.Name, docxNSW, "drawing"):
				img, err := p.parseDrawing(dec, t)
				if err != nil {
					return nil, err
				}
				if img.Name != "" {
					children = append(children, ir.InlineImage{Image: img})
				}
			default:
				if err := common.SkipElement(dec, t); err != nil {
					return nil, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return p.applyRunProps(props, children), nil
			}
		}
	}
}

type runProps struct {
	StyleID string
	Bold    bool
	Italic  bool
}

func (p *documentParser) parseRunProps(dec *xml.Decoder, start xml.StartElement) (runProps, error) {
	var props runProps
	for {
		token, err := dec.Token()
		if err != nil {
			return props, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "rStyle"):
				props.StyleID = common.Attr(t, "val")
			case docxName(t.Name, docxNSW, "b"):
				props.Bold = common.Attr(t, "val") != "0" && common.Attr(t, "val") != "false"
			case docxName(t.Name, docxNSW, "i"):
				props.Italic = common.Attr(t, "val") != "0" && common.Attr(t, "val") != "false"
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return props, nil
			}
		}
	}
}

func (p *documentParser) applyRunProps(props runProps, children []ir.Inline) []ir.Inline {
	if len(children) == 0 {
		return nil
	}
	if props.StyleID != "" {
		def := p.styles.Character[props.StyleID]
		resolved := p.cResolver.Resolve(props.StyleID, def.DisplayName, p.strict)
		common.AddStyleUsage(&p.report.CharacterStyles, common.StyleUsage(def.DisplayName, "character", resolved, "emphasis"))
		p.issues = append(p.issues, resolved.Issues...)
		if resolved.Status != "unknown" {
			return []ir.Inline{ir.Styled{Role: common.InlineRole(resolved.Mapping), Lang: resolved.Mapping.Lang, Children: children}}
		}
		p.issues = append(p.issues, common.UnknownStyleIssue("character", def.DisplayName))
	}
	if props.Bold {
		p.addDirect("strong")
		children = []ir.Inline{ir.Styled{Role: ir.Strong, Children: children}}
	}
	if props.Italic {
		p.addDirect("emphasis")
		children = []ir.Inline{ir.Styled{Role: ir.Emphasis, Children: children}}
	}
	return children
}

func (p *documentParser) parseHyperlink(dec *xml.Decoder, start xml.StartElement) ([]ir.Inline, string, error) {
	href := p.rels[common.Attr(start, "id")]
	if href == "" {
		href = "#" + common.Attr(start, "anchor")
	}
	var children []ir.Inline
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, "", err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if docxName(t.Name, docxNSW, "r") {
				run, err := p.parseRun(dec, t)
				if err != nil {
					return nil, "", err
				}
				children = append(children, run...)
			} else if err := common.SkipElement(dec, t); err != nil {
				return nil, "", err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return children, href, nil
			}
		}
	}
}

func (p *documentParser) parseDrawing(dec *xml.Decoder, start xml.StartElement) (ir.Image, error) {
	var relID, alt string
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.Image{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if docxName(t.Name, docxNSWP, "docPr") {
				alt = common.Attr(t, "descr")
				if alt == "" {
					alt = common.Attr(t, "title")
				}
			}
			if docxName(t.Name, docxNSA, "blip") {
				relID = common.Attr(t, "embed")
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				target := p.rels[relID]
				target = strings.TrimPrefix(target, "/")
				target = strings.TrimPrefix(target, "word/")
				name := "word/" + target
				if !strings.HasPrefix(name, "word/media/") {
					name = "word/media/" + path.Base(target)
				}
				data := p.images[name]
				return ir.Image{Name: path.Base(name), Data: data, MediaType: mediaTypeFromName(name), Alt: alt}, nil
			}
		}
	}
}

func (p *documentParser) parseFootnoteBody(dec *xml.Decoder, start xml.StartElement) ([]ir.Block, error) {
	var blocks []ir.Block
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if docxName(t.Name, docxNSW, "p") {
				newBlocks, err := p.parseParagraph(dec, t)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, newBlocks...)
			} else if err := common.SkipElement(dec, t); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return blocks, nil
			}
		}
	}
}

func (p *documentParser) footnoteID(sourceID string) string {
	if existing := p.fnBySource[sourceID]; existing != "" {
		return existing
	}
	id := normalizedFootnoteID(sourceID, p.fnSeen, &p.fnSequence)
	children := p.footnoteMap[sourceID]
	if len(children) == 0 {
		children = []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: " "}}}}
	}
	p.footnotes = append(p.footnotes, ir.FootnoteDef{ID: id, Children: children})
	p.fnBySource[sourceID] = id
	return id
}

func (p *documentParser) parseTable(dec *xml.Decoder, start xml.StartElement) (ir.Block, error) {
	var rows []ir.TableRow
	for {
		token, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if docxName(t.Name, docxNSW, "tr") {
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

func (p *documentParser) parseTableRow(dec *xml.Decoder, start xml.StartElement) (ir.TableRow, error) {
	var cells []ir.TableCell
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.TableRow{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			if docxName(t.Name, docxNSW, "tc") {
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

func (p *documentParser) parseTableCell(dec *xml.Decoder, start xml.StartElement) (ir.TableCell, error) {
	var blocks []ir.Block
	colSpan := 1
	for {
		token, err := dec.Token()
		if err != nil {
			return ir.TableCell{}, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "gridSpan"):
				if parsed, err := strconv.Atoi(common.Attr(t, "val")); err == nil && parsed > 0 {
					colSpan = parsed
				}
			case docxName(t.Name, docxNSW, "vMerge"):
				p.issues = append(p.issues, ir.Issue{Severity: "warning", Code: "t-warn-table-merge", Message: "DOCX vertical table merge is not preserved"})
			case docxName(t.Name, docxNSW, "p"):
				newBlocks, err := p.parseParagraph(dec, t)
				if err != nil {
					return ir.TableCell{}, err
				}
				blocks = append(blocks, newBlocks...)
			default:
				if err := common.SkipElement(dec, t); err != nil {
					return ir.TableCell{}, err
				}
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return ir.TableCell{Children: blocks, ColSpan: colSpan, RowSpan: 1}, nil
			}
		}
	}
}

func (p *documentParser) addDirect(role string) {
	for i := range p.report.DirectFormatting {
		if p.report.DirectFormatting[i].Role == role {
			p.report.DirectFormatting[i].Count++
			return
		}
	}
	p.report.DirectFormatting = append(p.report.DirectFormatting, parser.DirectFormattingStat{Role: role, Count: 1})
}

func groupLists(blocks []ir.Block) []ir.Block {
	var out []ir.Block
	for i := 0; i < len(blocks); {
		p, ok := blocks[i].(ir.Paragraph)
		if !ok || (p.Role != "__list_ordered" && p.Role != "__list_unordered") {
			out = append(out, blocks[i])
			i++
			continue
		}
		ordered := p.Role == "__list_ordered"
		var items []ir.ListItem
		for i < len(blocks) {
			p, ok := blocks[i].(ir.Paragraph)
			if !ok || (p.Role == "__list_ordered") != ordered || (p.Role != "__list_ordered" && p.Role != "__list_unordered") {
				break
			}
			items = append(items, ir.ListItem{Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: p.Inlines}}})
			i++
		}
		out = append(out, ir.List{Ordered: ordered, Items: items})
	}
	return out
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

func trimInlines(inlines []ir.Inline) []ir.Inline {
	for len(inlines) > 0 {
		text, ok := inlines[0].(ir.Text)
		if !ok {
			break
		}
		text.Value = strings.TrimLeft(text.Value, " \t\r\n")
		if text.Value == "" {
			inlines = inlines[1:]
			continue
		}
		inlines[0] = text
		break
	}
	for len(inlines) > 0 {
		text, ok := inlines[len(inlines)-1].(ir.Text)
		if !ok {
			break
		}
		text.Value = strings.TrimRight(text.Value, " \t\r\n")
		if text.Value == "" {
			inlines = inlines[:len(inlines)-1]
			continue
		}
		inlines[len(inlines)-1] = text
		break
	}
	return inlines
}

func headingLevel(name string) int {
	fields := strings.Fields(name)
	if len(fields) > 0 {
		if parsed, err := strconv.Atoi(fields[len(fields)-1]); err == nil && parsed >= 1 && parsed <= 6 {
			return parsed
		}
	}
	return 1
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

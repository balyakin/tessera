package ir

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func MarshalCanonical(doc *Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("marshal IR: document is nil")
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal IR: %w", err)
	}
	return append(data, '\n'), nil
}

func UnmarshalCanonical(data []byte) (*Document, error) {
	var doc Document
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("unmarshal IR: %w", err)
	}
	if doc.IRVersion != CurrentIRVersion {
		return nil, fmt.Errorf("unmarshal IR: unsupported ir_version %q", doc.IRVersion)
	}
	if issues := ValidateDocument(&doc, nil); hasError(issues) {
		return nil, fmt.Errorf("unmarshal IR: invalid document: %s", issues[0].Code)
	}
	return &doc, nil
}

func (d Document) MarshalJSON() ([]byte, error) {
	type documentJSON struct {
		IRVersion string        `json:"ir_version"`
		Meta      Metadata      `json:"meta"`
		Cover     *Image        `json:"cover,omitempty"`
		Body      []Block       `json:"body"`
		Footnotes []FootnoteDef `json:"footnotes"`
	}
	body := d.Body
	if body == nil {
		body = []Block{}
	}
	footnotes := d.Footnotes
	if footnotes == nil {
		footnotes = []FootnoteDef{}
	}
	return json.Marshal(documentJSON{
		IRVersion: d.IRVersion,
		Meta:      d.Meta,
		Cover:     d.Cover,
		Body:      body,
		Footnotes: footnotes,
	})
}

func (d *Document) UnmarshalJSON(data []byte) error {
	type documentJSON struct {
		IRVersion string            `json:"ir_version"`
		Meta      Metadata          `json:"meta"`
		Cover     *Image            `json:"cover,omitempty"`
		Body      []json.RawMessage `json:"body"`
		Footnotes []footnoteJSON    `json:"footnotes"`
	}
	var decoded documentJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	blocks, err := decodeBlocks(decoded.Body)
	if err != nil {
		return err
	}
	footnotes := make([]FootnoteDef, 0, len(decoded.Footnotes))
	for _, fn := range decoded.Footnotes {
		children, err := decodeBlocks(fn.Children)
		if err != nil {
			return err
		}
		footnotes = append(footnotes, FootnoteDef{ID: fn.ID, Children: children})
	}
	*d = Document{
		IRVersion: decoded.IRVersion,
		Meta:      decoded.Meta,
		Cover:     decoded.Cover,
		Body:      blocks,
		Footnotes: footnotes,
	}
	return nil
}

type footnoteJSON struct {
	ID       string            `json:"id"`
	Children []json.RawMessage `json:"children"`
}

func marshalBlock(typeName string, v any) ([]byte, error) {
	type envelope map[string]any
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var fields envelope
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil, err
	}
	out := envelope{"_type": typeName}
	for k, v := range fields {
		out[k] = v
	}
	return json.Marshal(out)
}

type headingAlias Heading
type paragraphAlias Paragraph
type verseAlias Verse
type letterAlias Letter
type epigraphAlias Epigraph
type blockQuoteAlias BlockQuote
type listAlias List
type figureAlias Figure
type tableAlias Table
type horizontalRuleAlias HorizontalRule
type pageBreakAlias PageBreak
type textAlias Text
type lineBreakAlias LineBreak
type styledAlias Styled
type linkAlias Link
type footnoteRefAlias FootnoteRef
type inlineImageAlias InlineImage

func (v Heading) MarshalJSON() ([]byte, error)   { return marshalBlock("heading", headingAlias(v)) }
func (v Paragraph) MarshalJSON() ([]byte, error) { return marshalBlock("paragraph", paragraphAlias(v)) }
func (v Verse) MarshalJSON() ([]byte, error)     { return marshalBlock("verse", verseAlias(v)) }
func (v Letter) MarshalJSON() ([]byte, error)    { return marshalBlock("letter", letterAlias(v)) }
func (v Epigraph) MarshalJSON() ([]byte, error)  { return marshalBlock("epigraph", epigraphAlias(v)) }
func (v BlockQuote) MarshalJSON() ([]byte, error) {
	return marshalBlock("blockquote", blockQuoteAlias(v))
}
func (v List) MarshalJSON() ([]byte, error)   { return marshalBlock("list", listAlias(v)) }
func (v Figure) MarshalJSON() ([]byte, error) { return marshalBlock("figure", figureAlias(v)) }
func (v Table) MarshalJSON() ([]byte, error)  { return marshalBlock("table", tableAlias(v)) }
func (v HorizontalRule) MarshalJSON() ([]byte, error) {
	return marshalBlock("horizontal_rule", horizontalRuleAlias(v))
}
func (v PageBreak) MarshalJSON() ([]byte, error) {
	return marshalBlock("page_break", pageBreakAlias(v))
}

func (v Text) MarshalJSON() ([]byte, error) { return marshalBlock("text", textAlias(v)) }
func (v LineBreak) MarshalJSON() ([]byte, error) {
	return marshalBlock("line_break", lineBreakAlias(v))
}
func (v Styled) MarshalJSON() ([]byte, error) { return marshalBlock("styled", styledAlias(v)) }
func (v Link) MarshalJSON() ([]byte, error)   { return marshalBlock("link", linkAlias(v)) }
func (v FootnoteRef) MarshalJSON() ([]byte, error) {
	return marshalBlock("footnote_ref", footnoteRefAlias(v))
}
func (v InlineImage) MarshalJSON() ([]byte, error) {
	return marshalBlock("inline_image", inlineImageAlias(v))
}

func decodeBlocks(raw []json.RawMessage) ([]Block, error) {
	blocks := make([]Block, 0, len(raw))
	for _, item := range raw {
		block, err := decodeBlock(item)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, nil
}

func decodeBlock(raw json.RawMessage) (Block, error) {
	var probe struct {
		Type string `json:"_type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case "heading":
		var v headingJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		inlines, err := decodeInlines(v.Inlines)
		return Heading{Level: v.Level, Inlines: inlines, ID: v.ID}, err
	case "paragraph":
		var v paragraphJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		inlines, err := decodeInlines(v.Inlines)
		return Paragraph{Role: v.Role, Inlines: inlines}, err
	case "verse":
		var v verseJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		stanzas := make([][]Line, 0, len(v.Stanzas))
		for _, stanza := range v.Stanzas {
			lines := make([]Line, 0, len(stanza))
			for _, rawLine := range stanza {
				inlines, err := decodeInlines(rawLine.Inlines)
				if err != nil {
					return nil, err
				}
				lines = append(lines, Line{Inlines: inlines})
			}
			stanzas = append(stanzas, lines)
		}
		source, err := decodeInlines(v.Source)
		return Verse{Stanzas: stanzas, Source: source}, err
	case "letter":
		var v letterJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		children, err := decodeBlocks(v.Children)
		return Letter{Children: children}, err
	case "epigraph":
		var v epigraphJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		children, err := decodeBlocks(v.Children)
		if err != nil {
			return nil, err
		}
		source, err := decodeInlines(v.Source)
		return Epigraph{Children: children, Source: source}, err
	case "blockquote":
		var v blockQuoteJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		children, err := decodeBlocks(v.Children)
		if err != nil {
			return nil, err
		}
		cite, err := decodeInlines(v.Cite)
		return BlockQuote{Children: children, Cite: cite}, err
	case "list":
		var v listJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		items := make([]ListItem, 0, len(v.Items))
		for _, item := range v.Items {
			children, err := decodeBlocks(item.Children)
			if err != nil {
				return nil, err
			}
			items = append(items, ListItem{Children: children})
		}
		return List{Ordered: v.Ordered, Items: items}, nil
	case "figure":
		var v figureJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		caption, err := decodeInlines(v.Caption)
		return Figure{Image: v.Image, Caption: caption}, err
	case "table":
		var v tableJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		rows := make([]TableRow, 0, len(v.Rows))
		for _, row := range v.Rows {
			cells := make([]TableCell, 0, len(row.Cells))
			for _, cell := range row.Cells {
				children, err := decodeBlocks(cell.Children)
				if err != nil {
					return nil, err
				}
				cells = append(cells, TableCell{Children: children, ColSpan: cell.ColSpan, RowSpan: cell.RowSpan})
			}
			rows = append(rows, TableRow{Header: row.Header, Cells: cells})
		}
		return Table{Rows: rows}, nil
	case "horizontal_rule":
		return HorizontalRule{}, nil
	case "page_break":
		return PageBreak{}, nil
	default:
		return nil, fmt.Errorf("unknown block _type %q", probe.Type)
	}
}

type letterJSON struct {
	Children []json.RawMessage `json:"children"`
}

type headingJSON struct {
	Level   int               `json:"level"`
	Inlines []json.RawMessage `json:"inlines"`
	ID      string            `json:"id"`
}

type paragraphJSON struct {
	Role    BlockRole         `json:"role"`
	Inlines []json.RawMessage `json:"inlines"`
}

type verseJSON struct {
	Stanzas [][]struct {
		Inlines []json.RawMessage `json:"inlines"`
	} `json:"stanzas"`
	Source []json.RawMessage `json:"source"`
}

type epigraphJSON struct {
	Children []json.RawMessage `json:"children"`
	Source   []json.RawMessage `json:"source"`
}

type blockQuoteJSON struct {
	Children []json.RawMessage `json:"children"`
	Cite     []json.RawMessage `json:"cite"`
}

type listJSON struct {
	Ordered bool `json:"ordered"`
	Items   []struct {
		Children []json.RawMessage `json:"children"`
	} `json:"items"`
}

type figureJSON struct {
	Image   Image             `json:"image"`
	Caption []json.RawMessage `json:"caption"`
}

type tableJSON struct {
	Rows []struct {
		Header bool `json:"header"`
		Cells  []struct {
			Children []json.RawMessage `json:"children"`
			ColSpan  int               `json:"col_span"`
			RowSpan  int               `json:"row_span"`
		} `json:"cells"`
	} `json:"rows"`
}

func decodeInto(raw json.RawMessage, dst any) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	delete(fields, "_type")
	clean, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	return json.Unmarshal(clean, dst)
}

func decodeInlines(raw []json.RawMessage) ([]Inline, error) {
	inlines := make([]Inline, 0, len(raw))
	for _, item := range raw {
		inline, err := decodeInline(item)
		if err != nil {
			return nil, err
		}
		inlines = append(inlines, inline)
	}
	return inlines, nil
}

func decodeInline(raw json.RawMessage) (Inline, error) {
	var probe struct {
		Type string `json:"_type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil, err
	}
	switch probe.Type {
	case "text":
		var v Text
		return v, decodeInto(raw, &v)
	case "line_break":
		return LineBreak{}, nil
	case "styled":
		var v styledJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		children, err := decodeInlines(v.Children)
		return Styled{Role: v.Role, Lang: v.Lang, Children: children}, err
	case "link":
		var v linkJSON
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		children, err := decodeInlines(v.Children)
		return Link{Href: v.Href, Children: children}, err
	case "footnote_ref":
		var v FootnoteRef
		return v, decodeInto(raw, &v)
	case "inline_image":
		var v InlineImage
		return v, decodeInto(raw, &v)
	default:
		return nil, fmt.Errorf("unknown inline _type %q", probe.Type)
	}
}

type styledJSON struct {
	Role     InlineRole        `json:"role"`
	Lang     string            `json:"lang"`
	Children []json.RawMessage `json:"children"`
}

type linkJSON struct {
	Href     string            `json:"href"`
	Children []json.RawMessage `json:"children"`
}

func hasError(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

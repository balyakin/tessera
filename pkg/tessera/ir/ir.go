package ir

const CurrentIRVersion = "1.0"

type Issue struct {
	Severity string            `json:"severity"`
	Code     string            `json:"code"`
	Message  string            `json:"message"`
	Context  map[string]string `json:"context,omitempty"`
}

type Document struct {
	IRVersion string
	Meta      Metadata
	Cover     *Image
	Body      []Block
	Footnotes []FootnoteDef
}

type Metadata struct {
	Title       string            `json:"title"`
	Subtitle    string            `json:"subtitle,omitempty"`
	Author      string            `json:"author,omitempty"`
	Language    string            `json:"language"`
	Identifier  string            `json:"identifier,omitempty"`
	Date        string            `json:"date,omitempty"`
	Publisher   string            `json:"publisher,omitempty"`
	Description string            `json:"description,omitempty"`
	Extra       map[string]string `json:"extra,omitempty"`
}

type Block interface{ isBlock() }

type Heading struct {
	Level   int      `json:"level"`
	Inlines []Inline `json:"inlines"`
	ID      string   `json:"id,omitempty"`
}

type Paragraph struct {
	Role    BlockRole `json:"role"`
	Inlines []Inline  `json:"inlines"`
}

type Verse struct {
	Stanzas [][]Line `json:"stanzas"`
	Source  []Inline `json:"source,omitempty"`
}

type Line struct {
	Inlines []Inline `json:"inlines"`
}

type Letter struct {
	Children []Block `json:"children"`
}

type Epigraph struct {
	Children []Block  `json:"children"`
	Source   []Inline `json:"source,omitempty"`
}

type BlockQuote struct {
	Children []Block  `json:"children"`
	Cite     []Inline `json:"cite,omitempty"`
}

type List struct {
	Ordered bool       `json:"ordered"`
	Items   []ListItem `json:"items"`
}

type ListItem struct {
	Children []Block `json:"children"`
}

type Figure struct {
	Image   Image    `json:"image"`
	Caption []Inline `json:"caption,omitempty"`
}

type Table struct {
	Rows []TableRow `json:"rows"`
}

type TableRow struct {
	Header bool        `json:"header"`
	Cells  []TableCell `json:"cells"`
}

type TableCell struct {
	Children []Block `json:"children"`
	ColSpan  int     `json:"col_span,omitempty"`
	RowSpan  int     `json:"row_span,omitempty"`
}

type HorizontalRule struct{}

type PageBreak struct{}

type FootnoteDef struct {
	ID       string  `json:"id"`
	Children []Block `json:"children"`
}

func (Heading) isBlock()        {}
func (Paragraph) isBlock()      {}
func (Verse) isBlock()          {}
func (Letter) isBlock()         {}
func (Epigraph) isBlock()       {}
func (BlockQuote) isBlock()     {}
func (List) isBlock()           {}
func (Figure) isBlock()         {}
func (Table) isBlock()          {}
func (HorizontalRule) isBlock() {}
func (PageBreak) isBlock()      {}

type Inline interface{ isInline() }

type Text struct {
	Value string `json:"value"`
}

type LineBreak struct{}

type Styled struct {
	Role     InlineRole `json:"role"`
	Lang     string     `json:"lang,omitempty"`
	Children []Inline   `json:"children"`
}

type Link struct {
	Href     string   `json:"href"`
	Children []Inline `json:"children"`
}

type FootnoteRef struct {
	ID string `json:"id"`
}

type InlineImage struct {
	Image Image `json:"image"`
}

func (Text) isInline()        {}
func (LineBreak) isInline()   {}
func (Styled) isInline()      {}
func (Link) isInline()        {}
func (FootnoteRef) isInline() {}
func (InlineImage) isInline() {}

type Image struct {
	Name      string `json:"name"`
	Data      []byte `json:"data,omitempty"`
	MediaType string `json:"media_type"`
	Alt       string `json:"alt,omitempty"`
}

type BlockRole string

const (
	RoleBody       BlockRole = "body"
	RoleTitle      BlockRole = "title"
	RoleSubtitle   BlockRole = "subtitle"
	RoleDedication BlockRole = "dedication"
	RoleColophon   BlockRole = "colophon"
	RoleGlossary   BlockRole = "glossary"
	RoleHalftitle  BlockRole = "halftitle"
)

type InlineRole string

const (
	Emphasis  InlineRole = "emphasis"
	Strong    InlineRole = "strong"
	Foreign   InlineRole = "foreign"
	Thought   InlineRole = "thought"
	Prayer    InlineRole = "prayer"
	WorkTitle InlineRole = "work-title"
)

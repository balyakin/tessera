package ir

type Visitor interface {
	EnterBlock(Block) bool
	LeaveBlock(Block)
	EnterInline(Inline) bool
	LeaveInline(Inline)
}

type visitorFuncs struct {
	enterBlock  func(Block) bool
	leaveBlock  func(Block)
	enterInline func(Inline) bool
	leaveInline func(Inline)
}

func (v visitorFuncs) EnterBlock(block Block) bool {
	if v.enterBlock == nil {
		return true
	}
	return v.enterBlock(block)
}

func (v visitorFuncs) LeaveBlock(block Block) {
	if v.leaveBlock != nil {
		v.leaveBlock(block)
	}
}

func (v visitorFuncs) EnterInline(inline Inline) bool {
	if v.enterInline == nil {
		return true
	}
	return v.enterInline(inline)
}

func (v visitorFuncs) LeaveInline(inline Inline) {
	if v.leaveInline != nil {
		v.leaveInline(inline)
	}
}

func Walk(doc *Document, visitor Visitor) {
	if doc == nil || visitor == nil {
		return
	}
	for _, block := range doc.Body {
		walkBlock(block, visitor)
	}
	for _, footnote := range doc.Footnotes {
		for _, block := range footnote.Children {
			walkBlock(block, visitor)
		}
	}
}

func walkBlock(block Block, visitor Visitor) {
	if block == nil {
		return
	}
	if !visitor.EnterBlock(block) {
		visitor.LeaveBlock(block)
		return
	}
	switch v := block.(type) {
	case Heading:
		walkInlines(v.Inlines, visitor)
	case Paragraph:
		walkInlines(v.Inlines, visitor)
	case Verse:
		for _, stanza := range v.Stanzas {
			for _, line := range stanza {
				walkInlines(line.Inlines, visitor)
			}
		}
		walkInlines(v.Source, visitor)
	case Letter:
		for _, child := range v.Children {
			walkBlock(child, visitor)
		}
	case Epigraph:
		for _, child := range v.Children {
			walkBlock(child, visitor)
		}
		walkInlines(v.Source, visitor)
	case BlockQuote:
		for _, child := range v.Children {
			walkBlock(child, visitor)
		}
		walkInlines(v.Cite, visitor)
	case List:
		for _, item := range v.Items {
			for _, child := range item.Children {
				walkBlock(child, visitor)
			}
		}
	case Figure:
		walkInlines(v.Caption, visitor)
	case Table:
		for _, row := range v.Rows {
			for _, cell := range row.Cells {
				for _, child := range cell.Children {
					walkBlock(child, visitor)
				}
			}
		}
	}
	visitor.LeaveBlock(block)
}

func walkInlines(inlines []Inline, visitor Visitor) {
	for _, inline := range inlines {
		if inline == nil {
			continue
		}
		if !visitor.EnterInline(inline) {
			visitor.LeaveInline(inline)
			continue
		}
		switch v := inline.(type) {
		case Styled:
			walkInlines(v.Children, visitor)
		case Link:
			walkInlines(v.Children, visitor)
		}
		visitor.LeaveInline(inline)
	}
}

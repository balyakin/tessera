package latex

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type FileBackend struct {
	Kind backend.OutputKind
}

func (b FileBackend) Render(doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) ([]backend.Artifact, []ir.Issue, error) {
	result, err := Render(doc, cfg, opts)
	if err != nil {
		return nil, nil, err
	}
	name := opts.Basename
	if name == "" {
		name = "book"
	}
	path := filepath.Join(opts.OutputDir, name+".tex")
	if err := os.WriteFile(path, []byte(result.TexSource), 0o644); err != nil {
		return nil, nil, fmt.Errorf("write LaTeX: %w", err)
	}
	return []backend.Artifact{{Kind: backend.OutputTEX, Path: path}}, result.Issues, nil
}

func Render(doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) (backend.LatexResult, error) {
	if doc == nil {
		return backend.LatexResult{}, fmt.Errorf("render LaTeX: document is nil")
	}
	state := &renderState{
		doc:       doc,
		cfg:       cfg,
		imageRefs: map[string]string{},
		footnotes: map[string]ir.FootnoteDef{},
		seenNotes: map[string]bool{},
	}
	for _, def := range doc.Footnotes {
		state.footnotes[def.ID] = def
	}
	preamble, err := renderPreamble(doc, cfg)
	if err != nil {
		return backend.LatexResult{}, err
	}
	var b strings.Builder
	b.WriteString(preamble)
	if doc.Cover != nil {
		name := state.imageName(*doc.Cover)
		b.WriteString("\\thispagestyle{empty}\\begin{center}\n")
		fmt.Fprintf(&b, "\\includegraphics[width=\\paperwidth,height=\\paperheight,keepaspectratio]{%s}\n", EscapeText(name))
		b.WriteString("\\end{center}\\clearpage\n")
	}
	fmt.Fprintf(&b, "\\title{%s}\n", EscapeText(doc.Meta.Title))
	if doc.Meta.Author != "" {
		fmt.Fprintf(&b, "\\author{%s}\n", EscapeText(doc.Meta.Author))
	}
	b.WriteString("\\maketitle\n")
	if cfg.TOC.IncludeInPDF {
		fmt.Fprintf(&b, "\\setcounter{tocdepth}{%d}\n", cfg.TOC.Depth)
		b.WriteString("\\tableofcontents\\clearpage\n")
	}
	for _, block := range doc.Body {
		state.renderBlock(&b, block)
	}
	b.WriteString("\\end{document}\n")
	return backend.LatexResult{TexSource: b.String(), Images: state.images, Issues: state.issues}, nil
}

type renderState struct {
	doc       *ir.Document
	cfg       *config.Config
	imageRefs map[string]string
	images    []backend.RenderedImage
	issues    []ir.Issue
	footnotes map[string]ir.FootnoteDef
	seenNotes map[string]bool
}

func (s *renderState) renderBlock(b *strings.Builder, block ir.Block) {
	switch v := block.(type) {
	case ir.Heading:
		cmd := []string{"chapter", "section", "subsection", "subsubsection", "paragraph", "subparagraph"}[clamp(v.Level, 1, 6)-1]
		fmt.Fprintf(b, "\\%s{%s}\n\n", cmd, s.renderInlines(v.Inlines, false))
	case ir.Paragraph:
		switch v.Role {
		case ir.RoleDedication:
			b.WriteString("\\begin{dedication}\n")
			b.WriteString(s.renderInlines(v.Inlines, false))
			b.WriteString("\n\\end{dedication}\n\n")
		default:
			b.WriteString(s.renderInlines(v.Inlines, false))
			b.WriteString("\n\n")
		}
	case ir.Verse:
		b.WriteString("\\begin{verse}\n")
		for _, stanza := range v.Stanzas {
			for _, line := range stanza {
				b.WriteString(s.renderInlines(line.Inlines, true))
				b.WriteString("\\\\\n")
			}
			b.WriteString("\n")
		}
		b.WriteString("\\end{verse}\n\n")
	case ir.Letter:
		b.WriteString("\\begin{quote}\n")
		s.renderChildren(b, v.Children)
		b.WriteString("\\end{quote}\n\n")
	case ir.Epigraph:
		body := s.renderArgumentBlocks(v.Children)
		source := s.renderInlines(v.Source, false)
		if source != "" {
			source = "\\attrib{" + source + "}"
		}
		fmt.Fprintf(b, "\\epigraph{%s}{%s}\n\n", body, source)
	case ir.BlockQuote:
		b.WriteString("\\begin{quotation}\n")
		s.renderChildren(b, v.Children)
		b.WriteString("\\end{quotation}\n\n")
	case ir.List:
		env := "itemize"
		if v.Ordered {
			env = "enumerate"
		}
		fmt.Fprintf(b, "\\begin{%s}\n", env)
		for _, item := range v.Items {
			b.WriteString("\\item ")
			s.renderChildren(b, item.Children)
		}
		fmt.Fprintf(b, "\\end{%s}\n\n", env)
	case ir.Figure:
		name := s.imageName(v.Image)
		b.WriteString("\\begin{figure}[htbp]\n\\centering\n")
		fmt.Fprintf(b, "\\includegraphics[width=\\linewidth,keepaspectratio]{%s}\n", EscapeText(name))
		if len(v.Caption) > 0 {
			fmt.Fprintf(b, "\\caption{%s}\n", s.renderInlines(v.Caption, false))
		}
		b.WriteString("\\end{figure}\n\n")
	case ir.Table:
		s.renderTable(b, v)
	case ir.HorizontalRule:
		b.WriteString("\\noindent\\rule{\\linewidth}{0.4pt}\n\n")
	case ir.PageBreak:
		b.WriteString("\\clearpage\n")
	}
}

func (s *renderState) renderChildren(b *strings.Builder, children []ir.Block) {
	for _, child := range children {
		s.renderBlock(b, child)
	}
}

func (s *renderState) renderArgumentBlocks(blocks []ir.Block) string {
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		parts = append(parts, strings.TrimSpace(s.renderArgumentBlock(block)))
	}
	return strings.Join(nonEmpty(parts), `\par{} `)
}

func (s *renderState) renderArgumentBlock(block ir.Block) string {
	switch v := block.(type) {
	case ir.Heading:
		return s.renderInlines(v.Inlines, false)
	case ir.Paragraph:
		return s.renderInlines(v.Inlines, false)
	case ir.Verse:
		return s.renderVerseArgument(v)
	case ir.Letter:
		return s.renderArgumentBlocks(v.Children)
	case ir.Epigraph:
		return s.renderEpigraphArgument(v)
	case ir.BlockQuote:
		return s.renderArgumentBlocks(v.Children)
	case ir.List:
		return s.renderListArgument(v)
	case ir.Figure:
		return EscapeText(v.Image.Alt)
	case ir.Table:
		return EscapeText(blockText([]ir.Block{v}))
	default:
		return ""
	}
}

func (s *renderState) renderVerseArgument(verse ir.Verse) string {
	stanzas := make([]string, 0, len(verse.Stanzas))
	for _, stanza := range verse.Stanzas {
		lines := make([]string, 0, len(stanza))
		for _, line := range stanza {
			lines = append(lines, s.renderInlines(line.Inlines, true))
		}
		stanzas = append(stanzas, strings.Join(nonEmpty(lines), `\\ `))
	}
	return strings.Join(nonEmpty(stanzas), `\par{} `)
}

func (s *renderState) renderEpigraphArgument(epigraph ir.Epigraph) string {
	body := s.renderArgumentBlocks(epigraph.Children)
	source := s.renderInlines(epigraph.Source, false)
	if source == "" {
		return body
	}
	if body == "" {
		return source
	}
	return body + `\par{} ` + source
}

func (s *renderState) renderListArgument(list ir.List) string {
	items := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		body := s.renderArgumentBlocks(item.Children)
		if body != "" {
			items = append(items, `\textbullet{} `+body)
		}
	}
	return strings.Join(nonEmpty(items), `\par{} `)
}

func (s *renderState) renderTable(b *strings.Builder, table ir.Table) {
	cols := 1
	for _, row := range table.Rows {
		if len(row.Cells) > cols {
			cols = len(row.Cells)
		}
	}
	b.WriteString("\\begin{tabular}{")
	for i := 0; i < cols; i++ {
		b.WriteString("l")
	}
	b.WriteString("}\n")
	for _, row := range table.Rows {
		for i, cell := range row.Cells {
			if i > 0 {
				b.WriteString(" & ")
			}
			b.WriteString(s.renderTableCell(cell.Children))
		}
		b.WriteString(" \\\\\n")
	}
	b.WriteString("\\end{tabular}\n\n")
}

func (s *renderState) renderTableCell(blocks []ir.Block) string {
	var parts []string
	for _, block := range blocks {
		switch v := block.(type) {
		case ir.Heading:
			parts = append(parts, s.renderInlines(v.Inlines, false))
		case ir.Paragraph:
			parts = append(parts, s.renderInlines(v.Inlines, false))
		case ir.Verse:
			var lines []string
			for _, stanza := range v.Stanzas {
				for _, line := range stanza {
					lines = append(lines, s.renderInlines(line.Inlines, true))
				}
			}
			if len(lines) > 0 {
				parts = append(parts, strings.Join(lines, `\\`))
			}
		case ir.Letter:
			parts = append(parts, s.renderTableCell(v.Children))
		case ir.Epigraph:
			value := s.renderTableCell(v.Children)
			if len(v.Source) > 0 {
				value += " " + s.renderInlines(v.Source, false)
			}
			parts = append(parts, strings.TrimSpace(value))
		case ir.BlockQuote:
			parts = append(parts, s.renderTableCell(v.Children))
		case ir.List:
			var items []string
			for _, item := range v.Items {
				items = append(items, s.renderTableCell(item.Children))
			}
			parts = append(parts, strings.Join(items, "; "))
		case ir.Figure:
			if len(v.Caption) > 0 {
				parts = append(parts, s.renderInlines(v.Caption, false))
			}
		case ir.Table:
			parts = append(parts, EscapeText(blockText([]ir.Block{v})))
		}
	}
	return strings.Join(nonEmpty(parts), `\newline{} `)
}

func nonEmpty(values []string) []string {
	out := values[:0]
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func (s *renderState) renderInlines(inlines []ir.Inline, verse bool) string {
	var b strings.Builder
	for _, inline := range inlines {
		switch v := inline.(type) {
		case ir.Text:
			b.WriteString(EscapeText(v.Value))
		case ir.LineBreak:
			if verse {
				b.WriteString("\\\\")
			} else {
				b.WriteString("\\newline{}")
			}
		case ir.Styled:
			body := s.renderInlines(v.Children, verse)
			switch v.Role {
			case ir.Emphasis:
				fmt.Fprintf(&b, "\\semEmph{%s}", body)
			case ir.Strong:
				fmt.Fprintf(&b, "\\semStrong{%s}", body)
			case ir.Thought:
				fmt.Fprintf(&b, "\\semThought{%s}", body)
			case ir.Prayer:
				fmt.Fprintf(&b, "\\semPrayer{%s}", body)
			case ir.WorkTitle:
				fmt.Fprintf(&b, "\\semWorkTitle{%s}", body)
			case ir.Foreign:
				cmd := s.foreignCommand(v.Lang)
				if cmd == "" {
					s.issues = append(s.issues, ir.Issue{Severity: "warning", Code: "t-warn-foreign-latex", Message: "foreign language has no LaTeX command", Context: map[string]string{"language": v.Lang}})
					b.WriteString(body)
				} else {
					fmt.Fprintf(&b, "\\%s{%s}", cmd, body)
				}
			default:
				b.WriteString(body)
			}
		case ir.Link:
			fmt.Fprintf(&b, "\\href{%s}{%s}", EscapeURL(v.Href), s.renderInlines(v.Children, verse))
		case ir.FootnoteRef:
			def := s.footnotes[v.ID]
			if s.seenNotes[v.ID] {
				s.issues = append(s.issues, ir.Issue{Severity: "warning", Code: "t-warn-footnote-repeated", Message: "repeated footnote reference", Context: map[string]string{"id": v.ID}})
			}
			s.seenNotes[v.ID] = true
			fmt.Fprintf(&b, "\\footnote{%s}", s.renderArgumentBlocks(def.Children))
		case ir.InlineImage:
			name := s.imageName(v.Image)
			fmt.Fprintf(&b, "\\includegraphics[height=1em]{%s}", EscapeText(name))
		}
	}
	return b.String()
}

func (s *renderState) foreignCommand(lang string) string {
	name := s.cfg.Languages[lang]
	if name == "" {
		return ""
	}
	if name == "latin" {
		return "textlatin"
	}
	return "text" + strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return r
		}
		return -1
	}, name)
}

func (s *renderState) imageName(image ir.Image) string {
	key := backend.ImageKey(image)
	if existing := s.imageRefs[key]; existing != "" {
		return existing
	}
	ext := strings.ToLower(filepath.Ext(image.Name))
	if ext == "" {
		ext = ".bin"
	}
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		s.issues = append(s.issues, ir.Issue{Severity: "warning", Code: "t-warn-image-pdf-unsupported", Message: "image format unsupported by PDF backend", Context: map[string]string{"image": image.Name}})
	}
	name := fmt.Sprintf("image-%04d%s", len(s.images)+1, ext)
	s.imageRefs[key] = name
	s.images = append(s.images, backend.RenderedImage{Name: name, Data: image.Data})
	return name
}

func blockText(blocks []ir.Block) string {
	var out strings.Builder
	for _, block := range blocks {
		switch v := block.(type) {
		case ir.Heading:
			out.WriteString(inlineText(v.Inlines))
		case ir.Paragraph:
			out.WriteString(inlineText(v.Inlines))
		case ir.Verse:
			for _, stanza := range v.Stanzas {
				for _, line := range stanza {
					out.WriteString(inlineText(line.Inlines))
					out.WriteByte(' ')
				}
			}
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
	return out.String()
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

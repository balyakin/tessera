package latex

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func renderPreamble(doc *ir.Document, cfg *config.Config) (string, error) {
	var b strings.Builder
	options := cfg.Latex.DocumentClassOptions
	if options == "" {
		options = "11pt,oneside"
	}
	mainFont := cfg.Latex.MainFont
	if mainFont == "" {
		mainFont = "EB Garamond"
	}
	lang := cfg.Languages[doc.Meta.Language]
	if lang == "" {
		lang = "english"
	}
	fmt.Fprintf(&b, "\\documentclass[%s]{memoir}\n", EscapeText(options))
	b.WriteString("\\usepackage{fontspec}\n")
	b.WriteString("\\usepackage{polyglossia}\n")
	b.WriteString("\\usepackage{microtype}\n")
	b.WriteString("\\usepackage{graphicx}\n")
	b.WriteString("\\usepackage{csquotes}\n")
	b.WriteString("\\usepackage{xcolor}\n")
	b.WriteString("\\usepackage{unicode-math}\n")
	b.WriteString("\\usepackage[hidelinks]{hyperref}\n")
	fmt.Fprintf(&b, "\\setmainlanguage{%s}\n", lang)
	otherLangs := make([]string, 0, len(cfg.Languages))
	for code, name := range cfg.Languages {
		if code != doc.Meta.Language && name != "" && name != lang {
			otherLangs = append(otherLangs, name)
		}
	}
	sort.Strings(otherLangs)
	seenLang := map[string]bool{}
	for _, name := range otherLangs {
		if seenLang[name] {
			continue
		}
		seenLang[name] = true
		fmt.Fprintf(&b, "\\setotherlanguage{%s}\n", name)
	}
	fmt.Fprintf(&b, "\\setmainfont{%s}\n", EscapeText(mainFont))
	fmt.Fprintf(&b, "\\hypersetup{pdftitle={%s},pdfauthor={%s},pdfcreator={Tessera}", EscapeText(doc.Meta.Title), EscapeText(doc.Meta.Author))
	if doc.Meta.Description != "" {
		fmt.Fprintf(&b, ",pdfsubject={%s}", EscapeText(doc.Meta.Description))
	}
	if doc.Meta.Extra["keywords"] != "" {
		fmt.Fprintf(&b, ",pdfkeywords={%s}", EscapeText(doc.Meta.Extra["keywords"]))
	}
	b.WriteString("}\n")
	b.WriteString("\\newcommand{\\semEmph}[1]{\\emph{#1}}\n")
	b.WriteString("\\newcommand{\\semStrong}[1]{\\textbf{#1}}\n")
	b.WriteString("\\newcommand{\\semThought}[1]{\\emph{#1}}\n")
	b.WriteString("\\newcommand{\\semPrayer}[1]{\\emph{#1}}\n")
	b.WriteString("\\newcommand{\\semWorkTitle}[1]{\\emph{#1}}\n")
	b.WriteString("\\newcommand{\\attrib}[1]{\\par\\hfill\\emph{#1}}\n")
	for _, line := range cfg.Latex.ExtraPreamble {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range cfg.Latex.ExtraMacros {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if cfg.Latex.PreambleFile != "" {
		data, err := os.ReadFile(cfg.Latex.PreambleFile)
		if err != nil {
			return "", fmt.Errorf("read LaTeX preamble file: %w", err)
		}
		b.Write(data)
		if len(data) == 0 || data[len(data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	b.WriteString("\\begin{document}\n")
	return b.String(), nil
}

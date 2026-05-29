package ir

import (
	"regexp"
)

var bcp47Like = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

func ValidateDocument(doc *Document, languages map[string]string) []Issue {
	var issues []Issue
	if doc == nil {
		return []Issue{{Severity: "error", Code: "t-err-empty-body", Message: "document is nil"}}
	}
	if doc.IRVersion == "" || doc.IRVersion != CurrentIRVersion {
		issues = append(issues, Issue{Severity: "error", Code: "t-err-ir-version", Message: "unsupported or empty IR version"})
	}
	if len(doc.Body) == 0 {
		issues = append(issues, Issue{Severity: "error", Code: "t-err-empty-body", Message: "document body is empty"})
	}
	if doc.Meta.Title == "" {
		issues = append(issues, Issue{Severity: "error", Code: "t-err-title-empty", Message: "document title is empty"})
	}
	if doc.Meta.Language == "" {
		issues = append(issues, Issue{Severity: "warning", Code: "t-warn-lang-unknown", Message: "document language is empty"})
	} else if !bcp47Like.MatchString(doc.Meta.Language) {
		issues = append(issues, Issue{Severity: "warning", Code: "t-warn-lang-unknown", Message: "document language is not BCP-47-like", Context: map[string]string{"language": doc.Meta.Language}})
	} else if languages != nil {
		if _, ok := languages[doc.Meta.Language]; !ok {
			issues = append(issues, Issue{Severity: "warning", Code: "t-warn-lang-unknown", Message: "document language is not configured", Context: map[string]string{"language": doc.Meta.Language}})
		}
	}
	if doc.Cover != nil && doc.Cover.Alt == "" {
		issues = append(issues, Issue{Severity: "warning", Code: "t-warn-cover-alt", Message: "cover image has empty alt text"})
	}

	footnotes := map[string]bool{}
	for _, def := range doc.Footnotes {
		if footnotes[def.ID] {
			issues = append(issues, Issue{Severity: "error", Code: "t-err-footnote-duplicate", Message: "duplicate footnote definition", Context: map[string]string{"id": def.ID}})
		}
		footnotes[def.ID] = true
	}
	refs := map[string]bool{}
	Walk(doc, visitorFuncs{
		enterBlock: func(block Block) bool {
			if h, ok := block.(Heading); ok && (h.Level < 1 || h.Level > 6) {
				issues = append(issues, Issue{Severity: "error", Code: "t-err-heading-level", Message: "heading level is outside 1..6"})
			}
			if fig, ok := block.(Figure); ok && fig.Image.Alt == "" {
				issues = append(issues, Issue{Severity: "warning", Code: "t-warn-alt", Message: "image alt text is empty", Context: map[string]string{"image": fig.Image.Name}})
			}
			return true
		},
		enterInline: func(inline Inline) bool {
			switch v := inline.(type) {
			case Styled:
				if v.Role == Foreign && v.Lang == "" {
					issues = append(issues, Issue{Severity: "warning", Code: "t-warn-foreign-nolang", Message: "foreign text has no language"})
				}
			case InlineImage:
				if v.Image.Alt == "" {
					issues = append(issues, Issue{Severity: "warning", Code: "t-warn-alt", Message: "image alt text is empty", Context: map[string]string{"image": v.Image.Name}})
				}
			case FootnoteRef:
				refs[v.ID] = true
			}
			return true
		},
	})
	for id := range refs {
		if !footnotes[id] {
			issues = append(issues, Issue{Severity: "error", Code: "t-err-footnote-missing", Message: "footnote reference has no matching definition", Context: map[string]string{"id": id}})
		}
	}
	return issues
}

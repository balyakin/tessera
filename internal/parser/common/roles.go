package common

import (
	"fmt"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func ParagraphRole(mapping config.StyleMapping) ir.BlockRole {
	switch mapping.Role {
	case "title":
		return ir.RoleTitle
	case "subtitle":
		return ir.RoleSubtitle
	case "dedication":
		return ir.RoleDedication
	case "colophon":
		return ir.RoleColophon
	case "glossary":
		return ir.RoleGlossary
	case "halftitle":
		return ir.RoleHalftitle
	default:
		return ir.RoleBody
	}
}

func InlineRole(mapping config.StyleMapping) ir.InlineRole {
	switch mapping.Role {
	case "strong":
		return ir.Strong
	case "foreign":
		return ir.Foreign
	case "thought":
		return ir.Thought
	case "prayer":
		return ir.Prayer
	case "work-title":
		return ir.WorkTitle
	default:
		return ir.Emphasis
	}
}

func StyleUsage(name, family string, resolved ResolvedStyle, defaultRole string) parser.StyleUsage {
	status := resolved.Status
	if status == "" {
		status = "mapped"
	}
	role := resolved.Role
	if role == "" {
		role = defaultRole
	}
	suggestionRole := "body"
	if family == "character" {
		suggestionRole = "emphasis"
	}
	suggestion := ""
	if status == "unknown" {
		suggestion = fmt.Sprintf("%q = { role = %q }", name, suggestionRole)
	}
	return parser.StyleUsage{
		Name:          strings.TrimSpace(name),
		Family:        family,
		Status:        status,
		Role:          role,
		MatchedName:   resolved.MatchedName,
		MatchKind:     resolved.MatchKind,
		SuggestedTOML: suggestion,
	}
}

func UnknownStyleIssue(family, name string) ir.Issue {
	code := "t-warn-unknown-pstyle"
	if family == "character" {
		code = "t-warn-unknown-cstyle"
	}
	return ir.Issue{
		Severity: "warning",
		Code:     code,
		Message:  fmt.Sprintf("unknown %s style %q", family, name),
		Context:  map[string]string{"style": name, "family": family},
	}
}

func AddStyleUsage(usages *[]parser.StyleUsage, usage parser.StyleUsage) {
	for i := range *usages {
		if (*usages)[i].Name == usage.Name && (*usages)[i].Family == usage.Family {
			if (*usages)[i].Status == "unknown" && usage.Status != "unknown" {
				(*usages)[i] = usage
			}
			return
		}
	}
	*usages = append(*usages, usage)
}

const CaptionBlockRole ir.BlockRole = "__caption"

func IsCaptionStyle(names ...string) bool {
	for _, name := range names {
		switch strings.TrimSpace(name) {
		case "Caption", "Illustration", "Подпись", "Иллюстрация":
			return true
		}
	}
	return false
}

func CaptionStyleUsage(name string) parser.StyleUsage {
	return parser.StyleUsage{
		Name:      strings.TrimSpace(name),
		Family:    "paragraph",
		Status:    "ignored",
		Role:      "caption",
		MatchKind: "unknown",
	}
}

func AttachCaptions(blocks []ir.Block) []ir.Block {
	out := make([]ir.Block, 0, len(blocks))
	for _, block := range blocks {
		p, ok := block.(ir.Paragraph)
		if !ok || p.Role != CaptionBlockRole {
			out = append(out, block)
			continue
		}
		if len(out) == 0 {
			out = append(out, ir.Paragraph{Role: ir.RoleBody, Inlines: p.Inlines})
			continue
		}
		fig, ok := out[len(out)-1].(ir.Figure)
		if !ok || len(fig.Caption) > 0 {
			out = append(out, ir.Paragraph{Role: ir.RoleBody, Inlines: p.Inlines})
			continue
		}
		fig.Caption = p.Inlines
		out[len(out)-1] = fig
	}
	return out
}

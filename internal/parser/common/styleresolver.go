package common

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type StyleDef struct {
	ID          string
	DisplayName string
	ParentID    string
}

type ResolvedStyle struct {
	Mapping     config.StyleMapping
	Status      string
	Role        string
	MatchedName string
	MatchKind   string
	Issues      []ir.Issue
}

type StyleResolver struct {
	family     string
	mappings   map[string]config.StyleMapping
	defs       map[string]StyleDef
	normalized map[string][]string
	fallback   bool
}

func NewStyleResolver(family string, mappings map[string]config.StyleMapping, defs map[string]StyleDef, fallback bool) *StyleResolver {
	normalized := map[string][]string{}
	for name := range mappings {
		n := NormalizeStyleName(name)
		normalized[n] = append(normalized[n], name)
	}
	return &StyleResolver{
		family:     family,
		mappings:   mappings,
		defs:       defs,
		normalized: normalized,
		fallback:   fallback,
	}
}

func (r *StyleResolver) Resolve(styleID, displayName string, strict bool) ResolvedStyle {
	candidates, chainIssues := r.candidateChain(styleID, displayName)
	for i, candidate := range candidates {
		name := strings.TrimSpace(candidate)
		if mapping, ok := r.mappings[name]; ok {
			kind := "exact"
			status := "mapped"
			if i > 0 {
				kind = "inherited"
				status = "inherited"
			}
			return ResolvedStyle{Mapping: mapping, Status: status, Role: mapping.Role, MatchedName: name, MatchKind: kind, Issues: chainIssues}
		}
	}
	if r.fallback && !strict {
		var ambiguousIssues []ir.Issue
		for _, candidate := range candidates {
			name := strings.TrimSpace(candidate)
			n := NormalizeStyleName(name)
			matches := r.normalized[n]
			if len(matches) == 1 {
				mapping := r.mappings[matches[0]]
				return ResolvedStyle{
					Mapping:     mapping,
					Status:      "mapped",
					Role:        mapping.Role,
					MatchedName: matches[0],
					MatchKind:   "normalized",
					Issues: append(chainIssues, ir.Issue{
						Severity: "warning",
						Code:     "t-warn-style-fuzzy",
						Message:  fmt.Sprintf("style %q matched %q by normalized fallback", name, matches[0]),
						Context:  map[string]string{"style": name, "matched": matches[0], "family": r.family},
					}),
				}
			}
			if len(matches) > 1 {
				ambiguousIssues = append(ambiguousIssues, ir.Issue{
					Severity: "warning",
					Code:     "t-warn-style-fuzzy-ambiguous",
					Message:  fmt.Sprintf("style %q has ambiguous normalized matches", name),
					Context:  map[string]string{"style": name, "family": r.family},
				})
			}
		}
		if len(ambiguousIssues) > 0 {
			return ResolvedStyle{Status: "unknown", MatchKind: "unknown", Issues: append(chainIssues, ambiguousIssues...)}
		}
	}
	return ResolvedStyle{Status: "unknown", MatchKind: "unknown", Issues: chainIssues}
}

func (r *StyleResolver) candidateChain(styleID, displayName string) ([]string, []ir.Issue) {
	var out []string
	var issues []ir.Issue
	seen := map[string]bool{}
	cycleReported := false
	if displayName != "" {
		out = append(out, displayName)
	}
	if styleID != "" && styleID != displayName {
		out = append(out, styleID)
	}
	current := styleID
	for hop := 0; hop < 32 && current != ""; hop++ {
		if seen[current] {
			issues = append(issues, ir.Issue{Severity: "warning", Code: "t-warn-style-cycle", Message: "style parent chain contains a cycle", Context: map[string]string{"style": current, "family": r.family}})
			cycleReported = true
			break
		}
		seen[current] = true
		def, ok := r.defs[current]
		if !ok {
			break
		}
		if def.DisplayName != "" && def.DisplayName != displayName {
			out = append(out, def.DisplayName)
		}
		if def.ParentID == "" {
			break
		}
		parent := r.defs[def.ParentID]
		if parent.DisplayName != "" {
			out = append(out, parent.DisplayName)
		}
		if def.ParentID != parent.DisplayName {
			out = append(out, def.ParentID)
		}
		current = def.ParentID
	}
	if current != "" && !cycleReported {
		if def, ok := r.defs[current]; ok && def.ParentID != "" {
			issues = append(issues, ir.Issue{Severity: "warning", Code: "t-warn-style-cycle", Message: "style parent chain exceeded 32 hops", Context: map[string]string{"style": current, "family": r.family}})
		}
	}
	return dedupe(out), issues
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func NormalizeStyleName(s string) string {
	var b strings.Builder
	var prev rune
	for _, r := range strings.TrimSpace(s) {
		if prev != 0 && unicode.IsLower(prev) && unicode.IsUpper(r) {
			b.WriteRune(' ')
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r) || r == '_' || r == '-':
			b.WriteRune(' ')
		}
		prev = r
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

package common

import (
	"testing"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func TestStyleResolverExactInheritedAndNormalized(t *testing.T) {
	mappings := map[string]config.StyleMapping{
		"Poem":      {Role: "verse"},
		"Body Text": {Role: "body"},
	}
	defs := map[string]StyleDef{
		"Child": {ID: "Child", DisplayName: "Child", ParentID: "Base"},
		"Base":  {ID: "Base", DisplayName: "Body Text"},
	}
	resolver := NewStyleResolver("paragraph", mappings, defs, true)
	if got := resolver.Resolve("Poem", "Poem", false); got.MatchKind != "exact" || got.Role != "verse" {
		t.Fatalf("exact mismatch: %#v", got)
	}
	if got := resolver.Resolve("Child", "Child", false); got.MatchKind != "inherited" || got.Role != "body" {
		t.Fatalf("inherited mismatch: %#v", got)
	}
	if got := resolver.Resolve("", "body-text", false); got.MatchKind != "normalized" || got.Role != "body" {
		t.Fatalf("normalized mismatch: %#v", got)
	}
}

func TestStyleResolverEmitsCycleWarning(t *testing.T) {
	resolver := NewStyleResolver("paragraph", map[string]config.StyleMapping{}, map[string]StyleDef{
		"A": {ID: "A", DisplayName: "A", ParentID: "B"},
		"B": {ID: "B", DisplayName: "B", ParentID: "A"},
	}, true)
	got := resolver.Resolve("A", "A", false)
	if len(got.Issues) == 0 || got.Issues[0].Code != "t-warn-style-cycle" {
		t.Fatalf("expected cycle warning, got %#v", got.Issues)
	}
}

func TestStyleResolverContinuesAfterAmbiguousNormalizedCandidate(t *testing.T) {
	mappings := map[string]config.StyleMapping{
		"Body Text": {Role: "body"},
		"Body-Text": {Role: "body"},
		"Poem":      {Role: "verse"},
	}
	defs := map[string]StyleDef{
		"Child": {ID: "Child", DisplayName: "Body/Text", ParentID: "Base"},
		"Base":  {ID: "Base", DisplayName: "poem!"},
	}
	resolver := NewStyleResolver("paragraph", mappings, defs, true)
	got := resolver.Resolve("Child", "Body/Text", false)
	if got.MatchKind != "normalized" || got.Role != "verse" || got.MatchedName != "Poem" {
		t.Fatalf("expected parent normalized match after ambiguous child, got %#v", got)
	}
}

func TestAttachCaptions(t *testing.T) {
	blocks := AttachCaptions([]ir.Block{
		ir.Figure{Image: ir.Image{Name: "image.png"}},
		ir.Paragraph{Role: CaptionBlockRole, Inlines: []ir.Inline{ir.Text{Value: "Caption"}}},
	})
	if len(blocks) != 1 {
		t.Fatalf("expected caption to attach to figure, got %#v", blocks)
	}
	fig, ok := blocks[0].(ir.Figure)
	if !ok || len(fig.Caption) != 1 {
		t.Fatalf("caption not attached: %#v", blocks[0])
	}
}

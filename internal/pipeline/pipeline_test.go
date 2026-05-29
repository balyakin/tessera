package pipeline

import (
	"testing"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func TestCalculateStats(t *testing.T) {
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Chapter"}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Hello, world."}}},
			ir.Table{Rows: []ir.TableRow{{Cells: []ir.TableCell{{Children: []ir.Block{ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "cell"}}}}}}}}},
		},
	}
	stats := CalculateStats(doc, parser.StyleReport{
		ParagraphStyles: []parser.StyleUsage{{Name: "Body", Status: "mapped"}, {Name: "Small", Status: "unknown"}},
	})
	if stats.Words != 4 || stats.Chapters != 1 || stats.Tables != 1 || stats.ParagraphStylesUnknown != 1 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestCalculateStatsUsesTopLevelHeadingsForChapters(t *testing.T) {
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.BlockQuote{Children: []ir.Block{ir.Heading{Level: 1, Inlines: []ir.Inline{ir.Text{Value: "Nested"}}}}},
			ir.Heading{Level: 2, Inlines: []ir.Inline{ir.Text{Value: "Chapter"}}},
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{ir.Text{Value: "Body"}}},
		},
	}
	stats := CalculateStats(doc, parser.StyleReport{})
	if stats.Chapters != 1 {
		t.Fatalf("expected top-level chapter count, got %#v", stats)
	}
}

func TestInlineDepthLimit(t *testing.T) {
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      ir.Metadata{Title: "Demo", Language: "en"},
		Body: []ir.Block{
			ir.Paragraph{Role: ir.RoleBody, Inlines: []ir.Inline{
				ir.Styled{Role: ir.Emphasis, Children: []ir.Inline{
					ir.Styled{Role: ir.Strong, Children: []ir.Inline{ir.Text{Value: "deep"}}},
				}},
			}},
		},
	}
	issues := enforceLimits(doc, config.LimitsConfig{MaxBlocks: 100, MaxInlineDepth: 2})
	found := false
	for _, issue := range issues {
		if issue.Code == "t-err-limit-inline-depth" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected inline depth issue, got %#v", issues)
	}
}

func TestValidateSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-an-int")
	if err := validateSourceDateEpoch(); err == nil {
		t.Fatalf("expected invalid SOURCE_DATE_EPOCH")
	}
	t.Setenv("SOURCE_DATE_EPOCH", "0")
	if err := validateSourceDateEpoch(); err != nil {
		t.Fatal(err)
	}
}

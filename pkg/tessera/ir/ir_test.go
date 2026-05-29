package ir

import (
	"strings"
	"testing"
)

func TestCanonicalIRRoundTrip(t *testing.T) {
	doc := &Document{
		IRVersion: CurrentIRVersion,
		Meta: Metadata{
			Title:      "Demo",
			Language:   "en",
			Identifier: "urn:test",
			Extra: map[string]string{
				"series": "A",
				"rights": "Copyright",
			},
		},
		Body: []Block{
			Heading{Level: 1, Inlines: []Inline{Text{Value: "Chapter"}}},
			Paragraph{Role: RoleBody, Inlines: []Inline{Styled{Role: Thought, Children: []Inline{Text{Value: "thinking"}}}}},
		},
	}
	data, err := MarshalCanonical(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"_type": "heading"`) {
		t.Fatalf("missing _type discriminator:\n%s", data)
	}
	roundTrip, err := UnmarshalCanonical(data)
	if err != nil {
		t.Fatal(err)
	}
	if roundTrip.Meta.Title != "Demo" || len(roundTrip.Body) != 2 {
		t.Fatalf("unexpected round trip document: %#v", roundTrip)
	}
}

func TestWalkDeterministic(t *testing.T) {
	doc := &Document{
		IRVersion: CurrentIRVersion,
		Meta:      Metadata{Title: "Demo", Language: "en"},
		Body: []Block{
			Paragraph{Role: RoleBody, Inlines: []Inline{Text{Value: "A"}, Styled{Role: Emphasis, Children: []Inline{Text{Value: "B"}}}}},
		},
	}
	var seen []string
	Walk(doc, visitorFuncs{
		enterBlock: func(Block) bool {
			seen = append(seen, "block")
			return true
		},
		enterInline: func(inline Inline) bool {
			switch inline.(type) {
			case Text:
				seen = append(seen, "text")
			case Styled:
				seen = append(seen, "styled")
			}
			return true
		},
	})
	got := strings.Join(seen, ",")
	if got != "block,text,styled,text" {
		t.Fatalf("unexpected walk order: %s", got)
	}
}

func TestValidateDocumentLanguageWarnings(t *testing.T) {
	base := &Document{
		IRVersion: CurrentIRVersion,
		Meta:      Metadata{Title: "Demo"},
		Body:      []Block{Paragraph{Role: RoleBody, Inlines: []Inline{Text{Value: "Body"}}}},
	}
	if issues := ValidateDocument(base, map[string]string{"en": "english"}); !hasCode(issues, "t-warn-lang-unknown") {
		t.Fatalf("empty language should warn, got %#v", issues)
	}
	base.Meta.Language = "zz"
	if issues := ValidateDocument(base, map[string]string{"en": "english"}); !hasCode(issues, "t-warn-lang-unknown") {
		t.Fatalf("unknown configured language should warn, got %#v", issues)
	}
}

func hasCode(issues []Issue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

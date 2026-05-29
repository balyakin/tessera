package parser

import "testing"

func TestStyleReportZeroValue(t *testing.T) {
	var report StyleReport
	if len(report.ParagraphStyles) != 0 || len(report.CharacterStyles) != 0 {
		t.Fatalf("unexpected zero value")
	}
}

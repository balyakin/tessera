package backend

import "testing"

func TestOutputKindConstants(t *testing.T) {
	if OutputPDF != "pdf" || OutputEPUB != "epub" || OutputTEX != "tex" {
		t.Fatalf("unexpected output constants")
	}
}

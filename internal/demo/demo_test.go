package demo

import "testing"

func TestDemoArchives(t *testing.T) {
	if data, err := ODT(); err != nil || len(data) == 0 {
		t.Fatalf("ODT generation failed: %v", err)
	}
	if data, err := DOCX(); err != nil || len(data) == 0 {
		t.Fatalf("DOCX generation failed: %v", err)
	}
}

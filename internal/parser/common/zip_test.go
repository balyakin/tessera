package common

import (
	"archive/zip"
	"bytes"
	"testing"
)

func TestOpenZipRejectsBackslashTraversal(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, err := zw.Create(`..\secret.txt`); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenZip(bytes.NewReader(buf.Bytes()), int64(buf.Len())); err == nil {
		t.Fatalf("expected traversal entry to be rejected")
	}
}

func TestContainsDTDOrEntityDirective(t *testing.T) {
	if !containsDTDOrEntity([]byte(`<?xml version="1.0"?><!DOCTYPE x><x/>`)) {
		t.Fatalf("expected DTD detection")
	}
}

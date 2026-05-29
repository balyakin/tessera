package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestHelpAndVersionJSON(t *testing.T) {
	var out, errb bytes.Buffer
	root := NewRootCommand(&out, &errb)
	root.SetArgs([]string{"version", "--format", "json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"version"`)) {
		t.Fatalf("missing version json: %s", out.String())
	}
}

func TestRunEPUBCheckModes(t *testing.T) {
	if err := runEPUBCheck("never", "missing.epub"); err != nil {
		t.Fatal(err)
	}
	if err := runEPUBCheck("always", "missing.epub"); err == nil {
		t.Fatalf("expected missing epubcheck to fail in always mode")
	}
	if runtime.GOOS == "windows" {
		t.Skip("shell script PATH test is Unix-specific")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "epubcheck")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	if err := runEPUBCheck("auto", "book.epub"); err != nil {
		t.Fatal(err)
	}
}

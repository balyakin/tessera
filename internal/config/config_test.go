package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefault(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.DefaultLanguage != "en" {
		t.Fatalf("unexpected default language %q", cfg.Document.DefaultLanguage)
	}
	if cfg.ParagraphStyles["Poem"].Role != "verse" {
		t.Fatalf("missing poem mapping")
	}
}

func TestUserConfigMergesStylesAndScalars(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tessera.toml")
	if err := os.WriteFile(path, []byte(`
[document]
default_language = "ru"

[paragraph_styles]
"Scene Break" = { role = "body" }
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Document.DefaultLanguage != "ru" {
		t.Fatalf("user scalar did not override")
	}
	if cfg.ParagraphStyles["Poem"].Role != "verse" || cfg.ParagraphStyles["Scene Break"].Role != "body" {
		t.Fatalf("style maps did not merge")
	}
}

func TestRejectsUnknownAndErrorSuppressedIssueCodes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte(`[issues]
suppress = ["t-lint-002"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected linter error suppression to fail")
	}
	if err := os.WriteFile(path, []byte(`[issues]
promote = ["t-warn-does-not-exist"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected unknown issue code to fail")
	}
}

func TestRejectsRemoteCustomCSSReferences(t *testing.T) {
	dir := t.TempDir()
	css := filepath.Join(dir, "custom.css")
	if err := os.WriteFile(css, []byte(`@import url("//example.com/a.css"); p { background: url(ftp://example.com/a.png); }`), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte(`[epub]
custom_css = "custom.css"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatalf("expected remote CSS references to fail")
	}
}

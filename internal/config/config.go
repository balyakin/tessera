package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Document        DocumentConfig          `toml:"document"`
	ParagraphStyles map[string]StyleMapping `toml:"paragraph_styles"`
	CharacterStyles map[string]StyleMapping `toml:"character_styles"`
	Languages       map[string]string       `toml:"languages"`
	StyleMatching   StyleMatchingConfig     `toml:"style_matching"`
	Latex           LatexConfig             `toml:"latex"`
	EPUB            EPUBConfig              `toml:"epub"`
	TOC             TOCConfig               `toml:"toc"`
	Issues          IssueConfig             `toml:"issues"`
	Limits          LimitsConfig            `toml:"limits"`
	Output          OutputConfig            `toml:"output"`

	BaseDir string `toml:"-"`
}

type DocumentConfig struct {
	DefaultLanguage string            `toml:"default_language"`
	Title           string            `toml:"title"`
	Author          string            `toml:"author"`
	CoverImage      string            `toml:"cover_image"`
	CoverAlt        string            `toml:"cover_alt"`
	Extra           map[string]string `toml:"extra"`
}

type StyleMapping struct {
	Role  string `toml:"role"`
	Level int    `toml:"level"`
	Lang  string `toml:"lang"`
}

type StyleMatchingConfig struct {
	NormalizedFallback bool `toml:"normalized_fallback"`
}

type LatexConfig struct {
	DocumentClassOptions string   `toml:"document_class_options"`
	MainFont             string   `toml:"main_font"`
	PreambleFile         string   `toml:"preamble_file"`
	ExtraPreamble        []string `toml:"extra_preamble"`
	ExtraMacros          []string `toml:"extra_macros"`
}

type EPUBConfig struct {
	CustomCSS       string   `toml:"custom_css"`
	AdditionalFonts []string `toml:"additional_fonts"`
}

type TOCConfig struct {
	Depth        int    `toml:"depth"`
	Title        string `toml:"title"`
	IncludeInPDF bool   `toml:"include_in_pdf"`
}

type IssueConfig struct {
	Suppress []string `toml:"suppress"`
	Promote  []string `toml:"promote"`
}

type LimitsConfig struct {
	MaxBlocks      int `toml:"max_blocks"`
	MaxFootnotes   int `toml:"max_footnotes"`
	MaxTableCells  int `toml:"max_table_cells"`
	MaxInlineDepth int `toml:"max_inline_depth"`
}

type OutputConfig struct {
	Reproducible bool `toml:"reproducible"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if err := toml.Unmarshal([]byte(defaultTOML), &cfg); err != nil {
		return nil, fmt.Errorf("load default config: %w", err)
	}
	baseDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read config %s: %w", path, err)
		}
		var user Config
		if err := toml.Unmarshal(data, &user); err != nil {
			return nil, fmt.Errorf("parse config %s: %w", path, err)
		}
		var raw map[string]any
		if err := toml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parse config keys %s: %w", path, err)
		}
		merge(&cfg, user, raw)
		baseDir = filepath.Dir(path)
	}
	cfg.BaseDir = baseDir
	resolvePaths(&cfg, baseDir)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func merge(dst *Config, src Config, raw map[string]any) {
	if section, ok := raw["document"].(map[string]any); ok {
		if has(section, "default_language") {
			dst.Document.DefaultLanguage = src.Document.DefaultLanguage
		}
		if has(section, "title") {
			dst.Document.Title = src.Document.Title
		}
		if has(section, "author") {
			dst.Document.Author = src.Document.Author
		}
		if has(section, "cover_image") {
			dst.Document.CoverImage = src.Document.CoverImage
		}
		if has(section, "cover_alt") {
			dst.Document.CoverAlt = src.Document.CoverAlt
		}
		if src.Document.Extra != nil {
			if dst.Document.Extra == nil {
				dst.Document.Extra = map[string]string{}
			}
			for k, v := range src.Document.Extra {
				dst.Document.Extra[k] = v
			}
		}
	}
	if src.ParagraphStyles != nil {
		if dst.ParagraphStyles == nil {
			dst.ParagraphStyles = map[string]StyleMapping{}
		}
		for k, v := range src.ParagraphStyles {
			dst.ParagraphStyles[k] = v
		}
	}
	if src.CharacterStyles != nil {
		if dst.CharacterStyles == nil {
			dst.CharacterStyles = map[string]StyleMapping{}
		}
		for k, v := range src.CharacterStyles {
			dst.CharacterStyles[k] = v
		}
	}
	if src.Languages != nil {
		if dst.Languages == nil {
			dst.Languages = map[string]string{}
		}
		for k, v := range src.Languages {
			dst.Languages[k] = v
		}
	}
	if section, ok := raw["style_matching"].(map[string]any); ok && has(section, "normalized_fallback") {
		dst.StyleMatching = src.StyleMatching
	}
	if section, ok := raw["latex"].(map[string]any); ok {
		if has(section, "document_class_options") {
			dst.Latex.DocumentClassOptions = src.Latex.DocumentClassOptions
		}
		if has(section, "main_font") {
			dst.Latex.MainFont = src.Latex.MainFont
		}
		if has(section, "preamble_file") {
			dst.Latex.PreambleFile = src.Latex.PreambleFile
		}
		if has(section, "extra_preamble") {
			dst.Latex.ExtraPreamble = src.Latex.ExtraPreamble
		}
		if has(section, "extra_macros") {
			dst.Latex.ExtraMacros = src.Latex.ExtraMacros
		}
	}
	if section, ok := raw["epub"].(map[string]any); ok {
		if has(section, "custom_css") {
			dst.EPUB.CustomCSS = src.EPUB.CustomCSS
		}
		if has(section, "additional_fonts") {
			dst.EPUB.AdditionalFonts = src.EPUB.AdditionalFonts
		}
	}
	if section, ok := raw["toc"].(map[string]any); ok {
		if has(section, "depth") {
			dst.TOC.Depth = src.TOC.Depth
		}
		if has(section, "title") {
			dst.TOC.Title = src.TOC.Title
		}
		if has(section, "include_in_pdf") {
			dst.TOC.IncludeInPDF = src.TOC.IncludeInPDF
		}
	}
	if section, ok := raw["issues"].(map[string]any); ok {
		if has(section, "suppress") {
			dst.Issues.Suppress = src.Issues.Suppress
		}
		if has(section, "promote") {
			dst.Issues.Promote = src.Issues.Promote
		}
	}
	if section, ok := raw["limits"].(map[string]any); ok {
		if has(section, "max_blocks") {
			dst.Limits.MaxBlocks = src.Limits.MaxBlocks
		}
		if has(section, "max_footnotes") {
			dst.Limits.MaxFootnotes = src.Limits.MaxFootnotes
		}
		if has(section, "max_table_cells") {
			dst.Limits.MaxTableCells = src.Limits.MaxTableCells
		}
		if has(section, "max_inline_depth") {
			dst.Limits.MaxInlineDepth = src.Limits.MaxInlineDepth
		}
	}
	if section, ok := raw["output"].(map[string]any); ok && has(section, "reproducible") {
		dst.Output.Reproducible = src.Output.Reproducible
	}
}

func has(m map[string]any, key string) bool {
	_, ok := m[key]
	return ok
}

func resolvePaths(cfg *Config, baseDir string) {
	cfg.Document.CoverImage = resolvePath(baseDir, cfg.Document.CoverImage)
	cfg.Latex.PreambleFile = resolvePath(baseDir, cfg.Latex.PreambleFile)
	cfg.EPUB.CustomCSS = resolvePath(baseDir, cfg.EPUB.CustomCSS)
	for i, font := range cfg.EPUB.AdditionalFonts {
		cfg.EPUB.AdditionalFonts[i] = resolvePath(baseDir, font)
	}
}

func resolvePath(baseDir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(baseDir, path))
}

func validate(cfg *Config) error {
	for name, mapping := range cfg.ParagraphStyles {
		if !validParagraphRole(mapping.Role) {
			return fmt.Errorf("config error: unknown paragraph role %q for style %q", mapping.Role, name)
		}
		if mapping.Role == "heading" && (mapping.Level < 1 || mapping.Level > 6) {
			return fmt.Errorf("config error: heading style %q requires level 1..6", name)
		}
	}
	for name, mapping := range cfg.CharacterStyles {
		if !validCharacterRole(mapping.Role) {
			return fmt.Errorf("config error: unknown character role %q for style %q", mapping.Role, name)
		}
	}
	if cfg.TOC.Depth < 0 || cfg.TOC.Depth > 6 {
		return fmt.Errorf("config error: toc.depth must be between 0 and 6")
	}
	if cfg.Document.CoverImage != "" {
		if err := validateReadableExt(cfg.Document.CoverImage, []string{".png", ".jpg", ".jpeg"}); err != nil {
			return fmt.Errorf("config error: cover_image: %w", err)
		}
	}
	if cfg.EPUB.CustomCSS != "" {
		if err := validateReadableExt(cfg.EPUB.CustomCSS, []string{".css"}); err != nil {
			return fmt.Errorf("config error: epub.custom_css: %w", err)
		}
		data, err := os.ReadFile(cfg.EPUB.CustomCSS)
		if err != nil {
			return fmt.Errorf("config error: epub.custom_css: %w", err)
		}
		if cssReferencesRemoteResource(string(data)) {
			return fmt.Errorf("config error: epub.custom_css must not reference remote resources")
		}
	}
	for _, font := range cfg.EPUB.AdditionalFonts {
		if err := validateReadableExt(font, []string{".otf", ".ttf", ".woff", ".woff2"}); err != nil {
			return fmt.Errorf("config error: epub.additional_fonts: %w", err)
		}
	}
	if cfg.Latex.PreambleFile != "" {
		if _, err := os.Stat(cfg.Latex.PreambleFile); err != nil {
			return fmt.Errorf("config error: latex.preamble_file: %w", err)
		}
	}
	for _, code := range cfg.Issues.Suppress {
		if strings.HasPrefix(code, "t-err-") {
			return fmt.Errorf("config error: cannot suppress error code %s", code)
		}
		if !knownIssueCodes[code] {
			return fmt.Errorf("config error: unknown issue code %s", code)
		}
		if linterErrorCodes[code] {
			return fmt.Errorf("config error: cannot suppress linter error code %s", code)
		}
	}
	for _, code := range cfg.Issues.Promote {
		if !knownIssueCodes[code] {
			return fmt.Errorf("config error: unknown issue code %s", code)
		}
	}
	return nil
}

var cssRemoteReferenceRe = regexp.MustCompile(`(?i)(url\(\s*['"]?\s*(?:[a-z][a-z0-9+.-]*:|//)|@import\s+(?:url\(\s*)?['"]?\s*(?:[a-z][a-z0-9+.-]*:|//))`)

func cssReferencesRemoteResource(css string) bool {
	return cssRemoteReferenceRe.MatchString(css)
}

var knownIssueCodes = map[string]bool{
	"t-err-ir-version":             true,
	"t-err-empty-body":             true,
	"t-err-title-empty":            true,
	"t-err-heading-level":          true,
	"t-err-footnote-missing":       true,
	"t-err-footnote-duplicate":     true,
	"t-err-limit-blocks":           true,
	"t-err-limit-footnotes":        true,
	"t-err-limit-table-cells":      true,
	"t-err-limit-inline-depth":     true,
	"t-err-latex-timeout":          true,
	"t-warn-foreign-nolang":        true,
	"t-warn-alt":                   true,
	"t-warn-cover-alt":             true,
	"t-warn-lang-unknown":          true,
	"t-warn-style-cycle":           true,
	"t-warn-style-fuzzy":           true,
	"t-warn-style-fuzzy-ambiguous": true,
	"t-warn-unknown-pstyle":        true,
	"t-warn-unknown-cstyle":        true,
	"t-warn-numbering":             true,
	"t-warn-list-nesting":          true,
	"t-warn-table-merge":           true,
	"t-warn-footnote-flattened":    true,
	"t-warn-footnote-repeated":     true,
	"t-warn-embedded-object":       true,
	"t-warn-link-target":           true,
	"t-warn-link-scheme":           true,
	"t-warn-image-pdf-unsupported": true,
	"t-warn-foreign-latex":         true,
	"t-warn-pagebreak":             true,
	"t-warn-font-fallback":         true,
	"t-lint-001":                   true,
	"t-lint-002":                   true,
	"t-lint-003":                   true,
	"t-lint-004":                   true,
	"t-lint-005":                   true,
	"t-lint-006":                   true,
	"t-lint-007":                   true,
	"t-lint-008":                   true,
	"t-lint-009":                   true,
	"t-lint-010":                   true,
	"t-lint-011":                   true,
	"t-lint-012":                   true,
}

var linterErrorCodes = map[string]bool{
	"t-lint-001": true,
	"t-lint-002": true,
	"t-lint-003": true,
	"t-lint-004": true,
	"t-lint-005": true,
	"t-lint-006": true,
	"t-lint-011": true,
}

func validParagraphRole(role string) bool {
	return slices.Contains([]string{
		"body", "heading", "title", "subtitle", "dedication", "colophon",
		"glossary", "halftitle", "verse", "letter", "epigraph", "blockquote",
	}, role)
}

func validCharacterRole(role string) bool {
	return slices.Contains([]string{"emphasis", "strong", "foreign", "thought", "prayer", "work-title"}, role)
}

func validateReadableExt(path string, allowed []string) error {
	ext := strings.ToLower(filepath.Ext(path))
	if !slices.Contains(allowed, ext) {
		return fmt.Errorf("unsupported extension %q", ext)
	}
	if _, err := os.Stat(path); err != nil {
		return err
	}
	return nil
}

package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/backend/epub"
	epublint "github.com/balyakin/tessera/internal/backend/epub/lint"
	"github.com/balyakin/tessera/internal/backend/latex"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/internal/parser/docx"
	"github.com/balyakin/tessera/internal/parser/odt"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type InputFormat string

const (
	FormatODT  InputFormat = "odt"
	FormatDOCX InputFormat = "docx"
)

type OutputKind string

const (
	OutputPDF  OutputKind = "pdf"
	OutputEPUB OutputKind = "epub"
	OutputTEX  OutputKind = "tex"
)

type BuildOptions struct {
	InputPath    string
	OutputDir    string
	ConfigPath   string
	Metadata     map[string]string
	Targets      []OutputKind
	Engine       string
	KeepTex      bool
	Lint         bool
	Reproducible bool
	StrictStyles bool
	DumpIRPath   string
	OnProgress   func(phase string, detail string, percent float64)
}

type Artifact struct {
	Kind OutputKind
	Path string
}

type BuildResult struct {
	InputPath     string
	InputFormat   InputFormat
	Artifacts     []Artifact
	Issues        []ir.Issue
	LintFindings  []LintFinding
	Stats         DocumentStats
	ElapsedMillis int64
}

type DocumentStats struct {
	Words                  int `json:"words"`
	Characters             int `json:"characters"`
	Chapters               int `json:"chapters"`
	Footnotes              int `json:"footnotes"`
	Images                 int `json:"images"`
	Tables                 int `json:"tables"`
	ParagraphStylesTotal   int `json:"paragraph_styles_total"`
	ParagraphStylesMapped  int `json:"paragraph_styles_mapped"`
	ParagraphStylesUnknown int `json:"paragraph_styles_unknown"`
	CharacterStylesTotal   int `json:"character_styles_total"`
	CharacterStylesMapped  int `json:"character_styles_mapped"`
	CharacterStylesUnknown int `json:"character_styles_unknown"`
}

type InspectResult struct {
	InputPath        string
	InputFormat      InputFormat
	Metadata         ir.Metadata
	Stats            DocumentStats
	ParagraphStyles  []InspectStyle
	CharacterStyles  []InspectStyle
	DirectFormatting []DirectFormattingStat
	Issues           []ir.Issue
}

type InspectStyle struct {
	Name          string
	Family        string
	Status        string
	Role          string
	MatchedName   string
	MatchKind     string
	SuggestedTOML string
}

type DirectFormattingStat struct {
	Role  string
	Count int
}

type LintFinding struct {
	RuleID   string
	Severity string
	File     string
	Line     int
	Message  string
}

type ParseFileOptions struct {
	InputPath    string
	ConfigPath   string
	Metadata     map[string]string
	StrictStyles bool
}

type ParseFileResult struct {
	Document    *ir.Document
	Format      InputFormat
	Issues      []ir.Issue
	StyleReport parser.StyleReport
	Stats       DocumentStats
}

type Error struct {
	Code  int
	Phase string
	Err   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %v", e.Phase, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
func (e *Error) ExitCode() int { return e.Code }

type pdfBackend struct {
	engine  string
	keepTex bool
}

func (b pdfBackend) Render(doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) ([]backend.Artifact, []ir.Issue, error) {
	result, err := latex.Render(doc, cfg, opts)
	if err != nil {
		return nil, nil, err
	}
	var artifacts []backend.Artifact
	if b.keepTex {
		texPath := filepath.Join(opts.OutputDir, opts.Basename+".tex")
		if err := os.WriteFile(texPath, []byte(result.TexSource), 0o644); err != nil {
			return nil, nil, fmt.Errorf("write LaTeX: %w", err)
		}
		artifacts = append(artifacts, backend.Artifact{Kind: backend.OutputTEX, Path: texPath})
	}
	pdfPath, err := compilePDF(b.engine, opts.OutputDir, opts.Basename, result)
	if err != nil {
		return nil, nil, err
	}
	artifacts = append(artifacts, backend.Artifact{Kind: backend.OutputPDF, Path: pdfPath})
	return artifacts, result.Issues, nil
}

func BuildFile(opts BuildOptions) (*BuildResult, error) {
	start := time.Now()
	if opts.InputPath == "" {
		return nil, coded(2, "input", errors.New("input path is required"))
	}
	if opts.OutputDir == "" {
		opts.OutputDir = "."
	}
	if opts.Engine == "" {
		opts.Engine = "lualatex"
	}
	targets := normalizeTargets(opts.Targets)
	if opts.Lint && !containsTarget(targets, OutputEPUB) {
		return nil, coded(3, "config", errors.New("--lint requires an EPUB target"))
	}
	if err := validateSourceDateEpoch(); err != nil {
		return nil, coded(3, "config", err)
	}
	progress(opts, "Load config", opts.ConfigPath, 0.05)
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, coded(3, "config", err)
	}
	if cfg.Output.Reproducible {
		opts.Reproducible = true
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return nil, coded(1, "write", err)
	}
	parsed, err := ParseFile(ParseFileOptions{
		InputPath:    opts.InputPath,
		ConfigPath:   opts.ConfigPath,
		Metadata:     opts.Metadata,
		StrictStyles: opts.StrictStyles,
	})
	if err != nil {
		return nil, err
	}
	doc := parsed.Document
	cover, coverIssues, err := loadCover(cfg, doc.Meta.Title)
	if err != nil {
		return nil, coded(3, "config", err)
	}
	doc.Cover = cover
	issues := append([]ir.Issue{}, parsed.Issues...)
	issues = append(issues, coverIssues...)
	progress(opts, "Validate", opts.InputPath, 0.45)
	issues = append(issues, ir.ValidateDocument(doc, cfg.Languages)...)
	issues = append(issues, enforceLimits(doc, cfg.Limits)...)
	validationIssues := filterIssues(issues, cfg)
	if hasIssueErrors(validationIssues) {
		return &BuildResult{InputPath: opts.InputPath, InputFormat: parsed.Format, Issues: validationIssues, Stats: parsed.Stats, ElapsedMillis: time.Since(start).Milliseconds()}, coded(2, "validate", errors.New(firstErrorMessage(validationIssues)))
	}
	if opts.DumpIRPath != "" {
		data, err := ir.MarshalCanonical(doc)
		if err != nil {
			return nil, coded(2, "validate", err)
		}
		if err := os.WriteFile(opts.DumpIRPath, data, 0o644); err != nil {
			return nil, coded(1, "write", err)
		}
	}
	base := strings.TrimSuffix(filepath.Base(opts.InputPath), filepath.Ext(opts.InputPath))
	renderOpts := backend.RenderOptions{OutputDir: opts.OutputDir, Basename: base, Reproducible: opts.Reproducible}
	if opts.Reproducible {
		renderOpts.SourceDateUTC = time.Unix(0, 0).UTC()
	}
	var artifacts []Artifact
	var epubPath string
	registry := newBackendRegistry(opts.Engine, opts.KeepTex && !containsTarget(targets, OutputTEX))
	for _, target := range targets {
		renderer := registry[target]
		if renderer == nil {
			return nil, coded(3, "config", fmt.Errorf("unsupported output target %q", target))
		}
		progress(opts, "Render", string(target), renderProgress(target))
		targetArtifacts, targetIssues, err := renderWithBackend(renderer, doc, cfg, renderOpts)
		if err != nil {
			return nil, coded(4, "render", err)
		}
		issues = append(issues, targetIssues...)
		artifacts = append(artifacts, targetArtifacts...)
		for _, artifact := range targetArtifacts {
			if artifact.Kind == OutputEPUB {
				epubPath = artifact.Path
			}
		}
	}
	issues = filterIssues(issues, cfg)
	if hasIssueErrors(issues) {
		return &BuildResult{InputPath: opts.InputPath, InputFormat: parsed.Format, Artifacts: orderArtifacts(artifacts), Issues: issues, Stats: parsed.Stats, ElapsedMillis: time.Since(start).Milliseconds()}, coded(2, "render", errors.New(firstErrorMessage(issues)))
	}
	var findings []LintFinding
	if opts.Lint && epubPath != "" {
		progress(opts, "Lint", "EPUB", 0.9)
		data, err := os.ReadFile(epubPath)
		if err != nil {
			return nil, coded(1, "lint", err)
		}
		for _, finding := range epublint.Lint(data) {
			findings = append(findings, LintFinding(finding))
		}
		findings = filterFindings(findings, cfg)
		if lintHasErrors(findings) {
			return &BuildResult{InputPath: opts.InputPath, InputFormat: parsed.Format, Artifacts: orderArtifacts(artifacts), Issues: issues, LintFindings: findings, Stats: parsed.Stats, ElapsedMillis: time.Since(start).Milliseconds()}, coded(5, "lint", errors.New("EPUB linter found errors"))
		}
	}
	progress(opts, "Write", opts.OutputDir, 1)
	return &BuildResult{
		InputPath:     opts.InputPath,
		InputFormat:   parsed.Format,
		Artifacts:     orderArtifacts(artifacts),
		Issues:        issues,
		LintFindings:  findings,
		Stats:         parsed.Stats,
		ElapsedMillis: time.Since(start).Milliseconds(),
	}, nil
}

func ParseFile(opts ParseFileOptions) (*ParseFileResult, error) {
	if opts.InputPath == "" {
		return nil, coded(2, "input", errors.New("input path is required"))
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, coded(3, "config", err)
	}
	data, err := os.ReadFile(opts.InputPath)
	if err != nil {
		return nil, coded(1, "input", err)
	}
	format, err := DetectFormat(opts.InputPath, data)
	if err != nil {
		return nil, coded(2, "detect", err)
	}
	var p parser.Parser
	switch format {
	case FormatODT:
		p = odt.Parser{}
	case FormatDOCX:
		p = docx.Parser{}
	default:
		return nil, coded(2, "detect", fmt.Errorf("unsupported format %q", format))
	}
	result, err := p.Parse(bytes.NewReader(data), int64(len(data)), cfg, parser.ParseOptions{
		InputPath:    opts.InputPath,
		Metadata:     opts.Metadata,
		StrictStyles: opts.StrictStyles,
	})
	if err != nil {
		return nil, coded(2, "parse", err)
	}
	stats := CalculateStats(result.Document, result.StyleReport)
	return &ParseFileResult{
		Document:    result.Document,
		Format:      format,
		Issues:      filterIssues(result.Issues, cfg),
		StyleReport: result.StyleReport,
		Stats:       stats,
	}, nil
}

func InspectFile(opts ParseFileOptions) (*InspectResult, error) {
	parsed, err := ParseFile(opts)
	if err != nil {
		return nil, err
	}
	return &InspectResult{
		InputPath:        opts.InputPath,
		InputFormat:      parsed.Format,
		Metadata:         parsed.Document.Meta,
		Stats:            parsed.Stats,
		ParagraphStyles:  convertStyles(parsed.StyleReport.ParagraphStyles),
		CharacterStyles:  convertStyles(parsed.StyleReport.CharacterStyles),
		DirectFormatting: convertDirect(parsed.StyleReport.DirectFormatting),
		Issues:           parsed.Issues,
	}, nil
}

func normalizeTargets(targets []OutputKind) []OutputKind {
	if len(targets) == 0 {
		return []OutputKind{OutputPDF, OutputEPUB}
	}
	return targets
}

func containsTarget(targets []OutputKind, target OutputKind) bool {
	for _, t := range targets {
		if t == target {
			return true
		}
	}
	return false
}

func newBackendRegistry(engine string, keepTex bool) map[OutputKind]backend.Backend {
	return map[OutputKind]backend.Backend{
		OutputPDF:  pdfBackend{engine: engine, keepTex: keepTex},
		OutputTEX:  latex.FileBackend{Kind: backend.OutputTEX},
		OutputEPUB: epub.FileBackend{},
	}
}

func renderWithBackend(renderer backend.Backend, doc *ir.Document, cfg *config.Config, opts backend.RenderOptions) ([]Artifact, []ir.Issue, error) {
	backendArtifacts, issues, err := renderer.Render(doc, cfg, opts)
	if err != nil {
		return nil, nil, err
	}
	artifacts := make([]Artifact, 0, len(backendArtifacts))
	for _, artifact := range backendArtifacts {
		artifacts = append(artifacts, Artifact{Kind: OutputKind(artifact.Kind), Path: artifact.Path})
	}
	return artifacts, issues, nil
}

func renderProgress(target OutputKind) float64 {
	switch target {
	case OutputPDF:
		return 0.6
	case OutputTEX:
		return 0.6
	case OutputEPUB:
		return 0.8
	default:
		return 0.6
	}
}

func progress(opts BuildOptions, phase, detail string, percent float64) {
	if opts.OnProgress != nil {
		opts.OnProgress(phase, detail, percent)
	}
}

func coded(code int, phase string, err error) *Error {
	return &Error{Code: code, Phase: phase, Err: err}
}

func convertStyles(styles []parser.StyleUsage) []InspectStyle {
	out := make([]InspectStyle, 0, len(styles))
	for _, style := range styles {
		out = append(out, InspectStyle(style))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func convertDirect(stats []parser.DirectFormattingStat) []DirectFormattingStat {
	out := make([]DirectFormattingStat, 0, len(stats))
	for _, stat := range stats {
		out = append(out, DirectFormattingStat(stat))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Role < out[j].Role })
	return out
}

func CalculateStats(doc *ir.Document, report parser.StyleReport) DocumentStats {
	stats := DocumentStats{Footnotes: len(doc.Footnotes)}
	minHeading := 7
	for _, block := range doc.Body {
		if h, ok := block.(ir.Heading); ok && h.Level < minHeading {
			minHeading = h.Level
		}
	}
	ir.Walk(doc, visitor{
		block: func(block ir.Block) bool {
			switch block.(type) {
			case ir.Figure:
				stats.Images++
			case ir.Table:
				stats.Tables++
			}
			return true
		},
		inline: func(inline ir.Inline) bool {
			switch v := inline.(type) {
			case ir.Text:
				stats.Characters += len([]rune(v.Value))
				stats.Words += countWords(v.Value)
			case ir.InlineImage:
				stats.Images++
			}
			return true
		},
	})
	if len(doc.Body) > 0 {
		if minHeading == 7 {
			stats.Chapters = 1
		} else {
			for _, block := range doc.Body {
				if h, ok := block.(ir.Heading); ok && h.Level == minHeading {
					stats.Chapters++
				}
			}
		}
	}
	stats.ParagraphStylesTotal, stats.ParagraphStylesMapped, stats.ParagraphStylesUnknown = styleCounts(report.ParagraphStyles)
	stats.CharacterStylesTotal, stats.CharacterStylesMapped, stats.CharacterStylesUnknown = styleCounts(report.CharacterStyles)
	return stats
}

type visitor struct {
	block  func(ir.Block) bool
	inline func(ir.Inline) bool
}

func (v visitor) EnterBlock(block ir.Block) bool {
	if v.block == nil {
		return true
	}
	return v.block(block)
}
func (v visitor) LeaveBlock(ir.Block) {}
func (v visitor) EnterInline(inline ir.Inline) bool {
	if v.inline == nil {
		return true
	}
	return v.inline(inline)
}
func (v visitor) LeaveInline(ir.Inline) {}

func countWords(s string) int {
	count := 0
	for _, field := range strings.Fields(s) {
		field = strings.TrimFunc(field, func(r rune) bool {
			return unicode.IsPunct(r) || unicode.IsSymbol(r)
		})
		if field != "" {
			count++
		}
	}
	return count
}

func styleCounts(styles []parser.StyleUsage) (total, mapped, unknown int) {
	total = len(styles)
	for _, style := range styles {
		if style.Status == "unknown" {
			unknown++
		} else {
			mapped++
		}
	}
	return total, mapped, unknown
}

func loadCover(cfg *config.Config, title string) (*ir.Image, []ir.Issue, error) {
	if cfg.Document.CoverImage == "" {
		return nil, nil, nil
	}
	data, err := os.ReadFile(cfg.Document.CoverImage)
	if err != nil {
		return nil, nil, err
	}
	alt := cfg.Document.CoverAlt
	var issues []ir.Issue
	if alt == "" {
		alt = title + " cover"
		issues = append(issues, ir.Issue{Severity: "warning", Code: "t-warn-cover-alt", Message: "cover alt text was generated from title"})
	}
	return &ir.Image{Name: filepath.Base(cfg.Document.CoverImage), Data: data, MediaType: mediaTypeFromName(cfg.Document.CoverImage), Alt: alt}, issues, nil
}

func mediaTypeFromName(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	default:
		return "application/octet-stream"
	}
}

func enforceLimits(doc *ir.Document, limits config.LimitsConfig) []ir.Issue {
	var issues []ir.Issue
	blocks, tableCells, maxInlineDepth := 0, 0, 0
	var walkBlock func(ir.Block, int)
	walkBlock = func(block ir.Block, depth int) {
		blocks++
		switch v := block.(type) {
		case ir.Heading:
			maxInlineDepth = max(maxInlineDepth, inlineDepth(v.Inlines, 1))
		case ir.Paragraph:
			maxInlineDepth = max(maxInlineDepth, inlineDepth(v.Inlines, 1))
		case ir.Verse:
			for _, stanza := range v.Stanzas {
				for _, line := range stanza {
					maxInlineDepth = max(maxInlineDepth, inlineDepth(line.Inlines, 1))
				}
			}
			maxInlineDepth = max(maxInlineDepth, inlineDepth(v.Source, 1))
		case ir.Letter:
			for _, child := range v.Children {
				walkBlock(child, depth+1)
			}
		case ir.Epigraph:
			maxInlineDepth = max(maxInlineDepth, inlineDepth(v.Source, 1))
			for _, child := range v.Children {
				walkBlock(child, depth+1)
			}
		case ir.BlockQuote:
			maxInlineDepth = max(maxInlineDepth, inlineDepth(v.Cite, 1))
			for _, child := range v.Children {
				walkBlock(child, depth+1)
			}
		case ir.List:
			for _, item := range v.Items {
				for _, child := range item.Children {
					walkBlock(child, depth+1)
				}
			}
		case ir.Table:
			for _, row := range v.Rows {
				tableCells += len(row.Cells)
				for _, cell := range row.Cells {
					for _, child := range cell.Children {
						walkBlock(child, depth+1)
					}
				}
			}
		}
	}
	for _, block := range doc.Body {
		walkBlock(block, 1)
	}
	if limits.MaxBlocks > 0 && blocks > limits.MaxBlocks {
		issues = append(issues, ir.Issue{Severity: "error", Code: "t-err-limit-blocks", Message: "document exceeds block limit"})
	}
	if limits.MaxFootnotes > 0 && len(doc.Footnotes) > limits.MaxFootnotes {
		issues = append(issues, ir.Issue{Severity: "error", Code: "t-err-limit-footnotes", Message: "document exceeds footnote limit"})
	}
	if limits.MaxTableCells > 0 && tableCells > limits.MaxTableCells {
		issues = append(issues, ir.Issue{Severity: "error", Code: "t-err-limit-table-cells", Message: "document exceeds table cell limit"})
	}
	if limits.MaxInlineDepth > 0 && maxInlineDepth > limits.MaxInlineDepth {
		issues = append(issues, ir.Issue{Severity: "error", Code: "t-err-limit-inline-depth", Message: "document exceeds nesting depth limit"})
	}
	return issues
}

func inlineDepth(inlines []ir.Inline, depth int) int {
	maxDepth := depth
	for _, inline := range inlines {
		switch v := inline.(type) {
		case ir.Styled:
			maxDepth = max(maxDepth, inlineDepth(v.Children, depth+1))
		case ir.Link:
			maxDepth = max(maxDepth, inlineDepth(v.Children, depth+1))
		}
	}
	return maxDepth
}

func validateSourceDateEpoch() error {
	raw := os.Getenv("SOURCE_DATE_EPOCH")
	if raw == "" {
		return nil
	}
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		return fmt.Errorf("invalid SOURCE_DATE_EPOCH: %w", err)
	}
	return nil
}

func filterIssues(issues []ir.Issue, cfg *config.Config) []ir.Issue {
	suppress := map[string]bool{}
	promote := map[string]bool{}
	for _, code := range cfg.Issues.Suppress {
		suppress[code] = true
	}
	for _, code := range cfg.Issues.Promote {
		promote[code] = true
	}
	var out []ir.Issue
	for _, issue := range issues {
		if issue.Severity == "warning" && suppress[issue.Code] {
			continue
		}
		if issue.Severity == "warning" && promote[issue.Code] {
			issue.Severity = "error"
		}
		out = append(out, issue)
	}
	return out
}

func filterFindings(findings []LintFinding, cfg *config.Config) []LintFinding {
	suppress := map[string]bool{}
	promote := map[string]bool{}
	for _, code := range cfg.Issues.Suppress {
		suppress[code] = true
	}
	for _, code := range cfg.Issues.Promote {
		promote[code] = true
	}
	var out []LintFinding
	for _, finding := range findings {
		if finding.Severity == "warning" && suppress[finding.RuleID] {
			continue
		}
		if finding.Severity == "warning" && promote[finding.RuleID] {
			finding.Severity = "error"
		}
		out = append(out, finding)
	}
	return out
}

func hasIssueErrors(issues []ir.Issue) bool {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return true
		}
	}
	return false
}

func firstErrorMessage(issues []ir.Issue) string {
	for _, issue := range issues {
		if issue.Severity == "error" {
			return issue.Code + ": " + issue.Message
		}
	}
	return "validation failed"
}

func lintHasErrors(findings []LintFinding) bool {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return true
		}
	}
	return false
}

func orderArtifacts(artifacts []Artifact) []Artifact {
	rank := map[OutputKind]int{OutputTEX: 0, OutputPDF: 1, OutputEPUB: 2}
	sort.SliceStable(artifacts, func(i, j int) bool {
		return rank[artifacts[i].Kind] < rank[artifacts[j].Kind]
	})
	return artifacts
}

func compilePDF(engine, outDir, base string, result backend.LatexResult) (string, error) {
	if engine != "lualatex" && engine != "xelatex" {
		return "", fmt.Errorf("unsupported TeX engine %q", engine)
	}
	if _, err := exec.LookPath(engine); err != nil {
		return "", fmt.Errorf("%s not found in PATH; install TeX Live or use Docker: %w", engine, err)
	}
	tmp, err := os.MkdirTemp("", "tessera-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	texName := base + ".tex"
	texPath := filepath.Join(tmp, texName)
	if err := os.WriteFile(texPath, []byte(result.TexSource), 0o644); err != nil {
		return "", err
	}
	for _, image := range result.Images {
		if err := os.WriteFile(filepath.Join(tmp, filepath.Base(image.Name)), image.Data, 0o644); err != nil {
			return "", err
		}
	}
	for pass := 0; pass < 2; pass++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		cmd := exec.CommandContext(ctx, engine, "-interaction=nonstopmode", "-halt-on-error", "--no-shell-escape", texName)
		cmd.Dir = tmp
		output, err := cmd.CombinedOutput()
		cancel()
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("t-err-latex-timeout: %s timed out", engine)
		}
		if err != nil {
			return "", fmt.Errorf("%s failed: %w\n%s", engine, err, lastLines(string(output), 40))
		}
	}
	src := filepath.Join(tmp, base+".pdf")
	dst := filepath.Join(outDir, base+".pdf")
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		return "", err
	}
	return dst, nil
}

func lastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

package tessera

import (
	"errors"
	"fmt"

	"github.com/balyakin/tessera/internal/backend"
	"github.com/balyakin/tessera/internal/backend/epub"
	"github.com/balyakin/tessera/internal/backend/latex"
	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/pipeline"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func ParseFile(path string, opts Options) (*ir.Document, []Issue, error) {
	result, err := pipeline.ParseFile(pipeline.ParseFileOptions{
		InputPath:    path,
		ConfigPath:   opts.ConfigPath,
		Metadata:     opts.Metadata,
		StrictStyles: opts.StrictStyles,
	})
	if err != nil {
		return nil, nil, err
	}
	return result.Document, result.Issues, nil
}

func BuildFile(opts BuildOptions) (*BuildResult, error) {
	result, err := pipeline.BuildFile(toPipelineBuildOptions(opts))
	if err != nil {
		return nil, err
	}
	return fromPipelineBuildResult(result), nil
}

func RenderEPUB(doc *ir.Document, opts Options) ([]byte, []Issue, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, nil, err
	}
	result, err := epub.Render(doc, cfg, backend.RenderOptions{Reproducible: opts.Reproducible})
	if err != nil {
		return nil, nil, err
	}
	return result.Bytes, result.Issues, nil
}

func RenderLaTeX(doc *ir.Document, opts Options) (string, []Issue, error) {
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return "", nil, err
	}
	result, err := latex.Render(doc, cfg, backend.RenderOptions{Reproducible: opts.Reproducible})
	if err != nil {
		return "", nil, err
	}
	return result.TexSource, result.Issues, nil
}

func InspectFile(path string, opts Options) (*InspectResult, error) {
	result, err := pipeline.InspectFile(pipeline.ParseFileOptions{
		InputPath:    path,
		ConfigPath:   opts.ConfigPath,
		Metadata:     opts.Metadata,
		StrictStyles: opts.StrictStyles,
	})
	if err != nil {
		return nil, err
	}
	return fromPipelineInspectResult(result), nil
}

func MarshalIR(doc *ir.Document) ([]byte, error) {
	return ir.MarshalCanonical(doc)
}

func UnmarshalIR(data []byte) (*ir.Document, error) {
	return ir.UnmarshalCanonical(data)
}

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var coder interface{ ExitCode() int }
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return 1
}

func toPipelineBuildOptions(opts BuildOptions) pipeline.BuildOptions {
	targets := make([]pipeline.OutputKind, 0, len(opts.Targets))
	for _, target := range opts.Targets {
		targets = append(targets, pipeline.OutputKind(target))
	}
	return pipeline.BuildOptions{
		InputPath:    opts.InputPath,
		OutputDir:    opts.OutputDir,
		ConfigPath:   opts.ConfigPath,
		Metadata:     opts.Metadata,
		Targets:      targets,
		Engine:       opts.Engine,
		KeepTex:      opts.KeepTex,
		Lint:         opts.Lint,
		Reproducible: opts.Reproducible,
		StrictStyles: opts.StrictStyles,
		DumpIRPath:   opts.DumpIRPath,
		OnProgress:   opts.OnProgress,
	}
}

func fromPipelineBuildResult(result *pipeline.BuildResult) *BuildResult {
	artifacts := make([]Artifact, 0, len(result.Artifacts))
	for _, artifact := range result.Artifacts {
		artifacts = append(artifacts, Artifact{Kind: OutputKind(artifact.Kind), Path: artifact.Path})
	}
	findings := make([]LintFinding, 0, len(result.LintFindings))
	for _, finding := range result.LintFindings {
		findings = append(findings, LintFinding(finding))
	}
	return &BuildResult{
		InputPath:     result.InputPath,
		InputFormat:   InputFormat(result.InputFormat),
		Artifacts:     artifacts,
		Issues:        result.Issues,
		LintFindings:  findings,
		Stats:         DocumentStats(result.Stats),
		ElapsedMillis: result.ElapsedMillis,
	}
}

func fromPipelineInspectResult(result *pipeline.InspectResult) *InspectResult {
	pstyles := make([]InspectStyle, 0, len(result.ParagraphStyles))
	for _, style := range result.ParagraphStyles {
		pstyles = append(pstyles, InspectStyle(style))
	}
	cstyles := make([]InspectStyle, 0, len(result.CharacterStyles))
	for _, style := range result.CharacterStyles {
		cstyles = append(cstyles, InspectStyle(style))
	}
	direct := make([]DirectFormattingStat, 0, len(result.DirectFormatting))
	for _, stat := range result.DirectFormatting {
		direct = append(direct, DirectFormattingStat(stat))
	}
	return &InspectResult{
		InputPath:        result.InputPath,
		InputFormat:      InputFormat(result.InputFormat),
		Metadata:         result.Metadata,
		Stats:            DocumentStats(result.Stats),
		ParagraphStyles:  pstyles,
		CharacterStyles:  cstyles,
		DirectFormatting: direct,
		Issues:           result.Issues,
	}
}

func FormatError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%v", err)
}

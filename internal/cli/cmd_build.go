package cli

import (
	"fmt"
	"strings"

	"github.com/balyakin/tessera/pkg/tessera"
	"github.com/spf13/cobra"
)

func newBuildCommand(g *globalOptions) *cobra.Command {
	var (
		to           string
		output       string
		configPath   string
		metadata     []string
		lintEPUB     bool
		keepTex      bool
		engine       string
		reproducible bool
		stats        bool
		strictStyles bool
		dumpIR       string
		epubcheck    string
	)
	cmd := &cobra.Command{
		Use:   "build <input>",
		Short: "Build PDF, EPUB, or LaTeX artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := parseTargets(to)
			if err != nil {
				return exitError{code: 3, err: err}
			}
			result, err := tessera.BuildFile(tessera.BuildOptions{
				InputPath:    args[0],
				OutputDir:    output,
				ConfigPath:   configPath,
				Metadata:     parseMetadata(metadata),
				Targets:      targets,
				Engine:       engine,
				KeepTex:      keepTex,
				Lint:         lintEPUB,
				Reproducible: reproducible,
				StrictStyles: strictStyles,
				DumpIRPath:   dumpIR,
				OnProgress: func(phase, detail string, percent float64) {
					if showProgress(g, cmd.ErrOrStderr()) {
						fmt.Fprintf(cmd.ErrOrStderr(), "%s  %s\n", phase, detail)
					}
				},
			})
			if err != nil {
				return err
			}
			if lintEPUB {
				if err := runEPUBCheck(epubcheck, epubArtifactPath(result)); err != nil {
					return err
				}
			}
			if g.format == "json" {
				return writeJSON(cmd.OutOrStdout(), buildJSON(result))
			}
			printBuildHuman(cmd, result, stats)
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "all", "pdf, epub, tex, or all")
	cmd.Flags().StringVarP(&output, "output", "o", ".", "output directory")
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "config TOML path")
	cmd.Flags().StringArrayVarP(&metadata, "metadata", "m", nil, "metadata key=value")
	cmd.Flags().BoolVar(&lintEPUB, "lint", false, "lint generated EPUB")
	cmd.Flags().StringVar(&epubcheck, "epubcheck", "auto", "auto, always, or never")
	cmd.Flags().BoolVar(&keepTex, "keep-tex", false, "keep generated TeX next to PDF")
	cmd.Flags().StringVar(&engine, "engine", "lualatex", "lualatex or xelatex")
	cmd.Flags().BoolVar(&reproducible, "reproducible", false, "deterministic timestamps and ordering")
	cmd.Flags().BoolVar(&stats, "stats", false, "include document statistics")
	cmd.Flags().BoolVar(&strictStyles, "strict-styles", false, "disable normalized style fallback")
	cmd.Flags().StringVar(&dumpIR, "dump-ir", "", "write canonical IR JSON")
	return cmd
}

func epubArtifactPath(result *tessera.BuildResult) string {
	for _, artifact := range result.Artifacts {
		if artifact.Kind == tessera.OutputEPUB {
			return artifact.Path
		}
	}
	return ""
}

func parseTargets(raw string) ([]tessera.OutputKind, error) {
	switch raw {
	case "all":
		return []tessera.OutputKind{tessera.OutputPDF, tessera.OutputEPUB}, nil
	case "pdf":
		return []tessera.OutputKind{tessera.OutputPDF}, nil
	case "epub":
		return []tessera.OutputKind{tessera.OutputEPUB}, nil
	case "tex":
		return []tessera.OutputKind{tessera.OutputTEX}, nil
	default:
		return nil, fmt.Errorf("invalid --to value %q", raw)
	}
}

func parseMetadata(values []string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		out[key] = val
	}
	return out
}

func buildJSON(result *tessera.BuildResult) map[string]any {
	return map[string]any{
		"command":       "build",
		"success":       true,
		"input":         result.InputPath,
		"format":        result.InputFormat,
		"artifacts":     result.Artifacts,
		"stats":         result.Stats,
		"issues":        nonNilIssues(result.Issues),
		"lint_findings": nonNilLintFindings(result.LintFindings),
		"elapsed_ms":    result.ElapsedMillis,
	}
}

func printBuildHuman(cmd *cobra.Command, result *tessera.BuildResult, stats bool) {
	fmt.Fprintln(cmd.ErrOrStderr(), "Build complete")
	fmt.Fprintln(cmd.ErrOrStderr(), "\nArtifacts")
	for _, artifact := range result.Artifacts {
		fmt.Fprintf(cmd.ErrOrStderr(), "  %-4s %s\n", strings.ToUpper(string(artifact.Kind)), artifact.Path)
	}
	if stats {
		fmt.Fprintln(cmd.ErrOrStderr(), "\nDocument Statistics")
		fmt.Fprintf(cmd.ErrOrStderr(), "  Words      %d\n", result.Stats.Words)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Chapters   %d\n", result.Stats.Chapters)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Footnotes  %d\n", result.Stats.Footnotes)
		fmt.Fprintf(cmd.ErrOrStderr(), "  Images     %d\n", result.Stats.Images)
	}
	if len(result.Issues) > 0 {
		fmt.Fprintln(cmd.ErrOrStderr(), "\nWarnings")
		for _, issue := range result.Issues {
			fmt.Fprintf(cmd.ErrOrStderr(), "  %s  %s\n", issue.Code, issue.Message)
		}
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "\nElapsed: %.2fs\n", float64(result.ElapsedMillis)/1000)
}

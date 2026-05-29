package cli

import (
	"fmt"
	"os"
	"time"

	epublint "github.com/balyakin/tessera/internal/backend/epub/lint"
	"github.com/spf13/cobra"
)

func newLintCommand(g *globalOptions) *cobra.Command {
	var fix bool
	var output string
	var epubcheck string
	cmd := &cobra.Command{
		Use:   "lint <input.epub>",
		Short: "Run the built-in EPUB linter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			findings := epublint.Lint(data)
			fixed := false
			checkPath := args[0]
			if fix {
				fixedData, fixedFindings, modified, err := epublint.Fix(data, time.Now().UTC())
				if err != nil {
					return err
				}
				if modified {
					if output == "" {
						return exitError{code: 3, err: fmt.Errorf("--output is required because --fix would modify content")}
					}
					if err := os.WriteFile(output, fixedData, 0o644); err != nil {
						return err
					}
					checkPath = output
					fixed = true
				} else if output != "" {
					if err := os.WriteFile(output, fixedData, 0o644); err != nil {
						return err
					}
					checkPath = output
				}
				findings = fixedFindings
			}
			if g.format == "json" {
				if err := writeJSON(cmd.OutOrStdout(), map[string]any{
					"command":  "lint",
					"success":  !epublint.HasErrors(findings),
					"input":    args[0],
					"findings": nonNilEPUBFindings(findings),
					"fixed":    fixed,
					"output":   output,
				}); err != nil {
					return err
				}
			} else {
				for _, finding := range findings {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s  %s  %s\n", finding.RuleID, finding.Severity, finding.Message)
				}
				if len(findings) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), "No EPUB lint findings")
				}
			}
			if epublint.HasErrors(findings) {
				return exitError{code: 5, err: fmt.Errorf("EPUB linter found errors")}
			}
			if err := runEPUBCheck(epubcheck, checkPath); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "write fixed EPUB when possible")
	cmd.Flags().StringVarP(&output, "output", "o", "", "fixed EPUB output path")
	cmd.Flags().StringVar(&epubcheck, "epubcheck", "auto", "auto, always, or never")
	return cmd
}

func nonNilEPUBFindings(values []epublint.Finding) []epublint.Finding {
	if values == nil {
		return []epublint.Finding{}
	}
	return values
}

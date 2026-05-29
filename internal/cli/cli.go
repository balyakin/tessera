package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/balyakin/tessera/pkg/tessera"
	"github.com/spf13/cobra"
)

var (
	Version   = "0.1.0"
	Commit    = "unknown"
	BuildDate = "unknown"
)

type globalOptions struct {
	format  string
	color   string
	quiet   bool
	verbose bool
}

type exitError struct {
	code int
	err  error
}

func (e exitError) Error() string { return e.err.Error() }
func (e exitError) Unwrap() error { return e.err }

func Execute() int {
	root := NewRootCommand(os.Stdout, os.Stderr)
	if err := root.Execute(); err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			fmt.Fprintln(os.Stderr, ee.err)
			return ee.code
		}
		code := tessera.ExitCode(err)
		fmt.Fprintln(os.Stderr, err)
		return code
	}
	return 0
}

func NewRootCommand(stdout, stderr io.Writer) *cobra.Command {
	g := &globalOptions{}
	root := &cobra.Command{
		Use:           "tessera",
		Short:         "Semantic book publishing from DOCX/ODT to PDF and EPUB",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&g.format, "format", "human", "human or json")
	root.PersistentFlags().StringVar(&g.color, "color", "auto", "auto, always, or never")
	root.PersistentFlags().BoolVar(&g.quiet, "quiet", false, "suppress non-error human progress")
	root.PersistentFlags().BoolVarP(&g.verbose, "verbose", "v", false, "show debug details")
	root.AddCommand(
		newBuildCommand(g),
		newLintCommand(g),
		newInitCommand(g),
		newInspectCommand(g),
		newDoctorCommand(g),
		newDemoCommand(g),
		newCompletionCommand(),
		newVersionCommand(g),
	)
	return root
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func human(g *globalOptions) bool {
	return g.format != "json"
}

func showProgress(g *globalOptions, w io.Writer) bool {
	if !human(g) || g.quiet || os.Getenv("CI") != "" {
		return false
	}
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func versionPayload() map[string]string {
	return map[string]string{
		"version":    Version,
		"commit":     Commit,
		"build_date": BuildDate,
		"go_version": runtime.Version(),
	}
}

func nonNilIssues(values []tessera.Issue) []tessera.Issue {
	if values == nil {
		return []tessera.Issue{}
	}
	return values
}

func nonNilLintFindings(values []tessera.LintFinding) []tessera.LintFinding {
	if values == nil {
		return []tessera.LintFinding{}
	}
	return values
}

func nonNilInspectStyles(values []tessera.InspectStyle) []tessera.InspectStyle {
	if values == nil {
		return []tessera.InspectStyle{}
	}
	return values
}

func nonNilDirectStats(values []tessera.DirectFormattingStat) []tessera.DirectFormattingStat {
	if values == nil {
		return []tessera.DirectFormattingStat{}
	}
	return values
}

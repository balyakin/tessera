package cli

import (
	"fmt"
	"os"

	"github.com/balyakin/tessera/pkg/tessera"
	"github.com/spf13/cobra"
)

func newInspectCommand(g *globalOptions) *cobra.Command {
	var configPath string
	var strictStyles bool
	var irDump string
	var metadata []string
	cmd := &cobra.Command{
		Use:   "inspect <input>",
		Short: "Inspect source metadata and named styles",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if irDump != "" {
				doc, _, err := tessera.ParseFile(args[0], tessera.Options{ConfigPath: configPath, StrictStyles: strictStyles, Metadata: parseMetadata(metadata)})
				if err != nil {
					return err
				}
				data, err := tessera.MarshalIR(doc)
				if err != nil {
					return err
				}
				if err := os.WriteFile(irDump, data, 0o644); err != nil {
					return err
				}
			}
			result, err := tessera.InspectFile(args[0], tessera.Options{ConfigPath: configPath, StrictStyles: strictStyles, Metadata: parseMetadata(metadata)})
			if err != nil {
				return err
			}
			if g.format == "json" {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"command":           "inspect",
					"input":             result.InputPath,
					"format":            result.InputFormat,
					"metadata":          result.Metadata,
					"stats":             result.Stats,
					"paragraph_styles":  nonNilInspectStyles(result.ParagraphStyles),
					"character_styles":  nonNilInspectStyles(result.CharacterStyles),
					"direct_formatting": nonNilDirectStats(result.DirectFormatting),
					"issues":            nonNilIssues(result.Issues),
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Input: %s (%s)\n", result.InputPath, result.InputFormat)
			fmt.Fprintf(cmd.ErrOrStderr(), "Title: %s\n", result.Metadata.Title)
			fmt.Fprintln(cmd.ErrOrStderr(), "\nParagraph styles")
			for _, style := range result.ParagraphStyles {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %-24s %-9s %s\n", style.Name, style.Status, style.Role)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "\nCharacter styles")
			for _, style := range result.CharacterStyles {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %-24s %-9s %s\n", style.Name, style.Status, style.Role)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&configPath, "config", "c", "", "config TOML path")
	cmd.Flags().StringArrayVarP(&metadata, "metadata", "m", nil, "metadata key=value")
	cmd.Flags().BoolVar(&strictStyles, "strict-styles", false, "disable normalized style fallback")
	cmd.Flags().StringVar(&irDump, "ir-dump", "", "write canonical IR JSON")
	return cmd
}

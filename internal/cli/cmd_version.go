package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCommand(g *globalOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print Tessera version",
		RunE: func(cmd *cobra.Command, args []string) error {
			if g.format == "json" {
				return writeJSON(cmd.OutOrStdout(), versionPayload())
			}
			v := versionPayload()
			fmt.Fprintf(cmd.OutOrStdout(), "Tessera %s\ncommit: %s\nbuild date: %s\ngo: %s\n", v["version"], v["commit"], v["build_date"], v["go_version"])
			return nil
		},
	}
}

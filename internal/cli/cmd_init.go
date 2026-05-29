package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/demo"
	"github.com/spf13/cobra"
)

func newInitCommand(g *globalOptions) *cobra.Command {
	var output string
	var force bool
	var examples bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a Tessera config file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(output); err == nil && !force {
				return exitError{code: 1, err: fmt.Errorf("%s already exists; use --force", output)}
			}
			if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil && filepath.Dir(output) != "." {
				return err
			}
			if err := os.WriteFile(output, []byte(config.DefaultTOML()), 0o644); err != nil {
				return err
			}
			if examples {
				if err := demo.WriteDemoFiles(filepath.Join(filepath.Dir(output), "examples")); err != nil {
					return err
				}
			}
			if human(g) {
				fmt.Fprintf(cmd.ErrOrStderr(), "Wrote %s\n", output)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "tessera.toml", "config file path")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing config")
	cmd.Flags().BoolVar(&examples, "examples", false, "copy example manuscripts next to config")
	return cmd
}

package cli

import (
	"fmt"
	"path/filepath"

	"github.com/balyakin/tessera/internal/demo"
	"github.com/balyakin/tessera/pkg/tessera"
	"github.com/spf13/cobra"
)

func newDemoCommand(g *globalOptions) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "demo",
		Short: "Build the embedded semantic demo to EPUB",
		RunE: func(cmd *cobra.Command, args []string) error {
			sourceDir := filepath.Join(output, "source")
			distDir := filepath.Join(output, "dist")
			if err := demo.WriteDemoFiles(sourceDir); err != nil {
				return err
			}
			result, err := tessera.BuildFile(tessera.BuildOptions{
				InputPath:    filepath.Join(sourceDir, demo.DOCXName),
				OutputDir:    distDir,
				Targets:      []tessera.OutputKind{tessera.OutputEPUB},
				Lint:         true,
				Reproducible: true,
				StrictStyles: false,
			})
			if err != nil {
				return err
			}
			if g.format == "json" {
				return writeJSON(cmd.OutOrStdout(), buildJSON(result))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Demo EPUB: %s\n", filepath.Join(distDir, "semantic-demo.epub"))
			fmt.Fprintln(cmd.ErrOrStderr(), `PDF with Docker: docker run --rm -v "$PWD:/work" ghcr.io/balyakin/tessera:latest build /work/tessera-demo/source/semantic-demo.docx --output /work/tessera-demo/dist --lint`)
			return nil
		},
	}
	cmd.Flags().StringVarP(&output, "output", "o", "tessera-demo", "demo output directory")
	return cmd
}

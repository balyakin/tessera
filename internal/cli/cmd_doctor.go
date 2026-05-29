package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

func newDoctorCommand(g *globalOptions) *cobra.Command {
	var to string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check local Tessera environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := []doctorCheck{
				toolCheck("lualatex"),
				toolCheck("xelatex"),
				toolCheck("kpsewhich"),
				toolCheck("epubcheck"),
			}
			checks = append(checks, latexPackageChecks()...)
			checks = append(checks, latexFontChecks()...)
			var issues []string
			var next []string
			if (to == "auto" || to == "pdf" || to == "tex" || to == "all") && checks[0].Status == "missing" {
				next = append(next, `docker run --rm -v "$PWD:/work" ghcr.io/balyakin/tessera:latest build /work/examples/semantic-demo.docx --output /work/dist --lint`)
			}
			if (to == "pdf" || to == "tex" || to == "all") && checks[0].Status == "missing" {
				issues = append(issues, "lualatex is required for PDF output")
			}
			if to == "pdf" || to == "tex" || to == "all" {
				for _, check := range checks {
					if check.Status == "missing" && isRequiredLatexPackage(check.Name) {
						issues = append(issues, check.Name+" is required for PDF output")
					}
				}
			}
			success := len(issues) == 0
			if g.format == "json" {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"command":    "doctor",
					"success":    success,
					"version":    Version,
					"os":         runtime.GOOS,
					"arch":       runtime.GOARCH,
					"checks":     checks,
					"issues":     nonNilStrings(issues),
					"next_steps": nonNilStrings(next),
				})
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Tessera %s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
			for _, check := range checks {
				fmt.Fprintf(cmd.ErrOrStderr(), "  %-10s %-8s %s\n", check.Name, check.Status, check.Detail)
			}
			if len(next) > 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "\nNext step")
				for _, step := range next {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s\n", step)
				}
			}
			if !success {
				return exitError{code: 4, err: fmt.Errorf("required dependency missing")}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&to, "to", "auto", "auto, pdf, epub, tex, or all")
	return cmd
}

func latexFontChecks() []doctorCheck {
	engine, err := exec.LookPath("lualatex")
	if err != nil {
		return nil
	}
	fonts := []string{"EB Garamond", "Latin Modern Roman"}
	checks := make([]doctorCheck, 0, len(fonts))
	for _, font := range fonts {
		checks = append(checks, latexFontCheck(engine, font))
	}
	return checks
}

func latexFontCheck(engine, font string) doctorCheck {
	dir, err := os.MkdirTemp("", "tessera-doctor-*")
	if err != nil {
		return doctorCheck{Name: "font:" + font, Status: "error", Detail: err.Error()}
	}
	defer os.RemoveAll(dir)
	tex := fmt.Sprintf("\\documentclass{article}\n\\usepackage{fontspec}\n\\setmainfont{%s}\n\\begin{document}ok\\end{document}\n", font)
	texPath := filepath.Join(dir, "fontcheck.tex")
	if err := os.WriteFile(texPath, []byte(tex), 0o644); err != nil {
		return doctorCheck{Name: "font:" + font, Status: "error", Detail: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, engine, "-interaction=nonstopmode", "-halt-on-error", "--no-shell-escape", "fontcheck.tex")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return doctorCheck{Name: "font:" + font, Status: "warning", Detail: "font check timed out"}
	}
	if err != nil {
		return doctorCheck{Name: "font:" + font, Status: "warning", Detail: doctorOutputSummary(out)}
	}
	return doctorCheck{Name: "font:" + font, Status: "ok", Detail: "available"}
}

func doctorOutputSummary(out []byte) string {
	text := string(bytesTrimSpace(out))
	if len(text) > 160 {
		return text[len(text)-160:]
	}
	if text == "" {
		return "not available"
	}
	return text
}

func latexPackageChecks() []doctorCheck {
	kpsewhich, err := exec.LookPath("kpsewhich")
	if err != nil {
		return nil
	}
	packages := []string{"memoir", "fontspec", "polyglossia", "microtype", "graphicx", "csquotes", "hyperref", "unicode-math", "xcolor"}
	checks := make([]doctorCheck, 0, len(packages))
	for _, pkg := range packages {
		cmd := exec.Command(kpsewhich, pkg+".sty")
		out, err := cmd.Output()
		if err != nil {
			checks = append(checks, doctorCheck{Name: "latex:" + pkg, Status: "missing", Detail: "not found"})
			continue
		}
		checks = append(checks, doctorCheck{Name: "latex:" + pkg, Status: "ok", Detail: string(bytesTrimSpace(out))})
	}
	return checks
}

func bytesTrimSpace(b []byte) []byte {
	for len(b) > 0 && (b[0] == ' ' || b[0] == '\n' || b[0] == '\r' || b[0] == '\t') {
		b = b[1:]
	}
	for len(b) > 0 && (b[len(b)-1] == ' ' || b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == '\t') {
		b = b[:len(b)-1]
	}
	return b
}

func isRequiredLatexPackage(name string) bool {
	return len(name) > len("latex:") && name[:len("latex:")] == "latex:"
}

func toolCheck(name string) doctorCheck {
	path, err := exec.LookPath(name)
	if err != nil {
		return doctorCheck{Name: name, Status: "missing", Detail: "not found"}
	}
	return doctorCheck{Name: name, Status: "ok", Detail: path}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

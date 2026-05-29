package cli

import (
	"bytes"
	"fmt"
	"os/exec"
)

func runEPUBCheck(mode, epubPath string) error {
	switch mode {
	case "never", "":
		return nil
	case "auto", "always":
	default:
		return exitError{code: 3, err: fmt.Errorf("invalid --epubcheck value %q", mode)}
	}
	path, err := exec.LookPath("epubcheck")
	if err != nil {
		if mode == "always" {
			return exitError{code: 4, err: fmt.Errorf("epubcheck not found in PATH")}
		}
		return nil
	}
	cmd := exec.Command(path, epubPath)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return exitError{code: 5, err: fmt.Errorf("epubcheck failed: %w\n%s", err, output.String())}
	}
	return nil
}

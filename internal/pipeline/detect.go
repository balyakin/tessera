package pipeline

import (
	"archive/zip"
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

func DetectFormat(path string, data []byte) (InputFormat, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".odt":
		return FormatODT, nil
	case ".docx":
		return FormatDOCX, nil
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("detect input format: %w", err)
	}
	hasODT, hasDOCX := false, false
	for _, file := range zr.File {
		switch file.Name {
		case "content.xml":
			hasODT = true
		case "word/document.xml":
			hasDOCX = true
		}
	}
	if hasODT {
		return FormatODT, nil
	}
	if hasDOCX {
		return FormatDOCX, nil
	}
	return "", fmt.Errorf("detect input format: unsupported document")
}

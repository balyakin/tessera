package common

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

const (
	MaxArchiveBytes = 512 << 20
	MaxXMLBytes     = 128 << 20
	MaxImageBytes   = 64 << 20
)

func ReadAllReaderAt(reader io.ReaderAt, size int64) ([]byte, error) {
	if size > MaxArchiveBytes {
		return nil, fmt.Errorf("archive exceeds %d bytes", MaxArchiveBytes)
	}
	data := make([]byte, size)
	_, err := reader.ReadAt(data, 0)
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

func OpenZip(reader io.ReaderAt, size int64) (*zip.Reader, error) {
	zr, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}
	var total uint64
	for _, file := range zr.File {
		if unsafeArchivePath(file.Name) {
			return nil, fmt.Errorf("unsafe archive path %q", file.Name)
		}
		total += file.UncompressedSize64
		if total > MaxArchiveBytes {
			return nil, fmt.Errorf("archive uncompressed size exceeds %d bytes", MaxArchiveBytes)
		}
		if file.CompressedSize64 > 0 && file.UncompressedSize64/file.CompressedSize64 > 100 && !looksImage(file.Name) {
			return nil, fmt.Errorf("archive entry %q has suspicious compression ratio", file.Name)
		}
	}
	return zr, nil
}

func ReadZipEntry(zr *zip.Reader, name string, limit int64) ([]byte, bool, error) {
	for _, file := range zr.File {
		if file.Name != name {
			continue
		}
		if file.UncompressedSize64 > uint64(limit) {
			return nil, true, fmt.Errorf("entry %q exceeds %d bytes", name, limit)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, true, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		if err != nil {
			return nil, true, err
		}
		if int64(len(data)) > limit {
			return nil, true, fmt.Errorf("entry %q exceeds %d bytes", name, limit)
		}
		if containsDTDOrEntity(data) {
			return nil, true, fmt.Errorf("entry %q contains DTD or entity declarations", name)
		}
		return data, true, nil
	}
	return nil, false, nil
}

func ReadZipPrefix(zr *zip.Reader, prefix string, limit int64) (map[string][]byte, error) {
	out := map[string][]byte{}
	for _, file := range zr.File {
		if !strings.HasPrefix(file.Name, prefix) || strings.HasSuffix(file.Name, "/") {
			continue
		}
		if file.UncompressedSize64 > uint64(limit) {
			return nil, fmt.Errorf("entry %q exceeds %d bytes", file.Name, limit)
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(rc, limit+1))
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > limit {
			return nil, fmt.Errorf("entry %q exceeds %d bytes", file.Name, limit)
		}
		if isNestedZip(data) {
			return nil, fmt.Errorf("entry %q looks like nested zip", file.Name)
		}
		out[file.Name] = data
	}
	return out, nil
}

func unsafeArchivePath(name string) bool {
	normalized := strings.ReplaceAll(name, `\`, "/")
	clean := path.Clean(normalized)
	return strings.HasPrefix(normalized, "/") || strings.HasPrefix(clean, "../") || clean == ".." || strings.Contains(clean, "/../")
}

func looksImage(name string) bool {
	name = strings.ToLower(name)
	return strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") ||
		strings.HasSuffix(name, ".gif") || strings.HasSuffix(name, ".svg")
}

func isNestedZip(data []byte) bool {
	return len(data) >= 4 && data[0] == 'P' && data[1] == 'K' && data[2] == 3 && data[3] == 4
}

func containsDTDOrEntity(data []byte) bool {
	upper := bytes.ToUpper(data)
	if bytes.Contains(upper, []byte("<!DOCTYPE")) || bytes.Contains(upper, []byte("<!ENTITY")) {
		return true
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return false
		}
		if err != nil {
			return false
		}
		if directive, ok := token.(xml.Directive); ok {
			text := strings.ToUpper(string(directive))
			if strings.Contains(text, "DOCTYPE") || strings.Contains(text, "ENTITY") {
				return true
			}
		}
	}
}

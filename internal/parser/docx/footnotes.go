package docx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser/common"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

func parseFootnotes(data []byte, cfg *config.Config, styles styleInfo, rels map[string]string, images map[string][]byte, strict bool) (map[string][]ir.Block, error) {
	out := map[string][]ir.Block{}
	if len(data) == 0 {
		return out, nil
	}
	p := newDocumentParser(cfg, styles, rels, images, nil, strict)
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || !docxName(start.Name, docxNSW, "footnote") {
			continue
		}
		id := common.Attr(start, "id")
		if id == "-1" || id == "0" {
			if err := common.SkipElement(dec, start); err != nil {
				return nil, err
			}
			continue
		}
		blocks, err := p.parseFootnoteBody(dec, start)
		if err != nil {
			return nil, err
		}
		out[id] = blocks
	}
}

func normalizedFootnoteID(source string, seen map[string]int, seq *int) string {
	source = strings.TrimSpace(source)
	if source != "" {
		var clean []rune
		for _, r := range source {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			id := "fn-" + strings.ToLower(string(clean))
			seen[id]++
			if seen[id] > 1 {
				return fmt.Sprintf("%s-%d", id, seen[id])
			}
			return id
		}
	}
	*seq = *seq + 1
	return fmt.Sprintf("fn-%04d", *seq)
}

func readMedia(zr *zip.Reader) (map[string][]byte, error) {
	return common.ReadZipPrefix(zr, "word/media/", common.MaxImageBytes)
}

package docx

import (
	"bytes"
	"encoding/xml"
	"io"

	"github.com/balyakin/tessera/internal/parser/common"
)

type styleInfo struct {
	Paragraph map[string]common.StyleDef
	Character map[string]common.StyleDef
}

func parseStyles(data []byte) (styleInfo, error) {
	info := styleInfo{Paragraph: map[string]common.StyleDef{}, Character: map[string]common.StyleDef{}}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return info, nil
		}
		if err != nil {
			return info, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || !docxName(start.Name, docxNSW, "style") {
			continue
		}
		styleID := common.Attr(start, "styleId")
		styleType := common.Attr(start, "type")
		def := common.StyleDef{ID: styleID, DisplayName: styleID}
		if err := parseStyleChildren(dec, start, &def); err != nil {
			return info, err
		}
		switch styleType {
		case "paragraph":
			info.Paragraph[styleID] = def
		case "character":
			info.Character[styleID] = def
		}
	}
}

func parseStyleChildren(dec *xml.Decoder, parent xml.StartElement, def *common.StyleDef) error {
	depth := 1
	for depth > 0 {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			switch {
			case docxName(t.Name, docxNSW, "name"):
				if value := common.Attr(t, "val"); value != "" {
					def.DisplayName = value
				}
			case docxName(t.Name, docxNSW, "basedOn"):
				def.ParentID = common.Attr(t, "val")
			}
		case xml.EndElement:
			depth--
			if depth == 0 && t.Name.Local == parent.Name.Local && t.Name.Space == parent.Name.Space {
				return nil
			}
		}
	}
	return nil
}

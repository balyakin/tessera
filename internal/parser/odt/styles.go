package odt

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"

	"github.com/balyakin/tessera/internal/parser/common"
)

type styleInfo struct {
	Paragraph map[string]common.StyleDef
	Character map[string]common.StyleDef
	PageBreak map[string]bool
	Bold      map[string]bool
	Italic    map[string]bool
}

const odtNSStyle = "urn:oasis:names:tc:opendocument:xmlns:style:1.0"

func parseStyles(data []byte) (styleInfo, error) {
	info := styleInfo{
		Paragraph: map[string]common.StyleDef{},
		Character: map[string]common.StyleDef{},
		PageBreak: map[string]bool{},
		Bold:      map[string]bool{},
		Italic:    map[string]bool{},
	}
	if len(data) == 0 {
		return info, nil
	}
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
		if !ok || start.Name.Space != odtNSStyle || start.Name.Local != "style" {
			continue
		}
		family := common.Attr(start, "family")
		name := common.Attr(start, "name")
		display := common.Attr(start, "display-name")
		if display == "" {
			display = name
		}
		parent := common.Attr(start, "parent-style-name")
		def := common.StyleDef{ID: name, DisplayName: display, ParentID: parent}
		switch family {
		case "paragraph":
			info.Paragraph[name] = def
		case "text":
			info.Character[name] = def
		}
		if err := parseStyleChildren(dec, start, family, name, &info); err != nil {
			return info, err
		}
	}
}

func parseStyleChildren(dec *xml.Decoder, parent xml.StartElement, family, name string, info *styleInfo) error {
	depth := 1
	for depth > 0 {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch t := token.(type) {
		case xml.StartElement:
			depth++
			if family == "paragraph" && t.Name.Space == odtNSStyle && t.Name.Local == "paragraph-properties" && common.AttrNS(t, "urn:oasis:names:tc:opendocument:xmlns:xsl-fo-compatible:1.0", "break-before") == "page" {
				info.PageBreak[name] = true
			}
			if t.Name.Space == odtNSStyle && t.Name.Local == "text-properties" {
				weight := common.Attr(t, "font-weight")
				style := common.Attr(t, "font-style")
				if strings.EqualFold(weight, "bold") {
					info.Bold[name] = true
				}
				if strings.EqualFold(style, "italic") || strings.EqualFold(style, "oblique") {
					info.Italic[name] = true
				}
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

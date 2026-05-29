package common

import (
	"encoding/xml"
	"io"
)

func Attr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func AttrNS(start xml.StartElement, space, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local && attr.Name.Space == space {
			return attr.Value
		}
	}
	return ""
}

func SkipElement(dec *xml.Decoder, start xml.StartElement) error {
	depth := 1
	for depth > 0 {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func ReadTextElement(dec *xml.Decoder, start xml.StartElement) (string, error) {
	var out string
	for {
		token, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := token.(type) {
		case xml.CharData:
			out += string(t)
		case xml.StartElement:
			if err := SkipElement(dec, t); err != nil {
				return out, err
			}
		case xml.EndElement:
			if t.Name.Local == start.Name.Local && t.Name.Space == start.Name.Space {
				return out, nil
			}
		}
	}
}

func Drain(dec *xml.Decoder) error {
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/balyakin/tessera/internal/config"
	"github.com/balyakin/tessera/internal/parser"
	"github.com/balyakin/tessera/internal/parser/common"
	"github.com/balyakin/tessera/pkg/tessera/ir"
)

type Parser struct{}

const docxNSRel = "http://schemas.openxmlformats.org/package/2006/relationships"

func (Parser) Parse(reader io.ReaderAt, size int64, cfg *config.Config, opts parser.ParseOptions) (*parser.ParseResult, error) {
	raw, err := common.ReadAllReaderAt(reader, size)
	if err != nil {
		return nil, fmt.Errorf("read DOCX: %w", err)
	}
	zr, err := common.OpenZip(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	documentXML, ok, err := common.ReadZipEntry(zr, "word/document.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("DOCX missing word/document.xml")
	}
	stylesXML, ok, err := common.ReadZipEntry(zr, "word/styles.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("DOCX missing word/styles.xml")
	}
	relsXML, _, err := common.ReadZipEntry(zr, "word/_rels/document.xml.rels", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	coreXML, _, err := common.ReadZipEntry(zr, "docProps/core.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	numberingXML, _, err := common.ReadZipEntry(zr, "word/numbering.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	footnotesXML, _, err := common.ReadZipEntry(zr, "word/footnotes.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	styles, err := parseStyles(stylesXML)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX styles: %w", err)
	}
	rels, err := parseRels(relsXML)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX relationships: %w", err)
	}
	media, err := readMedia(zr)
	if err != nil {
		return nil, err
	}
	numbering, err := parseNumbering(numberingXML)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX numbering: %w", err)
	}
	footnoteMap, err := parseFootnotes(footnotesXML, cfg, styles, rels, media, opts.StrictStyles)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX footnotes: %w", err)
	}
	sourceMeta, err := parseCore(coreXML)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX metadata: %w", err)
	}
	meta, err := common.ResolveMetadata(raw, opts.InputPath, sourceMeta, cfg, opts.Metadata)
	if err != nil {
		return nil, err
	}
	body, footnotes, issues, report, err := parseDocument(documentXML, cfg, styles, rels, media, numbering, footnoteMap, opts.StrictStyles)
	if err != nil {
		return nil, fmt.Errorf("parse DOCX document: %w", err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      meta,
		Body:      body,
		Footnotes: footnotes,
	}
	return &parser.ParseResult{Document: doc, Issues: issues, StyleReport: report}, nil
}

func parseRels(data []byte) (map[string]string, error) {
	rels := map[string]string{}
	if len(data) == 0 {
		return rels, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return rels, nil
		}
		if err != nil {
			return nil, err
		}
		start, ok := token.(xml.StartElement)
		if !ok || !docxName(start.Name, docxNSRel, "Relationship") {
			continue
		}
		rels[common.Attr(start, "Id")] = common.Attr(start, "Target")
	}
}

func parseCore(data []byte) (ir.Metadata, error) {
	meta := ir.Metadata{Extra: map[string]string{}}
	if len(data) == 0 {
		return meta, nil
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := dec.Token()
		if err == io.EOF {
			return meta, nil
		}
		if err != nil {
			return meta, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		switch {
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "title":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			meta.Title = strings.TrimSpace(value)
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "creator":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			meta.Author = strings.TrimSpace(value)
		case start.Name.Space == "http://purl.org/dc/terms/" && start.Name.Local == "created":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			meta.Date = strings.TrimSpace(value)
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "description":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			meta.Description = strings.TrimSpace(value)
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "language":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			meta.Language = strings.TrimSpace(value)
		}
	}
}

func parseNumbering(data []byte) (map[string]bool, error) {
	out := map[string]bool{}
	if len(data) == 0 {
		return out, nil
	}
	abstractFmt := map[string]string{}
	numToAbstract := map[string]string{}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var currentAbstract, currentNum string
	for {
		token, err := dec.Token()
		if err == io.EOF {
			for numID, abstractID := range numToAbstract {
				out[numID] = abstractFmt[abstractID] == "decimal"
			}
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		switch t := token.(type) {
		case xml.StartElement:
			switch {
			case docxName(t.Name, docxNSW, "abstractNum"):
				currentAbstract = common.Attr(t, "abstractNumId")
			case docxName(t.Name, docxNSW, "numFmt"):
				if currentAbstract != "" {
					abstractFmt[currentAbstract] = common.Attr(t, "val")
				}
			case docxName(t.Name, docxNSW, "num"):
				currentNum = common.Attr(t, "numId")
			case docxName(t.Name, docxNSW, "abstractNumId"):
				if currentNum != "" {
					numToAbstract[currentNum] = common.Attr(t, "val")
				}
			}
		case xml.EndElement:
			switch {
			case docxName(t.Name, docxNSW, "abstractNum"):
				currentAbstract = ""
			case docxName(t.Name, docxNSW, "num"):
				currentNum = ""
			}
		}
	}
}

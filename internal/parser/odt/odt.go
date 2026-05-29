package odt

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

func (Parser) Parse(reader io.ReaderAt, size int64, cfg *config.Config, opts parser.ParseOptions) (*parser.ParseResult, error) {
	raw, err := common.ReadAllReaderAt(reader, size)
	if err != nil {
		return nil, fmt.Errorf("read ODT: %w", err)
	}
	zr, err := common.OpenZip(bytes.NewReader(raw), int64(len(raw)))
	if err != nil {
		return nil, err
	}
	content, ok, err := common.ReadZipEntry(zr, "content.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ODT missing content.xml")
	}
	stylesData, ok, err := common.ReadZipEntry(zr, "styles.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("ODT missing styles.xml")
	}
	metaData, _, err := common.ReadZipEntry(zr, "meta.xml", common.MaxXMLBytes)
	if err != nil {
		return nil, err
	}
	styles, err := parseStyles(stylesData)
	if err != nil {
		return nil, fmt.Errorf("parse ODT styles: %w", err)
	}
	sourceMeta, err := parseMeta(metaData)
	if err != nil {
		return nil, fmt.Errorf("parse ODT metadata: %w", err)
	}
	meta, err := common.ResolveMetadata(raw, opts.InputPath, sourceMeta, cfg, opts.Metadata)
	if err != nil {
		return nil, err
	}
	body, footnotes, issues, report, err := parseContent(content, zr, cfg, styles, opts.StrictStyles)
	if err != nil {
		return nil, fmt.Errorf("parse ODT content: %w", err)
	}
	doc := &ir.Document{
		IRVersion: ir.CurrentIRVersion,
		Meta:      meta,
		Body:      body,
		Footnotes: footnotes,
	}
	return &parser.ParseResult{Document: doc, Issues: issues, StyleReport: report}, nil
}

func parseMeta(data []byte) (ir.Metadata, error) {
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
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "creator",
			start.Name.Space == "urn:oasis:names:tc:opendocument:xmlns:meta:1.0" && start.Name.Local == "initial-creator":
			value, err := common.ReadTextElement(dec, start)
			if err != nil {
				return meta, err
			}
			if meta.Author == "" {
				meta.Author = strings.TrimSpace(value)
			}
		case start.Name.Space == "http://purl.org/dc/elements/1.1/" && start.Name.Local == "date":
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

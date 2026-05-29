package lint

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	fontSizeRe      = regexp.MustCompile(`(?i)font-size\s*:\s*([0-9.]+)\s*([a-z%]+)`)
	fontSizePXRe    = regexp.MustCompile(`(?i)(font-size\s*:\s*)([0-9]+(?:\.[0-9]+)?)\s*px\b`)
	modRe           = regexp.MustCompile(`<meta[^>]+property="dcterms:modified"[^>]*>([^<]+)</meta>`)
	wordRe          = regexp.MustCompile(`<meta[^>]+property="schema:wordCount"[^>]*>([^<]+)</meta>`)
	possessiveFixRe = regexp.MustCompile(`(<i\b[^>]*epub:type="se:name\.publication"[^>]*>)([^<]*?)'s(</i>)`)
	metadataCloseRe = regexp.MustCompile(`(?i)</metadata>`)
	langRe          = regexp.MustCompile(`^[A-Za-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)
)

func Lint(epubBytes []byte) []Finding {
	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		return []Finding{{RuleID: RuleXHTMLXML, Severity: "error", File: "", Message: fmt.Sprintf("invalid EPUB zip: %v", err)}}
	}
	files := map[string][]byte{}
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			return []Finding{{RuleID: RuleXHTMLXML, Severity: "error", File: file.Name, Message: err.Error()}}
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return []Finding{{RuleID: RuleXHTMLXML, Severity: "error", File: file.Name, Message: err.Error()}}
		}
		files[file.Name] = data
	}
	var findings []Finding
	findings = append(findings, checkCSS(files)...)
	findings = append(findings, checkOPF(files["epub/content.opf"])...)
	findings = append(findings, checkXHTML(files)...)
	findings = append(findings, checkNoterefs(files)...)
	findings = append(findings, checkMedia(files)...)
	findings = append(findings, checkWordCount(files)...)
	return findings
}

func HasErrors(findings []Finding) bool {
	for _, finding := range findings {
		if finding.Severity == "error" {
			return true
		}
	}
	return false
}

type epubEntry struct {
	Name    string
	Data    []byte
	Store   bool
	ModTime time.Time
}

func Fix(epubBytes []byte, now time.Time) ([]byte, []Finding, bool, error) {
	entries, err := readEntries(epubBytes)
	if err != nil {
		return nil, nil, false, err
	}
	files := map[string][]byte{}
	for _, entry := range entries {
		files[entry.Name] = entry.Data
	}
	modified := false
	for i := range entries {
		name := entries[i].Name
		textData := string(entries[i].Data)
		next := textData
		if strings.HasSuffix(name, ".css") || strings.HasSuffix(name, ".xhtml") {
			next = fixFontSizePX(next)
		}
		if strings.HasSuffix(name, ".xhtml") {
			next = fixPossessive(next)
			next = fixStraightQuotes(next)
		}
		if next != textData {
			entries[i].Data = []byte(next)
			files[name] = entries[i].Data
			modified = true
		}
	}
	for i := range entries {
		if entries[i].Name != "epub/content.opf" {
			continue
		}
		next := fixModifiedOPF(string(entries[i].Data), fixTimestamp(now))
		next = fixWordCountOPF(next, files)
		if next != string(entries[i].Data) {
			entries[i].Data = []byte(next)
			files[entries[i].Name] = entries[i].Data
			modified = true
		}
	}
	if !modified {
		return epubBytes, Lint(epubBytes), false, nil
	}
	fixed, err := writeEntries(entries, fixTimestamp(now))
	if err != nil {
		return nil, nil, false, err
	}
	return fixed, Lint(fixed), true, nil
}

func readEntries(epubBytes []byte) ([]epubEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(epubBytes), int64(len(epubBytes)))
	if err != nil {
		return nil, fmt.Errorf("invalid EPUB zip: %w", err)
	}
	entries := make([]epubEntry, 0, len(zr.File))
	for _, file := range zr.File {
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			return nil, err
		}
		entries = append(entries, epubEntry{
			Name:    file.Name,
			Data:    data,
			Store:   file.Method == zip.Store || file.Name == "mimetype",
			ModTime: file.Modified,
		})
	}
	return entries, nil
}

func writeEntries(entries []epubEntry, modified time.Time) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, entry := range entries {
		method := zip.Deflate
		if entry.Store {
			method = zip.Store
		}
		header := &zip.FileHeader{Name: entry.Name, Method: method}
		if !modified.IsZero() {
			header.SetModTime(modified)
		} else {
			header.SetModTime(entry.ModTime)
		}
		w, err := zw.CreateHeader(header)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(entry.Data); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func fixFontSizePX(s string) string {
	return fontSizePXRe.ReplaceAllStringFunc(s, func(match string) string {
		parts := fontSizePXRe.FindStringSubmatch(match)
		if len(parts) != 3 {
			return match
		}
		px, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return match
		}
		return parts[1] + formatEm(px/16) + "em"
	})
}

func formatEm(value float64) string {
	out := strconv.FormatFloat(value, 'f', 4, 64)
	out = strings.TrimRight(out, "0")
	out = strings.TrimRight(out, ".")
	if out == "" {
		return "0"
	}
	return out
}

func fixModifiedOPF(opf string, ts time.Time) string {
	value := ts.UTC().Format("2006-01-02T15:04:05Z")
	if match := modRe.FindStringSubmatch(opf); len(match) == 2 {
		if _, err := time.Parse("2006-01-02T15:04:05Z", match[1]); err == nil {
			return opf
		}
		return modRe.ReplaceAllString(opf, `<meta property="dcterms:modified">`+value+`</meta>`)
	}
	return metadataCloseRe.ReplaceAllString(opf, `<meta property="dcterms:modified">`+value+`</meta>`+"\n</metadata>")
}

func fixPossessive(s string) string {
	return possessiveFixRe.ReplaceAllString(s, `$1$2$3's`)
}

func fixStraightQuotes(s string) string {
	var b strings.Builder
	open := true
	for len(s) > 0 {
		tag := strings.IndexByte(s, '<')
		if tag < 0 {
			b.WriteString(smartQuotes(s, &open))
			break
		}
		b.WriteString(smartQuotes(s[:tag], &open))
		end := strings.IndexByte(s[tag:], '>')
		if end < 0 {
			b.WriteString(s[tag:])
			break
		}
		end += tag
		b.WriteString(s[tag : end+1])
		s = s[end+1:]
	}
	return b.String()
}

func smartQuotes(s string, open *bool) string {
	var b strings.Builder
	for _, r := range s {
		if r == '"' {
			if *open {
				b.WriteString("&#8220;")
			} else {
				b.WriteString("&#8221;")
			}
			*open = !*open
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func fixWordCountOPF(opf string, files map[string][]byte) string {
	if !wordRe.MatchString(opf) {
		return opf
	}
	actual := 0
	for name, data := range files {
		if strings.HasSuffix(name, ".xhtml") {
			actual += len(strings.Fields(stripTags(string(data))))
		}
	}
	return wordRe.ReplaceAllString(opf, `<meta property="schema:wordCount">`+strconv.Itoa(actual)+`</meta>`)
}

func fixTimestamp(now time.Time) time.Time {
	if raw := os.Getenv("SOURCE_DATE_EPOCH"); raw != "" {
		if sec, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return time.Unix(sec, 0).UTC()
		}
	}
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}

func checkCSS(files map[string][]byte) []Finding {
	var findings []Finding
	for name, data := range files {
		if !strings.HasSuffix(name, ".css") && !strings.HasSuffix(name, ".xhtml") {
			continue
		}
		matches := fontSizeRe.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			unit := strings.ToLower(match[2])
			if unit != "em" && unit != "rem" {
				findings = append(findings, Finding{RuleID: RuleFontSize, Severity: "error", File: name, Message: "font-size must use em or rem"})
			}
		}
	}
	return findings
}

func checkOPF(data []byte) []Finding {
	if len(data) == 0 {
		return []Finding{{RuleID: RuleOPFRequired, Severity: "error", File: "epub/content.opf", Message: "content.opf is missing"}}
	}
	var findings []Finding
	text := string(data)
	for _, tag := range []string{"dc:title", "dc:language", "dc:identifier"} {
		if !strings.Contains(text, "<"+tag) {
			findings = append(findings, Finding{RuleID: RuleOPFRequired, Severity: "error", File: "epub/content.opf", Message: tag + " is missing"})
		}
	}
	match := modRe.FindStringSubmatch(text)
	if len(match) != 2 {
		findings = append(findings, Finding{RuleID: RuleModified, Severity: "error", File: "epub/content.opf", Message: "dcterms:modified is missing"})
	} else if _, err := time.Parse("2006-01-02T15:04:05Z", match[1]); err != nil {
		findings = append(findings, Finding{RuleID: RuleModified, Severity: "error", File: "epub/content.opf", Message: "dcterms:modified is invalid"})
	}
	return findings
}

func checkXHTML(files map[string][]byte) []Finding {
	var findings []Finding
	for name, data := range files {
		if !strings.HasSuffix(name, ".xhtml") {
			continue
		}
		dec := xml.NewDecoder(bytes.NewReader(data))
		publicationDepth := 0
		for {
			token, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				findings = append(findings, Finding{RuleID: RuleXHTMLXML, Severity: "error", File: name, Line: lineFor(data, dec.InputOffset()), Message: "XHTML is not well-formed XML"})
				break
			}
			switch t := token.(type) {
			case xml.StartElement:
				if t.Name.Local == "img" && strings.TrimSpace(attr(t, "alt")) == "" {
					findings = append(findings, Finding{RuleID: RuleImageAlt, Severity: "warning", File: name, Line: lineFor(data, dec.InputOffset()), Message: "img missing alt text"})
				}
				if lang := attrNS(t, "http://www.w3.org/XML/1998/namespace", "lang"); lang == "" && hasAttrNS(t, "http://www.w3.org/XML/1998/namespace", "lang") {
					findings = append(findings, Finding{RuleID: RuleXMLLang, Severity: "error", File: name, Line: lineFor(data, dec.InputOffset()), Message: "xml:lang is empty"})
				} else if lang != "" && !langRe.MatchString(lang) {
					findings = append(findings, Finding{RuleID: RuleXMLLang, Severity: "error", File: name, Line: lineFor(data, dec.InputOffset()), Message: "xml:lang is not BCP-47-like"})
				}
				epubType := attrNS(t, "http://www.idpf.org/2007/ops", "type")
				if strings.Contains(epubType, "title") && strings.Contains(epubType, "ordinal") {
					findings = append(findings, Finding{RuleID: RuleHeadingType, Severity: "warning", File: name, Line: lineFor(data, dec.InputOffset()), Message: "heading has incompatible epub:type values"})
				}
				if publicationDepth > 0 || (t.Name.Local == "i" && strings.Contains(epubType, "se:name.publication")) {
					publicationDepth++
				}
			case xml.EndElement:
				if publicationDepth > 0 {
					publicationDepth--
				}
			case xml.CharData:
				value := string(t)
				if strings.ContainsAny(value, `"`) {
					findings = append(findings, Finding{RuleID: RuleStraightQuote, Severity: "warning", File: name, Line: lineFor(data, dec.InputOffset()), Message: "straight quotes in text node"})
				}
				if publicationDepth > 0 && strings.Contains(value, "'s") {
					findings = append(findings, Finding{RuleID: RulePossessive, Severity: "warning", File: name, Line: lineFor(data, dec.InputOffset()), Message: "possessive is inside italicized work title"})
				}
			}
		}
	}
	return findings
}

func checkNoterefs(files map[string][]byte) []Finding {
	endnotes := string(files["epub/text/endnotes.xhtml"])
	ids := map[string]bool{}
	for _, match := range regexp.MustCompile(`id="([^"]+)"`).FindAllStringSubmatch(endnotes, -1) {
		ids[match[1]] = true
	}
	var findings []Finding
	for name, data := range files {
		if !strings.HasSuffix(name, ".xhtml") {
			continue
		}
		for _, match := range regexp.MustCompile(`epub:type="noteref"[^>]+href="(?:[^#"]*)#([^"]+)"`).FindAllStringSubmatch(string(data), -1) {
			if !ids[match[1]] {
				findings = append(findings, Finding{RuleID: RuleNoteref, Severity: "error", File: name, Message: "noteref points to missing endnote"})
			}
		}
	}
	return findings
}

func checkMedia(files map[string][]byte) []Finding {
	opf := string(files["epub/content.opf"])
	var findings []Finding
	for _, match := range regexp.MustCompile(`href="([^"]+)" media-type="([^"]+)"`).FindAllStringSubmatch(opf, -1) {
		href, media := "epub/"+match[1], match[2]
		data := files[href]
		if len(data) == 0 {
			continue
		}
		if strings.HasPrefix(media, "image/") && !imageMatches(media, data) {
			findings = append(findings, Finding{RuleID: RuleMediaType, Severity: "error", File: "epub/content.opf", Message: "manifest media type does not match image signature"})
		}
	}
	return findings
}

func checkWordCount(files map[string][]byte) []Finding {
	opf := string(files["epub/content.opf"])
	match := wordRe.FindStringSubmatch(opf)
	if len(match) != 2 {
		return nil
	}
	declared, err := strconv.Atoi(strings.TrimSpace(match[1]))
	if err != nil || declared <= 0 {
		return []Finding{{RuleID: RuleWordCount, Severity: "warning", File: "epub/content.opf", Message: "declared word_count is invalid"}}
	}
	actual := 0
	for name, data := range files {
		if strings.HasSuffix(name, ".xhtml") {
			actual += len(strings.Fields(stripTags(string(data))))
		}
	}
	if math.Abs(float64(declared-actual))/float64(declared) > 0.05 {
		return []Finding{{RuleID: RuleWordCount, Severity: "warning", File: "epub/content.opf", Message: "declared word_count differs from actual by more than 5%"}}
	}
	return nil
}

func attr(start xml.StartElement, local string) string {
	for _, a := range start.Attr {
		if a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func attrNS(start xml.StartElement, space, local string) string {
	for _, a := range start.Attr {
		if a.Name.Space == space && a.Name.Local == local {
			return a.Value
		}
	}
	return ""
}

func hasAttrNS(start xml.StartElement, space, local string) bool {
	for _, a := range start.Attr {
		if a.Name.Space == space && a.Name.Local == local {
			return true
		}
	}
	return false
}

func lineFor(data []byte, offset int64) int {
	if offset <= 0 || offset > int64(len(data)) {
		return 0
	}
	return bytes.Count(data[:offset], []byte{'\n'}) + 1
}

func imageMatches(media string, data []byte) bool {
	switch media {
	case "image/png":
		return len(data) >= 8 && bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10})
	case "image/jpeg":
		return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
	case "image/gif":
		return bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a"))
	case "image/svg+xml":
		return bytes.Contains(data, []byte("<svg"))
	default:
		return true
	}
}

func stripTags(s string) string {
	return regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, " ")
}

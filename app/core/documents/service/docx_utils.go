package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// FillDocxVariables rewrites a .docx file's word/document.xml in-memory,
// replacing every <VARIABLE> placeholder (stored as &lt;VARIABLE&gt; inside the
// XML) with the corresponding value from replacements (keyed as "<VARIABLE>").
// All other ZIP entries are copied through unchanged.
//
// The returned bytes form a valid .docx that can be saved directly to disk or
// streamed to a client.
//
// Split-run handling: Word sometimes stores a single placeholder across
// multiple <w:r> elements (e.g. &lt;NO_ in one run and HP&gt; in the next).
// FillDocxVariables detects and handles these splits transparently.
func FillDocxVariables(data []byte, replacements map[string]string, opsReplacements map[string]string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("fillDocx: open zip: %w", err)
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			// This entry must be decompressed so we can substitute variables.
			// Preserve the original file header (compression method, timestamps, …).
			fw, err := w.CreateHeader(&f.FileHeader)
			if err != nil {
				return nil, fmt.Errorf("fillDocx: create zip entry %q: %w", f.Name, err)
			}

			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("fillDocx: open zip entry %q: %w", f.Name, err)
			}
			content, readErr := io.ReadAll(rc)
			rc.Close()
			// zip.ErrChecksum means the CRC32 stored in the ZIP header does not match
			// the actual data. Google Drive sometimes exports .docx files with wrong
			// CRC32 values — Word opens them fine because it ignores this field, but
			// Go's zip package is strict. By the time ErrChecksum is returned, all
			// bytes have already been decompressed into `content`, so the data is
			// intact and safe to use.
			if readErr != nil && !errors.Is(readErr, zip.ErrChecksum) {
				return nil, fmt.Errorf("fillDocx: read zip entry %q: %w", f.Name, readErr)
			}

			content = replaceAngleBracketVars(content, replacements)
			content = replaceCurlyBraceVars(content, opsReplacements)

			if _, err := fw.Write(content); err != nil {
				return nil, fmt.Errorf("fillDocx: write zip entry %q: %w", f.Name, err)
			}
		} else {
			// For every other entry (styles, images, [Content_Types].xml, etc.) copy
			// the raw compressed bytes directly — no decompression, no CRC validation.
			// This is both faster and immune to the bad-CRC32 problem described above.
			fw, err := w.CreateRaw(&f.FileHeader)
			if err != nil {
				return nil, fmt.Errorf("fillDocx: create raw zip entry %q: %w", f.Name, err)
			}
			rc, err := f.OpenRaw()
			if err != nil {
				return nil, fmt.Errorf("fillDocx: open raw zip entry %q: %w", f.Name, err)
			}
			if _, err := io.Copy(fw, rc); err != nil {
				return nil, fmt.Errorf("fillDocx: copy raw zip entry %q: %w", f.Name, err)
			}
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("fillDocx: close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// replaceAngleBracketVars substitutes every &lt;VARIABLE&gt; occurrence in
// xmlContent with the matching value from replacements (key format: "<VARIABLE>").
//
// In a .docx file the literal characters < and > are XML-encoded as &lt; and
// &gt; inside <w:t> elements, so a template placeholder like <NAMA> lives in
// the raw XML as &lt;NAMA&gt;.
//
// Split-run problem: Word's XML editor sometimes stores a single placeholder
// across multiple <w:r> elements, e.g.:
//
//	&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;
//
// This function uses a per-variable regex (see buildSplitRunRegex) that matches
// the placeholder characters even when XML tags are interspersed between them,
// so both the normal and split-run cases are handled correctly.
//
// Replacement values are XML-escaped so that special characters such as &, <,
// >, ", and ' do not corrupt the document XML.
func replaceAngleBracketVars(xmlContent []byte, replacements map[string]string) []byte {
	result := string(xmlContent)
	for key, value := range replacements {
		// key format is "<VARIABLE>" — strip the outer angle brackets to get
		// just the variable name, then build a split-run-aware regex.
		if len(key) < 3 || key[0] != '<' || key[len(key)-1] != '>' {
			continue // unexpected key format — skip
		}
		varName := key[1 : len(key)-1]
		re := buildSplitRunRegex(varName)
		// Use ReplaceAllLiteralString so that $ in the value is not interpreted
		// as a regex back-reference.
		result = re.ReplaceAllLiteralString(result, xmlEscapeValue(value))
	}
	return []byte(result)
}

// buildSplitRunRegex returns a compiled regex that matches &lt;VARNAME&gt;
// even when XML tags are interspersed between any of the characters.
//
// Word's XML occasionally splits a placeholder like <NO_HP> across multiple
// <w:r> runs, producing raw XML such as:
//
//	&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;
//
// The returned pattern handles this by allowing `(?:<[^>]+>)*` — zero or more
// XML tags — between every pair of adjacent characters, including inside the
// &lt; and &gt; entity sequences themselves.
//
// When the regex matches a split-run placeholder, the replacement replaces the
// entire match (including the interleaved XML tags), leaving the surrounding
// <w:t>…</w:t> structure intact and producing a valid single-run element.
func buildSplitRunRegex(varName string) *regexp.Regexp {
	const optTag = `(?:<[^>]+>)*`

	// seq builds a pattern for a literal string s where any XML tags may appear
	// between consecutive characters.
	seq := func(s string) string {
		var sb strings.Builder
		for i, ch := range s {
			if i > 0 {
				sb.WriteString(optTag)
			}
			sb.WriteString(regexp.QuoteMeta(string(ch)))
		}
		return sb.String()
	}

	pattern := seq("&lt;") + optTag + seq(varName) + optTag + seq("&gt;")
	return regexp.MustCompile(pattern)
}

// replaceCurlyBraceVars substitutes every {VARIABLE} occurrence in xmlContent
// with the matching value from opsReplacements (keyed by the bare variable name,
// e.g. "nama" for a placeholder written as {nama}).
//
// Curly braces are not XML-entity-encoded in .docx files, so { and } appear
// literally in the raw XML.  The split-run problem still applies — Word may
// store {VARIABLE} across multiple <w:r> runs — so a split-run-aware regex
// (see buildCurlyBraceSplitRunRegex) is used for each variable.
//
// Replacement values are XML-escaped before insertion.
func replaceCurlyBraceVars(xmlContent []byte, opsReplacements map[string]string) []byte {
	if len(opsReplacements) == 0 {
		return xmlContent
	}
	result := string(xmlContent)
	for varName, value := range opsReplacements {
		if varName == "" {
			continue
		}
		// Normalise: strip outer braces if the caller already included them
		// (e.g. "{BLN}" → "BLN") so the regex always wraps exactly once.
		bareVar := varName
		if len(bareVar) > 2 && bareVar[0] == '{' && bareVar[len(bareVar)-1] == '}' {
			bareVar = bareVar[1 : len(bareVar)-1]
		}
		re := buildCurlyBraceSplitRunRegex(bareVar)
		result = re.ReplaceAllLiteralString(result, xmlEscapeValue(value))
	}
	return []byte(result)
}

// buildCurlyBraceSplitRunRegex returns a compiled regex that matches
// {VARNAME} even when XML tags are interspersed between any of the characters.
//
// Curly braces are literal in XML text, but Word may split a single placeholder
// like {nama} across multiple <w:r> runs, e.g.:
//
//	{na</w:t></w:r><w:r><w:t>ma}
//
// The pattern allows `(?:<[^>]+>)*` between every pair of consecutive characters
// so that both the normal and split-run cases are handled correctly.
func buildCurlyBraceSplitRunRegex(varName string) *regexp.Regexp {
	const optTag = `(?:<[^>]+>)*`

	seq := func(s string) string {
		var sb strings.Builder
		for i, ch := range s {
			if i > 0 {
				sb.WriteString(optTag)
			}
			sb.WriteString(regexp.QuoteMeta(string(ch)))
		}
		return sb.String()
	}

	pattern := seq("{") + optTag + seq(varName) + optTag + seq("}")
	return regexp.MustCompile(pattern)
}

// xmlEscapeValue escapes characters that are illegal (or ambiguous) inside XML
// text content so that replacement values are rendered correctly in Word.
func xmlEscapeValue(s string) string {
	// Order matters: & must be escaped first to avoid double-escaping.
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

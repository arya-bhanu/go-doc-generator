// tools/test_fill_doc/main.go
//
// Standalone script that exercises the docx variable-fill pipeline end-to-end
// using dummy values for <NO_KTP> and <NO_HP>.
//
// It reads a .docx template from the temp/ folder (or a path supplied via
// --input), applies dummy replacements, and writes the filled document back to
// temp/test_filled_output.docx so you can open it in Word / LibreOffice to
// verify that the placeholders were substituted correctly — including placeholders
// that Word split across multiple XML runs.
//
// Usage (run from the project root):
//
//	go run ./tools/test_fill_doc/
//	go run ./tools/test_fill_doc/ --input temp/MY_TEMPLATE.docx
package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
)

func main() {
	inputPath := flag.String("input", "", "Path to a .docx template (default: first non-filled .docx in temp/)")
	flag.Parse()

	// ── 1. Resolve input path ─────────────────────────────────────────────────
	if *inputPath == "" {
		entries, err := os.ReadDir("temp")
		if err != nil {
			log.Fatalf("cannot read temp/: %v", err)
		}
		for _, e := range entries {
			name := e.Name()
			if strings.HasSuffix(name, ".docx") && !strings.Contains(name, "_filled_") {
				*inputPath = "temp/" + name
				break
			}
		}
		if *inputPath == "" {
			log.Fatal("no suitable template found in temp/ — use --input to specify one")
		}
	}

	fmt.Printf("input  : %s\n", *inputPath)

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("read %s: %v", *inputPath, err)
	}

	// ── 2. Dummy replacements ─────────────────────────────────────────────────
	// These mirror the placeholders used in the banking document templates.
	// Add or edit entries here to test other variables.
	formFilledOps := map[string]string{
		"<NO_KTP>": "3171234567890001",
		"<NO_HP>":  "08123456789",
	}

	fmt.Println("replacements:")
	for k, v := range formFilledOps {
		fmt.Printf("  %-15s → %s\n", k, v)
	}

	// ── 3. Diagnose: show raw XML around the placeholders ─────────────────────
	showDiagnostics(data, formFilledOps)

	// ── 4. Fill document ──────────────────────────────────────────────────────
	filled, err := fillDocxVariables(data, formFilledOps)
	if err != nil {
		log.Fatalf("fillDocxVariables: %v", err)
	}

	// ── 5. Write output ───────────────────────────────────────────────────────
	outPath := "temp/test_filled_output.docx"
	if err := os.WriteFile(outPath, filled, 0o644); err != nil {
		log.Fatalf("write %s: %v", outPath, err)
	}

	fmt.Printf("output : %s\n", outPath)
	fmt.Println("done — open the output file in Word/LibreOffice to verify.")
}

// showDiagnostics uses the same split-run regex as the fill pipeline to detect
// and display exactly what will be matched (and replaced) in word/document.xml.
func showDiagnostics(data []byte, replacements map[string]string) {
	xmlStr := extractDocumentXML(data)
	if xmlStr == "" {
		return
	}

	fmt.Println("\n── diagnostics (raw XML context) ────────────────────────────")
	for key := range replacements {
		if len(key) < 3 || key[0] != '<' || key[len(key)-1] != '>' {
			continue
		}
		varName := key[1 : len(key)-1]

		// Use the exact same regex as the fill pipeline.
		re := buildSplitRunRegex(varName)
		loc := re.FindStringIndex(xmlStr)
		if loc == nil {
			fmt.Printf("[%s] ✗ NOT found (placeholder absent or encoded differently)\n\n", key)
			continue
		}

		matchText := xmlStr[loc[0]:loc[1]]
		start := max(0, loc[0]-50)
		end := min(len(xmlStr), loc[1]+50)
		context := xmlStr[start:end]

		// Is it a split-run or single-run match?
		singleEncoded := "&lt;" + varName + "&gt;"
		if matchText == singleEncoded {
			fmt.Printf("[%s] ✓ single run — match: %q\n  context: ...%s...\n\n", key, matchText, context)
		} else {
			fmt.Printf("[%s] ✓ split run — match spans %d chars across XML tags:\n  match:   %q\n  context: ...%s...\n\n",
				key, len(matchText), matchText, context)
		}
	}
	fmt.Println("─────────────────────────────────────────────────────────────")
}

func extractDocumentXML(data []byte) string {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return ""
		}
		defer rc.Close()
		b, _ := io.ReadAll(rc)
		return string(b)
	}
	return ""
}

// ── fill logic (self-contained copy so the tool has no server-side deps) ─────

func fillDocxVariables(data []byte, replacements map[string]string) ([]byte, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("open zip: %w", err)
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	for _, f := range r.File {
		fw, err := w.CreateHeader(&f.FileHeader)
		if err != nil {
			return nil, fmt.Errorf("create zip entry %q: %w", f.Name, err)
		}

		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("open zip entry %q: %w", f.Name, err)
		}
		content, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read zip entry %q: %w", f.Name, err)
		}

		if f.Name == "word/document.xml" {
			content = replaceAngleBracketVars(content, replacements)
		}

		if _, err := fw.Write(content); err != nil {
			return nil, fmt.Errorf("write zip entry %q: %w", f.Name, err)
		}
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func replaceAngleBracketVars(xmlContent []byte, replacements map[string]string) []byte {
	result := string(xmlContent)
	for key, value := range replacements {
		if len(key) < 3 || key[0] != '<' || key[len(key)-1] != '>' {
			continue
		}
		varName := key[1 : len(key)-1]
		re := buildSplitRunRegex(varName)
		result = re.ReplaceAllLiteralString(result, xmlEscapeValue(value))
	}
	return []byte(result)
}

// buildSplitRunRegex handles the Word "split-run" problem where a placeholder
// like <NO_HP> can be stored as &lt;NO_</w:t></w:r><w:r><w:t>HP&gt; in XML.
func buildSplitRunRegex(varName string) *regexp.Regexp {
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
	pattern := seq("&lt;") + optTag + seq(varName) + optTag + seq("&gt;")
	return regexp.MustCompile(pattern)
}

func xmlEscapeValue(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

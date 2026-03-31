package service

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// ── helpers ──────────────────────────────────────────────────────────────────

// makeDocx builds a minimal in-memory .docx (ZIP) containing only
// word/document.xml with the provided XML content.
func makeDocx(t *testing.T, xmlContent string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("word/document.xml")
	if err != nil {
		t.Fatalf("makeDocx: create entry: %v", err)
	}
	if _, err := f.Write([]byte(xmlContent)); err != nil {
		t.Fatalf("makeDocx: write xml: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("makeDocx: close zip: %v", err)
	}
	return buf.Bytes()
}

// extractDocXML reads word/document.xml back out of a .docx byte slice.
func extractDocXML(t *testing.T, docx []byte) string {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(docx), int64(len(docx)))
	if err != nil {
		t.Fatalf("extractDocXML: open zip: %v", err)
	}
	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("extractDocXML: open entry: %v", err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("extractDocXML: read: %v", err)
		}
		return string(data)
	}
	t.Fatal("extractDocXML: word/document.xml not found")
	return ""
}

// ── FillDocxVariables ─────────────────────────────────────────────────────────

// TestFillDocxVariables_SingleRun verifies that a placeholder contained
// entirely in one <w:t> element is replaced correctly.
func TestFillDocxVariables_SingleRun(t *testing.T) {
	xml := `<w:document><w:body><w:p>` +
		`<w:r><w:t>KTP: &lt;NO_KTP&gt; HP: &lt;NO_HP&gt;</w:t></w:r>` +
		`</w:p></w:body></w:document>`

	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_KTP>": "3171234567890001",
		"<NO_HP>":  "08123456789",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:document><w:body><w:p>` +
		`<w:r><w:t>KTP: 3171234567890001 HP: 08123456789</w:t></w:r>` +
		`</w:p></w:body></w:document>`

	if got != want {
		t.Errorf("single-run:\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_SplitRun_VarNameSplit tests the most common split-run
// case: the variable name characters are split across two consecutive <w:r>
// elements.
//
//	&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;
func TestFillDocxVariables_SplitRun_VarNameSplit(t *testing.T) {
	xml := `<w:r><w:t>&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;</w:t></w:r>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": "08123456789",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	// The two runs collapse into one: the first <w:r><w:t> remains open,
	// the value is written, then the trailing </w:t></w:r> from the second run
	// closes it — producing a single valid run element.
	want := `<w:r><w:t>08123456789</w:t></w:r>`
	if got != want {
		t.Errorf("split-run (varname split):\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_SplitRun_EntitySplit tests the case where the opening
// &lt; entity itself is in one run and the variable name + &gt; are in another.
//
//	<w:r><w:t>&lt;</w:t></w:r><w:r><w:t>NO_KTP&gt;</w:t></w:r>
func TestFillDocxVariables_SplitRun_EntitySplit(t *testing.T) {
	xml := `<w:r><w:t>&lt;</w:t></w:r><w:r><w:t>NO_KTP&gt;</w:t></w:r>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_KTP>": "3171234567890001",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:r><w:t>3171234567890001</w:t></w:r>`
	if got != want {
		t.Errorf("split-run (entity split):\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_SplitRun_ThreeWaySplit tests a three-way split where
// &lt;, the variable name, and &gt; each live in separate runs.
func TestFillDocxVariables_SplitRun_ThreeWaySplit(t *testing.T) {
	xml := `<w:r><w:t>&lt;</w:t></w:r>` +
		`<w:r><w:t>NO_HP</w:t></w:r>` +
		`<w:r><w:t>&gt;</w:t></w:r>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": "08123456789",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:r><w:t>08123456789</w:t></w:r>`
	if got != want {
		t.Errorf("split-run (three-way):\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_SplitRun_WithFormatting tests that run-level formatting
// (<w:rPr>) of the first run is preserved after a split-run merge.
func TestFillDocxVariables_SplitRun_WithFormatting(t *testing.T) {
	xml := `<w:r><w:rPr><w:b/></w:rPr><w:t>&lt;NO_</w:t></w:r>` +
		`<w:r><w:rPr><w:b/></w:rPr><w:t>HP&gt;</w:t></w:r>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": "08123456789",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	// First run's <w:rPr> is preserved; second run's formatting is absorbed
	// into the regex match and dropped (the value only needs one run).
	want := `<w:r><w:rPr><w:b/></w:rPr><w:t>08123456789</w:t></w:r>`
	if got != want {
		t.Errorf("split-run (with formatting):\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_XMLEscaping verifies that special characters in
// replacement values are properly escaped before being written into XML.
func TestFillDocxVariables_XMLEscaping(t *testing.T) {
	xml := `<w:t>&lt;NO_HP&gt;</w:t>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": `A&B<C>D"E'F`,
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:t>A&amp;B&lt;C&gt;D&quot;E&apos;F</w:t>`
	if got != want {
		t.Errorf("XML escaping:\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_DollarSignValue verifies that a value containing '$'
// is not misinterpreted as a regex back-reference.
func TestFillDocxVariables_DollarSignValue(t *testing.T) {
	xml := `<w:t>&lt;NO_HP&gt;</w:t>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": "$100",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:t>$100</w:t>`
	if got != want {
		t.Errorf("dollar-sign value:\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_InvalidKey checks that keys without the expected
// "<VARIABLE>" wrapping are silently skipped without corrupting the document.
func TestFillDocxVariables_InvalidKey(t *testing.T) {
	xml := `<w:t>&lt;NO_HP&gt;</w:t>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"NO_HP":   "ignored", // missing angle brackets
		"<NO_HP>": "08123456789",
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	want := `<w:t>08123456789</w:t>`
	if got != want {
		t.Errorf("invalid key:\ngot  %s\nwant %s", got, want)
	}
}

// TestFillDocxVariables_NoMatch_LeaveUnchanged ensures that a document is
// returned unmodified when none of the replacement keys match.
func TestFillDocxVariables_NoMatch_LeaveUnchanged(t *testing.T) {
	xml := `<w:t>&lt;NO_KTP&gt;</w:t>`
	docx := makeDocx(t, xml)

	replacements := map[string]string{
		"<NO_HP>": "08123456789", // different variable — should not touch NO_KTP
	}

	filled, err := FillDocxVariables(docx, replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	got := extractDocXML(t, filled)
	if got != xml {
		t.Errorf("no-match: document should be unchanged\ngot  %s\nwant %s", got, xml)
	}
}

// TestFillDocxVariables_NonDocumentEntries verifies that ZIP entries other than
// word/document.xml are passed through unchanged.
func TestFillDocxVariables_NonDocumentEntries(t *testing.T) {
	// Build a .docx with an extra entry that contains a placeholder-like string.
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	// word/document.xml — should be modified
	f1, _ := w.Create("word/document.xml")
	_, _ = f1.Write([]byte(`<w:t>&lt;NO_HP&gt;</w:t>`))

	// word/styles.xml — must NOT be touched even though it has the same pattern
	f2, _ := w.Create("word/styles.xml")
	_, _ = f2.Write([]byte(`<a:t>&lt;NO_HP&gt;</a:t>`))

	_ = w.Close()

	replacements := map[string]string{
		"<NO_HP>": "08123456789",
	}

	filled, err := FillDocxVariables(buf.Bytes(), replacements)
	if err != nil {
		t.Fatalf("FillDocxVariables: %v", err)
	}

	// Extract both entries and verify.
	r, _ := zip.NewReader(bytes.NewReader(filled), int64(len(filled)))
	results := map[string]string{}
	for _, f := range r.File {
		rc, _ := f.Open()
		data, _ := io.ReadAll(rc)
		rc.Close()
		results[f.Name] = string(data)
	}

	if got := results["word/document.xml"]; got != `<w:t>08123456789</w:t>` {
		t.Errorf("document.xml: got %q", got)
	}
	if got := results["word/styles.xml"]; got != `<a:t>&lt;NO_HP&gt;</a:t>` {
		t.Errorf("styles.xml should be unchanged, got %q", got)
	}
}

// ── xmlEscapeValue ────────────────────────────────────────────────────────────

func TestXMLEscapeValue(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"A&B", "A&amp;B"},
		{"<tag>", "&lt;tag&gt;"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
		{`a&b<c>d"e'f`, `a&amp;b&lt;c&gt;d&quot;e&apos;f`},
		// & must not be double-escaped
		{"&amp;", "&amp;amp;"},
	}
	for _, tc := range cases {
		got := xmlEscapeValue(tc.input)
		if got != tc.want {
			t.Errorf("xmlEscapeValue(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
		}
	}
}

// ── buildSplitRunRegex ────────────────────────────────────────────────────────

func TestBuildSplitRunRegex_MatchesSimple(t *testing.T) {
	re := buildSplitRunRegex("NO_HP")
	cases := []struct {
		input string
		match bool
	}{
		// exact single-run encoding
		{"&lt;NO_HP&gt;", true},
		// split at varname
		{"&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;", true},
		// split at opening entity
		{"&lt;</w:t></w:r><w:r><w:t>NO_HP&gt;", true},
		// three-way split
		{"&lt;</w:t></w:r><w:r><w:t>NO_HP</w:t></w:r><w:r><w:t>&gt;", true},
		// wrong variable — must not match
		{"&lt;NO_KTP&gt;", false},
		// partial — must not match
		{"&lt;NO_HP", false},
	}
	for _, tc := range cases {
		got := re.MatchString(tc.input)
		if got != tc.match {
			t.Errorf("buildSplitRunRegex(NO_HP).MatchString(%q) = %v, want %v",
				tc.input, got, tc.match)
		}
	}
}

// ── replaceAngleBracketVars (integration) ────────────────────────────────────

func TestReplaceAngleBracketVars_DummyValues(t *testing.T) {
	// Simulate the exact XML fragment that Word might generate for a document
	// that has both normal and split placeholders on the same page.
	xml := strings.Join([]string{
		// Normal run — placeholder in one piece
		`<w:r><w:t>No KTP: &lt;NO_KTP&gt;</w:t></w:r>`,
		// Split run — NO_HP broken across two runs (the reported failure case)
		`<w:r><w:t>&lt;NO_</w:t></w:r><w:r><w:t>HP&gt;</w:t></w:r>`,
	}, "\n")

	replacements := map[string]string{
		"<NO_KTP>": "3171234567890001",
		"<NO_HP>":  "08123456789",
	}

	result := string(replaceAngleBracketVars([]byte(xml), replacements))

	if strings.Contains(result, "&lt;NO_KTP&gt;") {
		t.Error("NO_KTP placeholder was not replaced")
	}
	if strings.Contains(result, "&lt;NO_") || strings.Contains(result, "HP&gt;") {
		t.Error("NO_HP split-run placeholder was not replaced")
	}
	if !strings.Contains(result, "3171234567890001") {
		t.Error("NO_KTP value missing from result")
	}
	if !strings.Contains(result, "08123456789") {
		t.Error("NO_HP value missing from result")
	}
}

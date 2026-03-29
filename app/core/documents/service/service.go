package service

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"regexp"

	"github.com/gin-gonic/gin"

	documents "github.com/arya-bhanu/go-doc-generator/app/core/documents"
	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/constants"
)

// Pre-compiled regexes for template variable patterns.
var (
	// curlyVarRe matches {variable} in raw XML text.
	curlyVarRe = regexp.MustCompile(`\{([^}]+)\}`)

	// angleVarRe matches <variable> patterns that are XML-encoded as &lt;...&gt;
	// inside a .docx word/document.xml file.
	// Only pure alphanumeric identifiers (letters, digits, underscores) are matched
	// to avoid capturing XML tags such as &lt;w:bCs w:val="0"/&gt;.
	angleVarRe = regexp.MustCompile(`&lt;([A-Za-z0-9_]+)&gt;`)
)

// DocumentService holds the repositories needed for document business logic.
type DocumentService struct {
	GDriveRepo docrepo.GDriveRepository
}

// NewDocumentService constructs a DocumentService with the provided GDrive repository.
func NewDocumentService(gdriveRepo docrepo.GDriveRepository) *DocumentService {
	return &DocumentService{GDriveRepo: gdriveRepo}
}

func fetchDocumentVariables() {}

// fetchDocumentTemplates fetches documents from Google Drive, saves each one
// to a temp file named "<uuidv6>_<original-name>.docx", and returns the
// resulting DocumentFile slice.
func (s *DocumentService) fetchDocumentTemplates(docIDs []string) ([]docrepo.DocumentFile, error) {
	return s.GDriveRepo.FetchDocuments(docIDs)
}

// ProcessDocuments retrieves the authenticated operator from the Gin context,
// downloads all requested document templates, scans each one for template
// variables, and returns the resulting DocumentFile slice.
func (s *DocumentService) ProcessDocuments(c *gin.Context, docIDs []string) ([]string, error) {
	userctx, exist := c.Get(constants.UserOpsContextKey)
	var docTitles []string
	if !exist {
		return nil, nil
	}

	userOps := userctx.(users.UserOps)
	_ = userOps // available for logging / auth checks in future steps

	docs, err := s.fetchDocumentTemplates(docIDs)
	if err != nil {
		return nil, err
	}

	storedVariables := make(map[string]documents.DocumentVariable)
	for _, doc := range docs {
		docTitles = append(docTitles, doc.Title)
		s.scanDocument(doc.Data, storedVariables)
	}

	fmt.Println(storedVariables)

	return docTitles, nil
}

// scanDocument scans the raw .docx bytes for template variable placeholders.
//
// It extracts word/document.xml from the .docx ZIP archive and applies two
// regex patterns to the raw XML text:
//   - {variable}  – curly-brace placeholders that appear as-is in XML text.
//   - <variable>  – angle-bracket placeholders that are encoded as &lt;...&gt;
//     inside the XML.
//
// Each unique match is stored in storedVariables as a key with a default
// documents.DocumentVariable{} value. Existing keys are left unchanged so that
// variables discovered earlier are not overwritten.
func (s *DocumentService) scanDocument(doc []byte, storedVariables map[string]documents.DocumentVariable) {
	// Open the .docx ZIP archive from the in-memory bytes.
	r, err := zip.NewReader(bytes.NewReader(doc), int64(len(doc)))
	if err != nil {
		return
	}

	for _, f := range r.File {
		if f.Name != "word/document.xml" {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return
		}
		xmlData, err := io.ReadAll(rc)
		defer rc.Close()
		if err != nil {
			return
		}

		xmlStr := string(xmlData)

		// ── {variable} patterns ───────────────────────────────────────────────
		for _, match := range curlyVarRe.FindAllString(xmlStr, -1) {
			if _, exists := storedVariables[match]; !exists {
				storedVariables[match] = documents.DocumentVariable{}
			}
		}

		// ── <variable> patterns (stored as &lt;variable&gt; inside XML) ──────
		for _, sub := range angleVarRe.FindAllStringSubmatch(xmlStr, -1) {
			key := "<" + sub[1] + ">"
			if _, exists := storedVariables[key]; !exists {
				storedVariables[key] = documents.DocumentVariable{}
			}
		}

		break
	}
}

func sendDocuments() {}

func createCustomerSession() {}

func createUserOpsSession() {}

func updateCustomerSession() {}

func updateUserOpsSession() {}

func deleteCustomerSession() {}

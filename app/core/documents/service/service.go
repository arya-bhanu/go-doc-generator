package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"regexp"
	"sync"

	"github.com/gin-gonic/gin"

	documents "github.com/arya-bhanu/go-doc-generator/app/core/documents"
	"github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	"github.com/arya-bhanu/go-doc-generator/constants"
)

var (
	// xmlTagRe strips XML tags AND any surrounding whitespace so that text split
	// across multiple <w:t> runs is concatenated without gaps.
	// e.g.  &lt;NO_H</w:t>\n  <w:t>P&gt;  →  &lt;NO_HP&gt;
	xmlTagRe = regexp.MustCompile(`\s*<[^>]+>\s*`)

	// curlyVarRe matches {variable} in the collapsed plain-text content.
	curlyVarRe = regexp.MustCompile(`\{([^}]+)\}`)

	// angleVarRe matches <variable> patterns after XML tags have been stripped.
	// The &lt; / &gt; entities remain in the collapsed text, so we still look for
	// them — but now across run boundaries. Only pure alphanumeric identifiers
	// (letters, digits, underscores) are accepted to avoid XML attribute noise.
	angleVarRe = regexp.MustCompile(`&lt;([A-Za-z0-9_]+)&gt;`)
)

type DocumentService struct {
	GDriveRepo docrepo.GDriveRepository
}

func NewDocumentService(gdriveRepo docrepo.GDriveRepository) *DocumentService {
	return &DocumentService{GDriveRepo: gdriveRepo}
}

func fetchDocumentVariables() {}

func (s *DocumentService) fetchDocumentTemplates(docIDs []string) ([]docrepo.DocumentFile, error) {
	return s.GDriveRepo.FetchDocuments(docIDs)
}

func (s *DocumentService) ProcessDocuments(c *gin.Context, docIDs []string) (map[string]*documents.DocumentVariable, map[string]*documents.DocumentVariable, documents.FormSessions, error) {
	userctx, exist := c.Get(constants.UserOpsContextKey)
	if !exist {
		return nil, nil, documents.FormSessions{}, errors.New("user not exist")
	}

	userOps := userctx.(users.UserOps)

	docs, err := s.fetchDocumentTemplates(docIDs)
	if err != nil {
		return nil, nil, documents.FormSessions{}, err
	}

	docDetails := make([]documents.DocumentDetail, len(docs))
	docTitles := make([]string, len(docs))
	for i, doc := range docs {
		docDetails[i] = documents.DocumentDetail{
			DocTempTitle: doc.Title,
			DocID:        docIDs[i],
		}
		docTitles[i] = doc.Title
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	storedCustVariables := make(map[string]*documents.DocumentVariable)
	custVariables := make(map[string]*documents.DocumentVariable)
	custVarChan := make(chan string)
	storedUserOpsVariables := make(map[string]*documents.DocumentVariable)
	userOpsVariables := make(map[string]*documents.DocumentVariable)
	userOpsVarChan := make(chan string)

	for _, doc := range docs {
		s.scanDocument(doc.Data, storedCustVariables, storedUserOpsVariables)
	}

	go func() {
		defer close(custVarChan)
		for key := range storedCustVariables {
			custVarChan <- key
		}
	}()

	go func() {
		defer close(userOpsVarChan)
		for key := range storedUserOpsVariables {
			userOpsVarChan <- key
		}
	}()

	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range custVarChan {
				res := repository.FetchDocVariable(key)
				mu.Lock()
				custVariables[key] = res
				mu.Unlock()
			}
		}()
	}

	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range userOpsVarChan {
				res := repository.FetchDocVariable(key)
				mu.Lock()
				userOpsVariables[key] = res
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	payload := documents.FormSessions{
		DocDetails:       docDetails,
		FormLink:         "",
		FormScaffoldCust: custVariables,
		FormScaffoldOps:  userOpsVariables,
		UserID:           userOps.ID,
	}

	return custVariables, userOpsVariables, payload, err
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
func (s *DocumentService) scanDocument(doc []byte, storedCustVariables map[string]*documents.DocumentVariable, storedUserOpsVariables map[string]*documents.DocumentVariable) {
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

		collapsed := xmlTagRe.ReplaceAllString(string(xmlData), "")

		// ── {variable} patterns ───────────────────────────────────────────────
		for _, match := range curlyVarRe.FindAllString(collapsed, -1) {
			if _, exists := storedUserOpsVariables[match]; !exists {
				storedUserOpsVariables[match] = nil
			}
		}

		// ── <variable> patterns (stored as &lt;variable&gt; inside XML) ──────
		for _, sub := range angleVarRe.FindAllStringSubmatch(collapsed, -1) {
			key := "<" + sub[1] + ">"
			if _, exists := storedCustVariables[key]; !exists {
				storedCustVariables[key] = nil
			}
		}

		break
	}
}

func (s *DocumentService) CreateSession(payload documents.FormSessions) error {
	if err := docrepo.CreateFormSessions(payload); err != nil {
		return err
	}
	return nil
}

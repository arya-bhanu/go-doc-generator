package service

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/arya-bhanu/go-doc-generator/app/conpool"
	documents "github.com/arya-bhanu/go-doc-generator/app/core/documents"
	"github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	docrepo "github.com/arya-bhanu/go-doc-generator/app/core/documents/repository"
	"github.com/arya-bhanu/go-doc-generator/app/core/users"
	usersrepo "github.com/arya-bhanu/go-doc-generator/app/core/users/repository"
	"github.com/arya-bhanu/go-doc-generator/app/email"
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

// CreateSession inserts a new form session row into the database.
func (s *DocumentService) CreateSession(payload documents.FormSessions) error {
	return docrepo.CreateFormSessions(payload)
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
	for i, doc := range docs {
		docDetails[i] = documents.DocumentDetail{
			DocTempTitle:  doc.Title,
			DocID:         docIDs[i],
			OriginalTitle: doc.OriginalTitle,
		}
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

// GenerateDocuments is triggered for every new Google Form response.
// It:
//  1. Fetches the form_sessions row that owns formID to get the document list
//     and the cust-variable scaffold (FormScaffoldCust).
//  2. Builds a formFilledOps map (placeholder → answer) by matching each
//     submitted question label against the scaffold's DocumentVariable.Label.
//     When a question has multiple answers they are joined with ", ".
//  3. For every document template listed in the session, reads the saved temp
//     copy, replaces all <VARIABLE> placeholders via fillDocxVariables, and
//     writes the filled document back to the temp folder with a unique name.
func (s *DocumentService) GenerateDocuments(formID string, qAndA []conpool.FormAnswer) {

	// ── 1. Fetch the session ──────────────────────────────────────────────────
	session, err := docrepo.FetchFormSession(formID)
	if err != nil {
		slog.Error("generateDocuments: fetch session", "formID", formID, "err", err)
		return
	}

	// ── 2. Build the placeholder → answer map ─────────────────────────────────
	// FormScaffoldCust keys are "<VARIABLE>" strings; values carry the Label
	// that matches the Google Form question title.
	formFilledOps := make(map[string]string)
	storedFormFilledDetail := make(map[string]conpool.FormAnswer)
	for _, qa := range qAndA {
		for key, docVar := range session.FormScaffoldCust {
			if docVar == nil {
				continue
			}
			if qa.Question != docVar.Label {
				continue
			}
			storedFormFilledDetail[key] = qa
			switch len(qa.Answers) {
			case 0:
				formFilledOps[key] = ""
			case 1:
				formFilledOps[key] = qa.Answers[0]
			default:
				formFilledOps[key] = strings.Join(qa.Answers, ", ")
			}
		}
	}

	go func() {
		// store qAndA into form_sessions.form_filled_customer as json
		if err := docrepo.StoreFormFilledCustomer(formID, storedFormFilledDetail); err != nil {
			slog.Error("generateDocuments: store form_filled_customer",
				"formID", formID, "err", err)
		}
	}()

	slog.Info("generateDocuments: variable map built",
		"formID", formID,
		"variables", len(formFilledOps),
	)

	slog.Info("formFilledOps:", "formFilledOps", formFilledOps)

	// ── 3. Fill each document template, collect as email attachments ──────────
	// Filled documents are kept in memory and attached directly to the email —
	// they are never written back to temp/.  The original template file is
	// deleted from temp/ after the email is dispatched.
	var attachments []email.Attachment
	var templatePaths []string

	for _, detail := range session.DocDetails {
		tempPath := filepath.Join("temp", detail.DocTempTitle)
		templatePaths = append(templatePaths, tempPath)

		data, err := os.ReadFile(tempPath)
		if err != nil {
			slog.Error("generateDocuments: read temp file",
				"path", tempPath, "err", err)
			continue
		}

		filled, err := FillDocxVariables(data, formFilledOps)
		if err != nil {
			slog.Error("generateDocuments: fill variables",
				"title", detail.DocTempTitle, "err", err)
			continue
		}

		uid, _ := uuid.NewV6()
		outTitle := fmt.Sprintf("%s_filled_%s.docx", uid.String(), detail.OriginalTitle)

		attachments = append(attachments, email.Attachment{
			Filename: outTitle,
			Data:     filled,
		})
		slog.Info("generateDocuments: document filled", "title", outTitle)
	}

	if len(attachments) == 0 {
		slog.Warn("generateDocuments: no documents filled, skipping email",
			"formID", formID)
		return
	}

	// ── 4. Resolve recipient email from session's user ID ─────────────────────
	userOps, err := usersrepo.GetUserByID(session.UserID)
	if err != nil {
		slog.Error("generateDocuments: get user by id",
			"userID", session.UserID, "err", err)
		return
	}

	// ── 5. Send email with all filled documents attached ──────────────────────
	email.SendEmail(
		fmt.Sprintf("Dokumen Anda Telah Diisi — %s", formID),
		attachments,
		userOps.Email,
	)

	// ── 6. Remove original template files from temp/ ──────────────────────────
	for _, path := range templatePaths {
		if rmErr := os.Remove(path); rmErr != nil {
			slog.Warn("generateDocuments: remove template",
				"path", path, "err", rmErr)
		} else {
			slog.Info("generateDocuments: template removed", "path", path)
		}
	}

	// ── 7. Delete the form session row from the database ─────────────────────
	if err := docrepo.DeleteFormIDSession(formID); err != nil {
		slog.Error("generateDocuments: delete form session",
			"formID", formID, "err", err)
	} else {
		slog.Info("generateDocuments: form session deleted", "formID", formID)
	}
}

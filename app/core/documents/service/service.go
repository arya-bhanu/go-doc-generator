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

// ClearFormScaffoldCust nulls out form_scaffold_cust, form_link, and form_id
// on the form_sessions row for the given userID.
func (s *DocumentService) ClearFormScaffoldCust(userID int) error {
	return docrepo.ClearFormScaffoldCust(userID)
}

// UpsertSession checks whether a form_sessions row already exists for the
// payload's UserID.  If it does, it updates form_link, form_scaffold_cust,
// doc_details, and form_id on that row.  Otherwise it creates a fresh row.
func (s *DocumentService) UpsertSession(payload documents.FormSessions) error {
	existing, err := docrepo.FetchFormSessionByUserID(payload.UserID)
	if err != nil {
		return fmt.Errorf("upsertSession: check existing session: %w", err)
	}

	if existing != nil {
		// Session already exists — update only the relevant fields.
		return docrepo.UpdateFormSession(payload.UserID, payload)
	}

	// No existing session — create a new one.
	return docrepo.CreateFormSessions(payload)
}

// SendDocumentsDirect fills every document template in payload.DocDetails with
// answers from answeredQuestCust (matched via FormScaffoldCust labels), then
// sends them by email to the session owner. Template files are removed from
// temp/ after dispatch, mirroring the cleanup done in GenerateDocuments.
func (s *DocumentService) SendDocumentsDirect(payload documents.FormSessions, answeredQuestCust map[string]conpool.FormAnswer) error {
	// Convert answeredQuestCust (map[string]conpool.FormAnswer) directly to a
	// string map for document filling — same conversion used in GenerateDocuments.
	formFilledOps := make(map[string]string)
	for key, qa := range answeredQuestCust {
		switch len(qa.Answers) {
		case 0:
			formFilledOps[key] = ""
		case 1:
			formFilledOps[key] = qa.Answers[0]
		default:
			formFilledOps[key] = strings.Join(qa.Answers, ", ")
		}
	}

	// Fetch ops-user's answered fields for {curly-brace} substitution —
	// same pattern used in GenerateDocuments.
	opsFormFilled := usersrepo.FetchOpsUserFormFilled(payload.UserID)
	opsAnswers := make(map[string]string, len(opsFormFilled))
	for _, field := range opsFormFilled {
		opsAnswers[field.Variable] = field.Answer
	}

	var attachments []email.Attachment
	var templatePaths []string

	for _, detail := range payload.DocDetails {
		tempPath := filepath.Join("temp", detail.DocTempTitle)
		templatePaths = append(templatePaths, tempPath)

		data, err := os.ReadFile(tempPath)
		if err != nil {
			slog.Error("sendDocumentsDirect: read temp file",
				"path", tempPath, "err", err)
			continue
		}

		filled, err := FillDocxVariables(data, formFilledOps, opsAnswers)
		if err != nil {
			slog.Error("sendDocumentsDirect: fill variables",
				"title", detail.DocTempTitle, "err", err)
			continue
		}

		uid, _ := uuid.NewV6()
		outTitle := fmt.Sprintf("%s_filled_%s.docx", uid.String(), detail.OriginalTitle)
		attachments = append(attachments, email.Attachment{
			Filename: outTitle,
			Data:     filled,
		})
		slog.Info("sendDocumentsDirect: document filled", "title", outTitle)
	}

	if len(attachments) == 0 {
		return fmt.Errorf("sendDocumentsDirect: no documents could be filled")
	}

	userOps, err := usersrepo.GetUserByID(payload.UserID)
	if err != nil {
		return fmt.Errorf("sendDocumentsDirect: get user by id %d: %w", payload.UserID, err)
	}

	email.SendEmail(
		"Dokumen Anda Telah Dikirim",
		attachments,
		userOps.Email,
	)

	for _, path := range templatePaths {
		if rmErr := os.Remove(path); rmErr != nil {
			slog.Warn("sendDocumentsDirect: remove template",
				"path", path, "err", rmErr)
		} else {
			slog.Info("sendDocumentsDirect: template removed", "path", path)
		}
	}

	return nil
}

func (s *DocumentService) fetchDocumentTemplates(docIDs []string) ([]docrepo.DocumentFile, error) {
	return s.GDriveRepo.FetchDocuments(docIDs)
}

func (s *DocumentService) ProcessDocuments(c *gin.Context, docIDs []string) (map[string]*documents.DocumentVariable, map[string]conpool.FormAnswer, documents.FormSessions, error) {
	var answeredQuestCust map[string]conpool.FormAnswer
	userctx, exist := c.Get(constants.UserOpsContextKey)
	if !exist {
		return nil, answeredQuestCust, documents.FormSessions{}, errors.New("user not exist")
	}

	userOps := userctx.(users.UserOps)

	answeredQuestCust = repository.FetchAnswerdCustomerForm(userOps.ID)

	docs, err := s.fetchDocumentTemplates(docIDs)
	if err != nil {
		return nil, answeredQuestCust, documents.FormSessions{}, err
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
				if _, ok := answeredQuestCust[key]; ok {
					continue
				}
				res := repository.FetchDocVariable(key)
				mu.Lock()
				if res != nil {
					custVariables[key] = res
				}
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
				if res != nil {
					userOpsVariables[key] = res
				}
				mu.Unlock()
			}
		}()
	}

	wg.Wait()

	payload := documents.FormSessions{
		DocDetails:       docDetails,
		FormLink:         "",
		FormScaffoldCust: &custVariables,
		UserID:           userOps.ID,
	}

	return custVariables, answeredQuestCust, payload, err
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

	// ── 2. Build placeholder → answer maps ────────────────────────────────────
	// FormScaffoldCust keys are "<VARIABLE>" strings; values carry the Label
	// that matches the Google Form question title.
	// storedFormFilledDetail keeps the full FormAnswer structs for DB storage.
	formFilledOps := make(map[string]string)
	storedFormFilledDetail := make(map[string]conpool.FormAnswer)
	if session.FormScaffoldCust != nil {
		for _, qa := range qAndA {
			for key, docVar := range *session.FormScaffoldCust {
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
	}

	// ── 2b. Fetch existing FormAnswer map, merge new answers in, persist, then
	// build the full string map (existing + new) for document filling ──────────
	existing, err := docrepo.FetchFormFilledCustomer(formID)
	if err != nil {
		slog.Warn("generateDocuments: fetch existing form_filled_customer, starting fresh",
			"formID", formID, "err", err)
		existing = make(map[string]conpool.FormAnswer)
	}
	// New answers overwrite old ones for the same key.
	for k, v := range storedFormFilledDetail {
		existing[k] = v
	}
	if err := docrepo.StoreFormFilledCustomer(formID, existing); err != nil {
		slog.Error("generateDocuments: store form_filled_customer",
			"formID", formID, "err", err)
	}

	slog.Info("existing:", "existing", existing)

	for key, qa := range existing {
		switch len(qa.Answers) {
		case 0:
			formFilledOps[key] = ""
		case 1:
			formFilledOps[key] = qa.Answers[0]
		default:
			formFilledOps[key] = strings.Join(qa.Answers, ", ")
		}
	}

	slog.Info("generateDocuments: variable map built",
		"formID", formID,
		"variables", len(formFilledOps),
	)

	slog.Info("formFilledOps:", "formFilledOps", formFilledOps)

	// ── 2c. Fetch ops-user's answered fields for {curly-brace} substitution ──
	opsFormFilled := usersrepo.FetchOpsUserFormFilled(session.UserID)
	opsAnswers := make(map[string]string, len(opsFormFilled))
	for _, field := range opsFormFilled {
		opsAnswers[field.Variable] = field.Answer
	}

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

		filled, err := FillDocxVariables(data, formFilledOps, opsAnswers)
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

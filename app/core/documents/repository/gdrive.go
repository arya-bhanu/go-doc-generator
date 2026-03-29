package repository

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/api/drive/v3"
)

// GDriveRepository abstracts Google Drive document fetching.
type GDriveRepository interface {
	FetchDocuments(docIDs []string) ([]DocumentFile, error)
}

// GDriveRepo is the concrete Drive-backed implementation of GDriveRepository.
type GDriveRepo struct {
	svc *drive.Service
}

// NewGDriveRepo constructs a GDriveRepo backed by the given Drive service.
func NewGDriveRepo(svc *drive.Service) *GDriveRepo {
	return &GDriveRepo{svc: svc}
}

// FetchDocuments downloads each Drive file, saves it to the OS temp directory,
// and returns a DocumentFile per ID. The title has the form:
//
//	<uuidv6>_<original-name-without-extension>.docx
func (r *GDriveRepo) FetchDocuments(docIDs []string) ([]DocumentFile, error) {
	result := make([]DocumentFile, 0, len(docIDs))

	for _, id := range docIDs {
		// 1. Fetch file metadata to get the original name.
		meta, err := r.svc.Files.Get(id).Fields("name").Do()
		if err != nil {
			return nil, fmt.Errorf("gdrive: get metadata %q: %w", id, err)
		}

		// 2. Download the raw file bytes.
		resp, err := r.svc.Files.Get(id).Download()
		if err != nil {
			return nil, fmt.Errorf("gdrive: download %q: %w", id, err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("gdrive: read %q: %w", id, err)
		}

		// 3. Build a UUID v6 title: <uuidv6>_<filename-without-ext>.docx
		uid, err := uuid.NewV6()
		if err != nil {
			return nil, fmt.Errorf("gdrive: generate uuid for %q: %w", id, err)
		}
		baseName := strings.TrimSuffix(meta.Name, filepath.Ext(meta.Name))
		title := fmt.Sprintf("%s_%s.docx", uid.String(), baseName)

		// 4. Persist to temp directory.
		if _, err := SaveToTemp(title, data); err != nil {
			return nil, err
		}

		result = append(result, DocumentFile{Title: title, Data: data})
	}

	return result, nil
}

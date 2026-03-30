package repository

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/api/drive/v3"
)

type GDriveRepository interface {
	FetchDocuments(docIDs []string) ([]DocumentFile, error)
}

type GDriveRepo struct {
	svc *drive.Service
}

func NewGDriveRepo(svc *drive.Service) *GDriveRepo {
	return &GDriveRepo{svc: svc}
}

func (r *GDriveRepo) FetchDocuments(docIDs []string) ([]DocumentFile, error) {
	result := make([]DocumentFile, 0, len(docIDs))

	for _, id := range docIDs {
		meta, err := r.svc.Files.Get(id).Fields("name").Do()
		if err != nil {
			return nil, fmt.Errorf("gdrive: get metadata %q: %w", id, err)
		}

		resp, err := r.svc.Files.Get(id).Download()
		if err != nil {
			return nil, fmt.Errorf("gdrive: download %q: %w", id, err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("gdrive: read %q: %w", id, err)
		}

		uid, err := uuid.NewV6()
		if err != nil {
			return nil, fmt.Errorf("gdrive: generate uuid for %q: %w", id, err)
		}
		baseName := strings.TrimSuffix(meta.Name, filepath.Ext(meta.Name))
		title := fmt.Sprintf("%s_%s.docx", uid.String(), baseName)

		if _, err := SaveToTemp(title, data); err != nil {
			return nil, err
		}

		result = append(result, DocumentFile{Title: title, Data: data, DocID: id, OriginalTitle: baseName})
	}

	return result, nil
}

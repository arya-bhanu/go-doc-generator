package repository

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/drive/v3"
)

type DriveFileInfo struct {
	ID          string
	Name        string
	WebViewLink string
}

type GDriveRepository interface {
	FetchDocuments(docIDs []string, docFilechan chan<- DocumentFile) error
	ListFolderDocuments(folderID string) ([]DriveFileInfo, error)
}

type GDriveRepo struct {
	svc *drive.Service
}

func NewGDriveRepo(svc *drive.Service) *GDriveRepo {
	return &GDriveRepo{svc: svc}
}

func (r *GDriveRepo) FetchDocuments(docIDs []string, docFilechan chan<- DocumentFile) error {
	docIDSChan := make(chan string)

	var g errgroup.Group

	go func() {
		defer close(docIDSChan)
		for _, docID := range docIDs {
			docIDSChan <- docID
		}
	}()

	for range 5 {
		g.Go(func() error {
			for id := range docIDSChan {
				meta, err := r.svc.Files.Get(id).Fields("name").Do()
				if err != nil {
					return fmt.Errorf("gdrive: get metadata %q: %w", id, err)
				}

				resp, err := r.svc.Files.Get(id).Download()
				if err != nil {
					return fmt.Errorf("gdrive: download %q: %w", id, err)
				}
				data, err := io.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil {
					return fmt.Errorf("gdrive: read %q: %w", id, err)
				}

				uid, err := uuid.NewV6()
				if err != nil {
					return fmt.Errorf("gdrive: generate uuid for %q: %w", id, err)
				}
				baseName := strings.TrimSuffix(meta.Name, filepath.Ext(meta.Name))
				title := fmt.Sprintf("%s_%s.docx", uid.String(), baseName)

				if _, err := SaveToTemp(title, data); err != nil {
					return err
				}

				docFilechan <- DocumentFile{Title: title, Data: data, DocID: id, OriginalTitle: baseName}
			}
			return nil
		})
	}

	return g.Wait()
}

func (r *GDriveRepo) ListFolderDocuments(folderID string) ([]DriveFileInfo, error) {
	const docxMIME = "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	query := fmt.Sprintf(
		"'%s' in parents and mimeType='%s' and trashed=false",
		folderID, docxMIME,
	)

	var results []DriveFileInfo
	pageToken := ""

	for {
		req := r.svc.Files.List().
			Q(query).
			Fields("nextPageToken, files(id, name, webViewLink)").
			PageSize(100)
		if pageToken != "" {
			req = req.PageToken(pageToken)
		}

		res, err := req.Do()
		if err != nil {
			return nil, fmt.Errorf("gdrive: list folder %q: %w", folderID, err)
		}

		for _, f := range res.Files {
			baseName := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
			results = append(results, DriveFileInfo{
				ID:          f.Id,
				Name:        baseName,
				WebViewLink: f.WebViewLink,
			})
		}

		if res.NextPageToken == "" {
			break
		}
		pageToken = res.NextPageToken
	}

	return results, nil
}

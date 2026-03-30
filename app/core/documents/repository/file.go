package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

type DocumentFile struct {
	Title         string // "<uuidv6>_<original-filename-without-ext>.docx"
	Data          []byte
	DocID         string
	OriginalTitle string
}

func SaveToTemp(title string, data []byte) (string, error) {
	if err := os.MkdirAll("temp", 0755); err != nil {
		return "", fmt.Errorf("file: create temp dir: %w", err)
	}
	path := filepath.Join("temp", title)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("file: save temp %q: %w", title, err)
	}
	return path, nil
}

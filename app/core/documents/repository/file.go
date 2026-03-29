package repository

import (
	"fmt"
	"os"
	"path/filepath"
)

// DocumentFile holds the raw bytes of a downloaded document together with
// the unique temporary file name assigned to it.
type DocumentFile struct {
	Title string // "<uuidv6>_<original-filename-without-ext>.docx"
	Data  []byte
}

// SaveToTemp writes data to the OS temporary directory using title as the
// file name. It returns the full path to the created file.
// The caller is responsible for removing the file when it is no longer needed.
func SaveToTemp(title string, data []byte) (string, error) {
	path := filepath.Join("temp", title)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("file: save temp %q: %w", title, err)
	}
	return path, nil
}

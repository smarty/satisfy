package packaging

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/smarty/satisfy/v2/internal/model"
)

type FileListingIntegrityChecker struct {
	fileSystem model.FileChecker
}

func NewFileListingIntegrityChecker(fileSystem model.FileChecker) *FileListingIntegrityChecker {
	return &FileListingIntegrityChecker{fileSystem: fileSystem}
}

func (this *FileListingIntegrityChecker) Verify(_ context.Context, manifest model.Manifest, localPath string) error {
	for _, item := range manifest.Archive.Contents {
		fullPath := filepath.Join(localPath, item.Path)
		fileInfo, err := this.fileSystem.Stat(fullPath)
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("filename not found for \"%s\"", fullPath)
		}
		if err != nil {
			return err
		}
		if item.Size != fileInfo.Size() {
			return fmt.Errorf("file size mismatch for \"%s\"(expected: [%d], actual: [%d])", fullPath, item.Size, fileInfo.Size())
		}
	}
	return nil
}

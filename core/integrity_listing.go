package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smartystreets/logging"
	"github.com/smartystreets/satisfy/contracts"
)

type FileListingIntegrityChecker struct {
	fileSystem contracts.FileChecker
	logger     *logging.Logger
}

func NewFileListingIntegrityChecker(fileSystem contracts.FileChecker) *FileListingIntegrityChecker {
	return &FileListingIntegrityChecker{fileSystem: fileSystem}
}

func (this *FileListingIntegrityChecker) Verify(manifest contracts.Manifest, localPath string) error {
	for _, item := range manifest.Archive.Contents {
		fullPath := filepath.Join(localPath, item.Path)
		fileInfo, err := this.fileSystem.Stat(fullPath)
		if os.IsNotExist(err) {
			return fmt.Errorf("filename not found for \"%s\"", fullPath)
		}
		if item.Size != fileInfo.Size() {
			return fmt.Errorf("file size mismatch for \"%s\"(expected: [%d], actual: [%d])", fullPath, item.Size, fileInfo.Size())
		}
	}
	this.logger.Printf("Listing integrity check passed: [%s @ %s]", manifest.Name, manifest.Version)
	return nil
}

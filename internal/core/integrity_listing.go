package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
)

type FileListingIntegrityChecker struct {
	fileSystem plumbing.FileChecker
	emit       func(contracts.Event)
}

func NewFileListingIntegrityChecker(fileSystem plumbing.FileChecker, emit func(contracts.Event)) *FileListingIntegrityChecker {
	if emit == nil {
		emit = func(contracts.Event) {}
	}
	return &FileListingIntegrityChecker{fileSystem: fileSystem, emit: emit}
}

func (this *FileListingIntegrityChecker) Verify(manifest plumbing.Manifest, localPath string) error {
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
	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Listing integrity check passed: [%s @ %s]", manifest.Name, manifest.Version)})
	return nil
}

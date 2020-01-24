package core

import (
	"fmt"
	"path/filepath"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type FileListingIntegrityChecker struct {
	fileSystem contracts.PathLister
}

func NewFileListingIntegrityChecker(fileSystem contracts.PathLister) *FileListingIntegrityChecker {
	return &FileListingIntegrityChecker{fileSystem: fileSystem}
}

func (this *FileListingIntegrityChecker) Verify(manifest contracts.Manifest, localPath string) error {
	files := this.buildFileMap()

	for _, item := range manifest.Archive.Contents {
		fullPath := filepath.Join(localPath, item.Path)
		if _, found := files[fullPath]; !found {
			return fmt.Errorf("filename not found for \"%s\" in [%s @ %s]",
				fullPath, manifest.Name, manifest.Version)
		}
		if item.Size != files[fullPath].Size() {
			return fmt.Errorf("file size mismatch for \"%s\" in [%s @ %s]",
				fullPath, manifest.Name, manifest.Version)
		}
	}

	return nil
}

func (this *FileListingIntegrityChecker) buildFileMap() map[string]contracts.FileInfo {
	files := make(map[string]contracts.FileInfo)
	for _, file := range this.fileSystem.Listing() {
		files[file.Path()] = file
	}
	return files
}

package core

import (
	"errors"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type FileListingIntegrityChecker struct {
	fileSystem contracts.PathLister
}

func NewFileListingIntegrityChecker(fileSystem contracts.PathLister) *FileListingIntegrityChecker {
	return &FileListingIntegrityChecker{fileSystem: fileSystem}
}

func (this *FileListingIntegrityChecker) Verify(manifest contracts.Manifest) error {
	files := this.buildFileMap()

	for _, item := range manifest.Archive.Contents {
		if _, found := files[item.Path]; !found {
			return errFileNotFound
		}
		if item.Size != files[item.Path].Size() {
			return errFileSizeMismatch
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

var (
	errFileNotFound     = errors.New("filename not found")
	errFileSizeMismatch = errors.New("file size mismatch")
)

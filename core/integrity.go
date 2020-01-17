package core

import "bitbucket.org/smartystreets/satisfy/contracts"

type FileListingIntegrityChecker struct {
	fileSystem contracts.FileSystem
}

func NewFileListingIntegrityChecker(fileSystem contracts.FileSystem) *FileListingIntegrityChecker {
	return &FileListingIntegrityChecker{fileSystem: fileSystem}
}

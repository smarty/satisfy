package core

import (
	"bytes"
	"errors"
	"hash"
	"io"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type FileContentIntegrityCheck struct {
	hasher hash.Hash
	fileSystem contracts.FileSystem
}

func NewFileContentIntegrityCheck(hasher hash.Hash, fileSystem contracts.FileSystem) *FileContentIntegrityCheck {
	return &FileContentIntegrityCheck{hasher: hasher, fileSystem:fileSystem}
}

func (this *FileContentIntegrityCheck) Verify(manifest contracts.Manifest) error {
	for _, item := range manifest.Archive.Contents {
		this.hasher.Reset()
		reader := this.fileSystem.Open(item.Path)
		_, err := io.Copy(this.hasher, reader)
		if err != nil {
			return err
		}
		err = reader.Close()
		if err != nil {
			return err
		}
		checksum := this.hasher.Sum(nil)
		if bytes.Compare(checksum, item.MD5Checksum) != 0 {
			return errors.New("checksum mismatch")
		}
	}
	return nil
}

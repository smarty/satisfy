package core

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"path/filepath"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type FileContentIntegrityCheck struct {
	hasher     func() hash.Hash
	fileSystem contracts.FileOpener
	enabled    bool
}

func NewFileContentIntegrityCheck(hasher func() hash.Hash, fileSystem contracts.FileOpener, enabled bool) *FileContentIntegrityCheck {
	return &FileContentIntegrityCheck{hasher: hasher, fileSystem: fileSystem, enabled: enabled}
}

func (this *FileContentIntegrityCheck) Verify(manifest contracts.Manifest, localPath string) error {
	if !this.enabled {
		return nil
	}
	for _, item := range manifest.Archive.Contents {
		hasher := this.hasher()
		reader := this.fileSystem.Open(filepath.Join(localPath, item.Path))
		_, err := io.Copy(hasher, reader)
		if err != nil {
			return err
		}
		err = reader.Close()
		if err != nil {
			return err
		}
		checksum := hasher.Sum(nil)
		if bytes.Compare(checksum, item.MD5Checksum) != 0 {
			return fmt.Errorf("checksum mismatch for \"%s\" in [%s @ %s]", item.Path, manifest.Name, manifest.Version)
		}
	}
	return nil
}

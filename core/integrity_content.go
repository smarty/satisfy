package core

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"path/filepath"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/logging"
)

type FileOpenChecker interface {
	contracts.FileOpener
	contracts.FileChecker
}

type FileContentIntegrityCheck struct {
	hasher     func() hash.Hash
	fileSystem FileOpenChecker
	enabled    bool
	logger     *logging.Logger
}

func NewFileContentIntegrityCheck(hasher func() hash.Hash, fileSystem FileOpenChecker, enabled bool) *FileContentIntegrityCheck {
	return &FileContentIntegrityCheck{hasher: hasher, fileSystem: fileSystem, enabled: enabled}
}

func (this *FileContentIntegrityCheck) Verify(manifest contracts.Manifest, localPath string) error {
	if !this.enabled {
		return nil
	}
	for _, item := range manifest.Archive.Contents {
		checksum, err := this.calculateChecksum(filepath.Join(localPath, item.Path))
		if err != nil {
			return err
		}
		if bytes.Compare(checksum, item.MD5Checksum) != 0 {
			return fmt.Errorf("checksum mismatch for \"%s\"", item.Path)
		}
	}
	this.logger.Printf("Content integrity check passed: [%s @ %s]", manifest.Name, manifest.Version)
	return nil
}

func (this *FileContentIntegrityCheck) calculateChecksum(path string) ([]byte, error) {
	hasher := this.hasher()
	info, _ := this.fileSystem.Stat(path)
	if info.Symlink() != "" {
		_, err := io.WriteString(hasher, info.Symlink())
		if err != nil {
			return nil, err
		}
	} else {
		reader := this.fileSystem.Open(path)
		_, err := io.Copy(hasher, reader)
		if err != nil {
			return nil, err
		}
		err = reader.Close()
		if err != nil {
			return nil, err
		}
	}
	checksum := hasher.Sum(nil)
	return checksum, nil
}

package packaging

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
	"io"
	"path/filepath"

	"github.com/smarty/satisfy/v2/internal/model"
)

type FileOpenChecker interface {
	model.FileOpener
	model.FileChecker
}

type FileContentIntegrityCheck struct {
	hasher     func() hash.Hash
	fileSystem FileOpenChecker
	enabled    bool
}

func NewFileContentIntegrityCheck(hasher func() hash.Hash, fileSystem FileOpenChecker, enabled bool) *FileContentIntegrityCheck {
	return &FileContentIntegrityCheck{hasher: hasher, fileSystem: fileSystem, enabled: enabled}
}

func (this *FileContentIntegrityCheck) Verify(manifest model.Manifest, localPath string) error {
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
	return nil
}

func (this *FileContentIntegrityCheck) calculateChecksum(path string) ([]byte, error) {
	hasher := this.hasher()
	info, err := this.fileSystem.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Symlink() != "" {
		_, err := io.WriteString(hasher, info.Symlink())
		if err != nil {
			return nil, err
		}
	} else {
		reader, err := this.fileSystem.Open(path)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(hasher, reader)
		closeErr := reader.Close()
		if err = errors.Join(err, closeErr); err != nil {
			return nil, err
		}
	}
	checksum := hasher.Sum(nil)
	return checksum, nil
}

package core

import (
	"bytes"
	"fmt"
	"hash"
	"io"
	"path/filepath"

	"github.com/smarty/satisfy/contracts"
	"github.com/smarty/satisfy/internal/plumbing"
)

type FileOpenChecker interface {
	plumbing.FileOpener
	plumbing.FileChecker
}

type FileContentIntegrityCheck struct {
	hasher     func() hash.Hash
	fileSystem FileOpenChecker
	emit       func(contracts.Event)
	enabled    bool
}

func NewFileContentIntegrityCheck(hasher func() hash.Hash, fileSystem FileOpenChecker, enabled bool, emit func(contracts.Event)) *FileContentIntegrityCheck {
	if emit == nil {
		emit = func(contracts.Event) {}
	}
	return &FileContentIntegrityCheck{hasher: hasher, fileSystem: fileSystem, enabled: enabled, emit: emit}
}

func (this *FileContentIntegrityCheck) Verify(manifest plumbing.Manifest, localPath string) error {
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
	this.emit(contracts.Event{Type: contracts.EventInfo, Message: fmt.Sprintf("Content integrity check passed: [%s @ %s]", manifest.Name, manifest.Version)})
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
		reader, err := this.fileSystem.Open(path)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(hasher, reader)
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

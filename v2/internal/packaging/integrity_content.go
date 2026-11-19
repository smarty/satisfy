package packaging

import (
	"bytes"
	"context"
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

// Verify hashes every file of the manifest. It stops with ctx's error once
// ctx is done, checking between files and while hashing each file.
func (this *FileContentIntegrityCheck) Verify(ctx context.Context, manifest model.Manifest, localPath string) error {
	if !this.enabled {
		return nil
	}
	for _, item := range manifest.Archive.Contents {
		if err := ctx.Err(); err != nil {
			return err
		}
		checksum, err := this.calculateChecksum(ctx, filepath.Join(localPath, item.Path))
		if err != nil {
			return err
		}
		if !bytes.Equal(checksum, item.MD5Checksum) {
			return fmt.Errorf("checksum mismatch for \"%s\"", item.Path)
		}
	}
	return nil
}

func (this *FileContentIntegrityCheck) calculateChecksum(ctx context.Context, path string) ([]byte, error) {
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
		_, err = io.Copy(hasher, contextReader{ctx: ctx, reader: reader})
		closeErr := reader.Close()
		if err = errors.Join(err, closeErr); err != nil {
			return nil, err
		}
	}
	checksum := hasher.Sum(nil)
	return checksum, nil
}

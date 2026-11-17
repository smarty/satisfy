package archive

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zip"
	"github.com/smarty/satisfy/v2/internal/model"
)

type ZipArchiveReader struct {
	checksumReader       io.Reader
	currentZipFileReader io.Reader
	zipReader            *zip.Reader
	currentFileCount     int
	lastBytesRetrieved   *bytes.Buffer
	size                 int64
	file                 *os.File
	progress             model.ProgressFunc
}

func (this *ZipArchiveReader) Next() (*tar.Header, error) {
	if this.zipReader == nil {
		reader, err := zip.NewReader(this.file, this.size)
		if err != nil {
			return nil, err
		}
		this.zipReader = reader
	}

	if this.currentFileCount >= len(this.zipReader.File) {
		return nil, io.EOF
	}

	zipHeader := this.zipReader.File[this.currentFileCount]
	this.currentFileCount++

	reader, err := zipHeader.Open()
	if err != nil {
		return nil, err
	}
	this.currentZipFileReader = reader
	return &tar.Header{
		Typeflag:   0,
		Name:       zipHeader.Name,
		Linkname:   "",
		Size:       int64(zipHeader.UncompressedSize64),
		Mode:       0644,
		Uid:        0,
		Gid:        0,
		Uname:      "",
		Gname:      "",
		ModTime:    zipHeader.Modified,
		AccessTime: zipHeader.Modified,
		ChangeTime: zipHeader.Modified,
		Devmajor:   0,
		Devminor:   0,
		PAXRecords: nil,
		Format:     0,
	}, nil
}

func (this *ZipArchiveReader) DownloadArchiveToTemp(reader io.Reader) error {
	tmp, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return err
	}

	this.file = tmp

	progress := NewArchiveProgressCounter(this.size, func(written, total int64, done bool) {
		this.progress.Report(model.Progress{
			Phase: model.ProgressDownloading, Name: "zip archive", Written: written, Total: total, Done: done,
		})
	})

	multiWriter := io.MultiWriter(tmp, progress)
	this.size, err = io.Copy(multiWriter, reader)

	if err != nil {
		return errors.Join(err, tmp.Close())
	}

	err = progress.Close()
	if err != nil {
		return err
	}

	_, err = tmp.Seek(0, io.SeekStart)
	return err
}

func (this *ZipArchiveReader) Read(p []byte) (n int, err error) {
	return this.currentZipFileReader.Read(p)
}

func (this *ZipArchiveReader) ReadAt(p []byte, off int64) (int, error) {
	return this.file.ReadAt(p, off)
}

func (this *ZipArchiveReader) Close() error {
	if this.file != nil {
		name := this.file.Name()
		closeErr := this.file.Close()
		if errors.Is(closeErr, os.ErrClosed) {
			closeErr = nil
		}
		return errors.Join(closeErr, os.Remove(name))
	}
	return nil
}

func NewZipArchiveReader(reader io.Reader, progress model.ProgressFunc) (io.ReadCloser, error) {
	zipArchiveReader := &ZipArchiveReader{checksumReader: reader, progress: progress}
	err := zipArchiveReader.DownloadArchiveToTemp(reader)
	if err != nil {
		_ = zipArchiveReader.Close()
		return nil, fmt.Errorf("failed to download zip archive: %w", err)
	}
	return zipArchiveReader, nil
}

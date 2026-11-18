package archive

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zip"
	"github.com/smarty/satisfy/v2/internal/model"
)

type ZipArchiveReader struct {
	currentZipFileReader io.Reader
	zipReader            *zip.Reader
	currentFileCount     int
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

	var (
		typeflag byte
		linkname string
		size     = int64(zipHeader.UncompressedSize64)
		mode     = int64(0644)
	)
	switch fileMode := zipHeader.Mode(); {
	case fileMode&os.ModeSymlink != 0:
		target, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		typeflag, linkname, size = tar.TypeSymlink, string(target), 0
	case model.IsExecutable(fileMode):
		mode = 0755
	}
	return &tar.Header{
		Typeflag:   typeflag,
		Name:       zipHeader.Name,
		Linkname:   linkname,
		Size:       size,
		Mode:       mode,
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

	progress := NewProgressCounter(this.progress, model.ProgressDownloading, "zip archive", this.size)

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
	zipArchiveReader := &ZipArchiveReader{progress: progress}
	err := zipArchiveReader.DownloadArchiveToTemp(reader)
	if err != nil {
		_ = zipArchiveReader.Close()
		return nil, fmt.Errorf("failed to download zip archive: %w", err)
	}
	return zipArchiveReader, nil
}

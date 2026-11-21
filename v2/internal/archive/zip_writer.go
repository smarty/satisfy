package archive

import (
	"archive/zip"
	"io"
	"os"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/smarty/satisfy/v2/internal/model"
)

type ZipArchiveWriter struct {
	inner   *zip.Writer
	current io.Writer
	once    sync.Once
}

func NewZipArchiveWriter(writer io.Writer, level int) model.ArchiveWriter {
	inner := zip.NewWriter(writer)
	inner.RegisterCompressor(zip.Deflate, func(target io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(target, level)
	})
	return &ZipArchiveWriter{inner: inner}
}

// WriteHeader starts an entry. Like the tar writer, it records executables
// with mode 0755 and other files with 0644; a symlink is recorded with
// os.ModeSymlink and its target as the entry's content (the zip convention).
func (this *ZipArchiveWriter) WriteHeader(header model.ArchiveHeader) (err error) {
	fileHeader := &zip.FileHeader{
		Name:               header.Name,
		Modified:           header.ModTime,
		UncompressedSize64: uint64(header.Size),
		Method:             zip.Deflate,
	}
	switch {
	case header.LinkName != "":
		fileHeader.SetMode(os.ModeSymlink | 0777)
		fileHeader.UncompressedSize64 = uint64(len(header.LinkName))
	case header.Executable:
		fileHeader.SetMode(0755)
	default:
		fileHeader.SetMode(0644)
	}
	this.current, err = this.inner.CreateHeader(fileHeader)
	if err != nil || header.LinkName == "" {
		return err
	}
	_, err = io.WriteString(this.current, header.LinkName)
	return err
}
func (this *ZipArchiveWriter) Write(buffer []byte) (int, error) {
	return this.current.Write(buffer)
}
func (this *ZipArchiveWriter) Close() (err error) {
	this.current = nil
	this.once.Do(func() { err = this.inner.Close() })
	return err
}

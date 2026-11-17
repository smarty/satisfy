package shell

import (
	"archive/zip"
	"io"
	"sync"

	"github.com/klauspost/compress/flate"
	"github.com/smarty/satisfy/v2/contracts"
)

type ZipArchiveWriter struct {
	inner   *zip.Writer
	current io.Writer
	once    sync.Once
}

func NewZipArchiveWriter(writer io.Writer, level int) contracts.ArchiveWriter {
	inner := zip.NewWriter(writer)
	inner.RegisterCompressor(zip.Deflate, func(target io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(target, level)
	})
	return &ZipArchiveWriter{inner: inner}
}

func (this *ZipArchiveWriter) WriteHeader(header contracts.ArchiveHeader) (err error) {
	this.current, err = this.inner.CreateHeader(&zip.FileHeader{
		Name:               header.Name,
		Modified:           header.ModTime,
		UncompressedSize64: uint64(header.Size),
		Method:             zip.Deflate,
	})
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

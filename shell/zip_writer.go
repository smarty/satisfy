package shell

import (
	"archive/zip"
	"compress/flate"
	"io"
	"sync"

	"github.com/smarty/satisfy/contracts"
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

func (this *ZipArchiveWriter) WriteHeader(header contracts.ArchiveHeader) {
	var err error

	this.current, err = this.inner.CreateHeader(&zip.FileHeader{
		Name:               header.Name,
		Modified:           header.ModTime,
		UncompressedSize64: uint64(header.Size),
		Method:             zip.Deflate,
	})

	if err != nil {
		panic(err)
	}
}
func (this *ZipArchiveWriter) Write(buffer []byte) (int, error) {
	return this.current.Write(buffer)
}
func (this *ZipArchiveWriter) Close() (err error) {
	this.current = nil
	this.once.Do(func() { err = this.inner.Close() })
	return err
}

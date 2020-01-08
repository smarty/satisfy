package archive

import (
	"archive/tar"
	"io"
	"log"
)

type TarArchiveWriter struct {
	writer *tar.Writer
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(name string, size, mode int64) {
	err := this.writer.WriteHeader(&tar.Header{Name:name, Size: size, Mode: 0644})
	if err != nil {
		log.Panic(err)
	}
}

func (this *TarArchiveWriter) Write(p []byte) (n int, err error) {
	return this.writer.Write(p)
}

func (this *TarArchiveWriter) Close() error {
	return this.writer.Close()
}

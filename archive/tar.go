package archive

import (
	"archive/tar"
	"io"
	"log"
)

type TarArchiveWriter struct {
	*tar.Writer
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{Writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(name string, size int64) {
	err := this.Writer.WriteHeader(&tar.Header{Name:name, Size: size, Mode: 0644})
	if err != nil {
		log.Panic(err)
	}
}


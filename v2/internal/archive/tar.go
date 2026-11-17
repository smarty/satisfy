package archive

import (
	"archive/tar"
	"io"

	"github.com/smarty/satisfy/v2/internal/model"
)

type TarArchiveWriter struct {
	*tar.Writer
}

func NewSwitchArchiveWriter(writer io.Writer) model.ArchiveWriter {
	if inner, ok := writer.(model.ArchiveWriter); ok {
		return inner
	} else {
		return NewTarArchiveWriter(writer)
	}
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{Writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(header model.ArchiveHeader) error {
	tarHeader := &tar.Header{
		Name:    header.Name,
		Size:    header.Size,
		ModTime: header.ModTime,
		Mode:    0644,
	}
	if header.LinkName != "" {
		tarHeader.Linkname = header.LinkName
		tarHeader.Typeflag = tar.TypeSymlink
	}
	if header.Executable {
		tarHeader.Mode = 0755
	}
	return this.Writer.WriteHeader(tarHeader)
}

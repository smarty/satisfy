package shell

import (
	"archive/tar"
	"io"

	"github.com/smarty/satisfy/internal/plumbing"
)

type TarArchiveWriter struct {
	*tar.Writer
}

func NewSwitchArchiveWriter(writer io.Writer) plumbing.ArchiveWriter {
	if inner, ok := writer.(plumbing.ArchiveWriter); ok {
		return inner
	} else {
		return NewTarArchiveWriter(writer)
	}
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{Writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(header plumbing.ArchiveHeader) error {
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

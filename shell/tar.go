package shell

import (
	"archive/tar"
	"io"
	"log"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type TarArchiveWriter struct {
	*tar.Writer
}

func NewTarArchiveWriter(writer io.Writer) *TarArchiveWriter {
	return &TarArchiveWriter{Writer: tar.NewWriter(writer)}
}

func (this *TarArchiveWriter) WriteHeader(header contracts.ArchiveHeader) {
	// TODO: currently if we receive a symlink, it gets the symlink size
	// but then file.Open gets the actual stream (e.g. 500MB)
	// whereas the symlink pointer is just a few bytes long
	// this results in a tar error: "WriteTooLong"
	// https://stackoverflow.com/questions/38454850/getting-write-too-long-error-when-trying-to-create-tar-gz-file-from-file-and-d
	tarHeader := &tar.Header{
		Name:    header.Name,
		Size:    header.Size,
		ModTime: header.ModTime,
		Mode:    0644,
	}
	err := this.Writer.WriteHeader(tarHeader)
	if err != nil {
		log.Panic(err)
	}
}

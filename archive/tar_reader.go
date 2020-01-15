package archive

import (
	"archive/tar"
	"io"
	"log"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type TarArchiveReader struct {
	reader *tar.Reader
	header *tar.Header
}

func NewTarArchiveReader(reader io.Reader) *TarArchiveReader {
	return &TarArchiveReader{reader: tar.NewReader(reader)}
}

func (this *TarArchiveReader) Next() bool {
	tarHeader, err := this.reader.Next()
	if err == io.EOF {
		return false
	}
	if err != nil {
		log.Panic(err)
	}
	this.header = tarHeader
	return true
}

func (this *TarArchiveReader) Header() contracts.ArchiveHeader {
	return contracts.ArchiveHeader{
		Name:    this.header.Name,
		Size:    this.header.Size,
		ModTime: this.header.ModTime,
	}
}

func (this *TarArchiveReader) Reader() io.Reader {
	return this.reader
}

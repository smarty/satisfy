package shell

import (
	"archive/tar"
	"bytes"
	"github.com/klauspost/compress/zip"
	"github.com/smarty/satisfy/contracts"
	"io"
	"net/url"
)

type ZipArchiveReader struct {
	checksumReader       io.Reader
	currentZipFileReader io.Reader
	zipReader            *zip.Reader
	archiveURL           url.URL
	downloader           contracts.Downloader
	currentFileCount     int
	offset               int64
	lastBytesRetrieved   *bytes.Buffer
	lastOffset           int64
	size                 int64
}

func (this *ZipArchiveReader) Next() (*tar.Header, error) {
	size, sizeError := this.downloader.Size(this.archiveURL)
	this.size = size
	if sizeError != nil {
		return nil, sizeError
	}
	var readerError error
	this.zipReader, readerError = zip.NewReader(this, size)
	if readerError != nil {
		return nil, readerError
	}
	this.currentFileCount++
	if len(this.zipReader.File) < this.currentFileCount {
		return nil, io.EOF
	}
	zipHeader := this.zipReader.File[this.currentFileCount-1]
	reader, err := this.zipReader.File[this.currentFileCount-1].Open()
	if err != nil {
		return nil, err
	}
	this.currentZipFileReader = reader
	return &tar.Header{
		Typeflag:   0,
		Name:       zipHeader.Name,
		Linkname:   "",
		Size:       int64(zipHeader.UncompressedSize64),
		Mode:       0644,
		Uid:        0,
		Gid:        0,
		Uname:      "",
		Gname:      "",
		ModTime:    zipHeader.Modified,
		AccessTime: zipHeader.Modified,
		ChangeTime: zipHeader.Modified,
		Devmajor:   0,
		Devminor:   0,
		Xattrs:     nil,
		PAXRecords: nil,
		Format:     0,
	}, nil
}

func (this *ZipArchiveReader) SetDownloader(request url.URL, downloader contracts.Downloader) {
	this.archiveURL = request
	this.downloader = downloader
}

func (this *ZipArchiveReader) Read(p []byte) (n int, err error) {
	n, err = this.checksumReader.Read(p)
	if err != nil {
		if err != io.EOF {
			return 0, err
		}
	}
	return this.currentZipFileReader.Read(p)
}

func (this *ZipArchiveReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 {
		return 0, io.EOF
	}

	if this.lastBytesRetrieved != nil && off > this.lastOffset {
		end := off + int64(len(p))
		if end <= this.lastOffset+int64(this.lastBytesRetrieved.Len()) {
			start := off - this.lastOffset
			copy(p, this.lastBytesRetrieved.Bytes()[start:end-this.lastOffset])
			return len(p), nil
		}
	}

	// Make a 10 MB minimum seek call. This reduces network calls. We'll use a buffer for any other
	// calls within this 10 MB window.
	wanted := 1024 * 1024 * 10
	if wanted < len(p) {
		wanted = len(p)
	}

	reader, err := this.downloader.Seek(this.archiveURL, off, off+int64(wanted))
	if err != nil {
		return 0, err
	}

	// Seek must always be closed at the end of this method.
	defer func(reader io.ReadCloser) {
		cErr := reader.Close()
		if err == nil && cErr != nil {
			err = cErr
		}
	}(reader)

	if this.lastBytesRetrieved == nil {
		this.lastBytesRetrieved = &bytes.Buffer{}
	} else {
		this.lastBytesRetrieved.Reset()
	}

	var numRead int64
	numRead, err = this.lastBytesRetrieved.ReadFrom(reader)
	if numRead < 0 {
		return 0, io.EOF
	}
	if err != nil {
		return 0, err
	}

	this.lastOffset = off

	if this.lastBytesRetrieved.Len() < len(p) {
		n = this.lastBytesRetrieved.Len()
		copy(p, this.lastBytesRetrieved.Bytes()[0:n])
	} else {
		n = len(p)
		copy(p, this.lastBytesRetrieved.Bytes())
	}

	if err == io.EOF && n == len(p) {
		err = nil
	}

	return n, err
}

func (this *ZipArchiveReader) Close() error {
	return nil
}

func NewZipArchiveReader(reader io.Reader) io.ReadCloser {
	return &ZipArchiveReader{checksumReader: reader}
}

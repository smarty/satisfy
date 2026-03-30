package shell

import (
	"archive/tar"
	"bytes"
	"io"
	"net/url"
	"os"

	"github.com/klauspost/compress/zip"
	"github.com/smarty/satisfy/internal/plumbing"
)

type ZipArchiveReader struct {
	checksumReader       io.Reader
	currentZipFileReader io.Reader
	zipReader            *zip.Reader
	archiveURL           url.URL
	downloader           plumbing.Downloader
	newProgress          func(int64) io.WriteCloser
	currentFileCount     int
	lastBytesRetrieved   *bytes.Buffer
	size                 int64
	file                 *os.File
}

func (this *ZipArchiveReader) Next() (*tar.Header, error) {
	if this.zipReader == nil {
		reader, err := zip.NewReader(this.file, this.size)
		if err != nil {
			return nil, err
		}
		this.zipReader = reader
	}

	if this.currentFileCount >= len(this.zipReader.File) {
		return nil, io.EOF
	}

	zipHeader := this.zipReader.File[this.currentFileCount]
	this.currentFileCount++

	reader, err := zipHeader.Open()
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
		PAXRecords: nil,
		Format:     0,
	}, nil
}

func (this *ZipArchiveReader) SetDownloader(request url.URL, downloader plumbing.Downloader) {
	this.archiveURL = request
	this.downloader = downloader
}

func (this *ZipArchiveReader) DownloadArchiveToTemp(reader io.Reader) error {
	tmp, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return err
	}

	this.file = tmp

	progress := this.newProgress(this.size)

	multiWriter := io.MultiWriter(tmp, progress)
	this.size, err = io.Copy(multiWriter, reader)

	if err != nil {
		err := tmp.Close()
		if err != nil {
			return err
		}
		return err
	}

	err = progress.Close()
	if err != nil {
		return err
	}

	_, err = tmp.Seek(0, io.SeekStart)
	return err
}

func (this *ZipArchiveReader) Read(p []byte) (n int, err error) {
	return this.currentZipFileReader.Read(p)
}

func (this *ZipArchiveReader) ReadAt(p []byte, off int64) (int, error) {
	return this.file.ReadAt(p, off)
}

func (this *ZipArchiveReader) Close() error {
	if this.file != nil {
		name := this.file.Name()
		err := this.file.Close()
		if err != nil {
			return err
		}
		return os.Remove(name)
	}
	return nil
}

func NewZipArchiveReader(reader io.Reader, newProgress func(int64) io.WriteCloser) (*ZipArchiveReader, error) {
	if newProgress == nil {
		newProgress = noopProgress
	}

	zipArchiveReader := &ZipArchiveReader{checksumReader: reader, newProgress: newProgress}
	if err := zipArchiveReader.DownloadArchiveToTemp(reader); err != nil {
		return nil, err
	}
	return zipArchiveReader, nil
}

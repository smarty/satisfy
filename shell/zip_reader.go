package shell

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/klauspost/compress/zip"
	"github.com/smarty/satisfy/cmd/archive_progress"
	"github.com/smarty/satisfy/contracts"
)

type ZipArchiveReader struct {
	checksumReader       io.Reader
	currentZipFileReader io.Reader
	zipReader            *zip.Reader
	archiveURL           url.URL
	downloader           contracts.Downloader
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

func (this *ZipArchiveReader) SetDownloader(request url.URL, downloader contracts.Downloader) {
	this.archiveURL = request
	this.downloader = downloader
}

func (this *ZipArchiveReader) DownloadArchiveToTemp(reader io.Reader) error {
	tmp, err := os.CreateTemp("", "archive-*.zip")
	if err != nil {
		return err
	}

	this.file = tmp

	progress := archive_progress.NewArchiveProgressCounter(this.size, func(archived, total string, done bool) {
		if done {
			fmt.Printf("\nDone downloading archive %s.\n", archived)
		} else {
			fmt.Printf("\033[2K\rDownloading archive... %s.", archived)
		}
	})

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

func NewZipArchiveReader(reader io.Reader) io.ReadCloser {
	zipArchiveReader := &ZipArchiveReader{checksumReader: reader}
	err := zipArchiveReader.DownloadArchiveToTemp(reader)
	if err != nil {
		log.Fatalf("failed to download zip archive: %s", err)
	}
	return zipArchiveReader
}

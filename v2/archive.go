package satisfy

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/url"

	"github.com/smarty/satisfy/v2/internal/model"
	"github.com/smarty/satisfy/v2/internal/packaging"
)

// Manifest returns the manifest of a package version without installing it.
// version may be a version, a tag, or "latest", resolved as Install resolves
// them: a version takes precedence over a tag of the same name. The result
// never carries the tag listing (see Tags). An unknown version or tag fails
// with an error matching ErrNotFound.
func (this *Client) Manifest(ctx context.Context, public PackageRef, version string) (Manifest, error) {
	manifest, err := this.resolveManifest(ctx, public, version)
	if err != nil {
		return Manifest{}, err
	}
	manifest.Tags = nil
	return publicManifest(manifest), nil
}

func (this *Client) resolveManifest(ctx context.Context, public PackageRef, version string) (model.Manifest, error) {
	ref, err := validatePackageRef(public)
	if err != nil {
		return model.Manifest{}, err
	}
	if version == "" {
		return model.Manifest{}, fmt.Errorf("%w: package version is required", ErrInvalidConfig)
	}
	if version == "latest" {
		return this.rootManifest(ctx, public)
	}

	manifest, err := this.downloadManifest(ctx, ref.ComposeManifestAddress(version))
	if !model.IsNotFound(err) {
		return manifest, err
	}
	root, err := this.rootManifest(ctx, public)
	if err != nil {
		return model.Manifest{}, fmt.Errorf("no version %q of package %q, and its tags could not be read: %w", version, ref.Name, err)
	}
	tagged, found := root.TagVersion(version)
	if !found {
		return model.Manifest{}, fmt.Errorf("no version or tag %q exists for package %q: %w", version, ref.Name, ErrNotFound)
	}
	this.logger.Info("resolved tag", "tag", version, "version", tagged, "package", ref.Name)
	return this.downloadManifest(ctx, ref.ComposeManifestAddress(tagged))
}

// OpenArchive streams the archive of the package version that manifest
// describes (see Manifest), without writing anything to disk. The caller must
// Close the returned reader. ctx governs the whole download: when the
// connection fails part-way, reading resumes where it stopped, using ctx.
func (this *Client) OpenArchive(ctx context.Context, public PackageRef, manifest Manifest) (*ArchiveReader, error) {
	ref, err := validatePackageRef(public)
	if err != nil {
		return nil, err
	}
	if manifest.Version == "" || manifest.Version == "latest" {
		return nil, fmt.Errorf("%w: the manifest must name a concrete version", ErrInvalidConfig)
	}
	filename := manifest.Archive.Filename
	if filename == "" {
		filename = model.RemoteArchiveFilename
	}
	address := model.AppendRemotePath(url.URL(ref.RemoteAddress), ref.Name, manifest.Version, filename)
	body, err := this.storage.Download(ctx, address)
	if err != nil {
		return nil, err
	}
	body = packaging.NewResumingReader(ctx, this.storage, address, body, manifest.Archive.Size)
	hashed := packaging.NewHashReader(body, md5.New())
	reader, decompressor, err := packaging.OpenArchiveReader(hashed, manifest.Archive.CompressionAlgorithm, this.progress)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	return &ArchiveReader{
		body:         body,
		hashed:       hashed,
		reader:       reader,
		decompressor: decompressor,
		checksum:     manifest.Archive.MD5Checksum,
	}, nil
}

// ArchiveReader reads the files of a package archive in order, like a
// tar.Reader: Next advances to the next file, and Read reads its contents.
// Once Next has returned every file, the archive's checksum is verified, and
// Next returns a mismatch as an error instead of io.EOF.
type ArchiveReader struct {
	body         io.ReadCloser
	hashed       *packaging.HashReader
	reader       packaging.ArchiveReader
	decompressor io.Closer
	checksum     []byte
	done         error // set once the end of the archive has been reached
	decompressed bool  // the decompressor has been closed
	closed       bool
}

// Next advances to the next file in the archive. It returns io.EOF at the
// end of an archive whose checksum matches its manifest.
func (this *ArchiveReader) Next() (*tar.Header, error) {
	if this.done != nil {
		return nil, this.done
	}
	header, err := this.reader.Next()
	if errors.Is(err, io.EOF) {
		this.done = this.verify()
		return nil, this.done
	}
	return header, err
}

// Read reads from the current file in the archive.
func (this *ArchiveReader) Read(buffer []byte) (int, error) {
	return this.reader.Read(buffer)
}

// Close releases the download and the decompressor.
func (this *ArchiveReader) Close() error {
	if this.closed {
		return nil
	}
	this.closed = true
	return errors.Join(this.closeDecompressor(), this.body.Close())
}

func (this *ArchiveReader) closeDecompressor() error {
	if this.decompressed {
		return nil
	}
	this.decompressed = true
	return this.decompressor.Close()
}

// verify reads whatever the decompressor left unread so the checksum covers
// the whole archive, then compares it with the manifest's.
func (this *ArchiveReader) verify() error {
	if len(this.checksum) == 0 {
		return io.EOF
	}
	// the decompressor may read ahead on other goroutines; stop it first
	if err := this.closeDecompressor(); err != nil {
		return err
	}
	if _, err := io.Copy(io.Discard, this.hashed); err != nil {
		return fmt.Errorf("could not read the rest of the archive: %w", err)
	}
	if actual := this.hashed.Sum(nil); !bytes.Equal(actual, this.checksum) {
		return fmt.Errorf("archive checksum mismatch: actual [%x] != expected [%x]", actual, this.checksum)
	}
	return io.EOF
}

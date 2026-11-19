package satisfy

import (
	"archive/tar"
	"errors"
	"io"

	"github.com/smarty/assertions/should"
)

func (this *ClientFixture) uploadWith(algorithm, version string, tags ...string) Manifest {
	pkg := this.pkg(version, tags...)
	pkg.CompressionAlgorithm = algorithm
	manifest, err := this.client.Upload(this.ctx, UploadRequest{Package: pkg})
	this.So(err, should.BeNil)
	return manifest
}

func (this *ClientFixture) TestManifestOfVersion() {
	uploaded := this.uploadWith("zstd", "1.0.0")
	this.uploadWith("zstd", "2.0.0")

	manifest, err := this.client.Manifest(this.ctx, this.ref(), "1.0.0")
	this.So(err, should.BeNil)
	this.So(manifest, should.Resemble, uploaded)
}

func (this *ClientFixture) TestManifestOfTag() {
	this.uploadWith("zstd", "1.0.0", "stable")
	this.uploadWith("zstd", "2.0.0")

	manifest, err := this.client.Manifest(this.ctx, this.ref(), "stable")
	this.So(err, should.BeNil)
	this.So(manifest.Version, should.Equal, "1.0.0")
}

func (this *ClientFixture) TestVersionTakesPrecedenceOverTagOfTheSameName() {
	this.uploadWith("zstd", "1.0.0", "2.0.0")
	this.uploadWith("zstd", "2.0.0")

	manifest, err := this.client.Manifest(this.ctx, this.ref(), "2.0.0")
	this.So(err, should.BeNil)
	this.So(manifest.Version, should.Equal, "2.0.0")
}

func (this *ClientFixture) TestManifestOfLatestOmitsTags() {
	this.uploadWith("zstd", "1.0.0", "stable")
	this.uploadWith("zstd", "2.0.0")

	manifest, err := this.client.Manifest(this.ctx, this.ref(), "latest")
	this.So(err, should.BeNil)
	this.So(manifest.Version, should.Equal, "2.0.0")
	this.So(manifest.Tags, should.BeNil)
}

func (this *ClientFixture) TestUnknownVersionOrTagIsNotFound() {
	this.uploadWith("zstd", "1.0.0")

	_, err := this.client.Manifest(this.ctx, this.ref(), "nope")
	this.So(errors.Is(err, ErrNotFound), should.BeTrue)

	_, err = this.client.Manifest(this.ctx, PackageRef{RemoteAddress: this.remote, Name: "no/such/package"}, "1.0.0")
	this.So(errors.Is(err, ErrNotFound), should.BeTrue)
}

func (this *ClientFixture) TestManifestRequiresVersion() {
	_, err := this.client.Manifest(this.ctx, this.ref(), "")
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

func (this *ClientFixture) TestOpenArchiveStreamsEveryFile() {
	for _, algorithm := range []string{"zstd", "gzip", "zip"} {
		manifest := this.uploadWith(algorithm, "1.0.0-"+algorithm)

		files := this.readArchive(manifest)

		this.So(files, should.Resemble, map[string]string{
			"hello.txt":          "Hello, World!",
			"nested/goodbye.txt": "Goodbye!",
		})
	}
}

func (this *ClientFixture) TestChecksumMismatchIsReportedAtTheEnd() {
	manifest := this.uploadWith("zstd", "1.0.0")
	manifest.Archive.MD5Checksum = []byte("not the checksum")

	reader, err := this.client.OpenArchive(this.ctx, this.ref(), manifest)
	this.So(err, should.BeNil)
	defer func() { _ = reader.Close() }()
	for {
		_, err = reader.Next()
		if err != nil {
			break
		}
	}
	this.So(err, should.NotBeNil)
	this.So(errors.Is(err, io.EOF), should.BeFalse)
	this.So(err.Error(), should.ContainSubstring, "checksum mismatch")

	_, again := reader.Next()
	this.So(again, should.Equal, err)
}

func (this *ClientFixture) TestClosingBeforeTheEndIsFine() {
	manifest := this.uploadWith("zstd", "1.0.0")
	reader, err := this.client.OpenArchive(this.ctx, this.ref(), manifest)
	this.So(err, should.BeNil)
	_, err = reader.Next()
	this.So(err, should.BeNil)
	this.So(reader.Close(), should.BeNil)
	this.So(reader.Close(), should.BeNil)
}

func (this *ClientFixture) TestOpenArchiveRequiresConcreteVersion() {
	_, err := this.client.OpenArchive(this.ctx, this.ref(), Manifest{Version: "latest"})
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

func (this *ClientFixture) readArchive(manifest Manifest) map[string]string {
	reader, err := this.client.OpenArchive(this.ctx, this.ref(), manifest)
	this.So(err, should.BeNil)
	defer func() { this.So(reader.Close(), should.BeNil) }()

	files := make(map[string]string)
	for {
		var header *tar.Header
		header, err = reader.Next()
		if err == io.EOF {
			break
		}
		this.So(err, should.BeNil)
		if err != nil {
			break
		}
		raw, err := io.ReadAll(reader)
		this.So(err, should.BeNil)
		files[header.Name] = string(raw)
	}
	return files
}

package satisfy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/v2/internal/model"
)

func TestClientFixture(t *testing.T) {
	gunit.Run(new(ClientFixture), t)
}

type ClientFixture struct {
	*gunit.Fixture
	ctx      context.Context
	storage  *fakeStorage
	client   *Client
	source   string
	target   string
	remote   URL
	progress []Progress
	lock     sync.Mutex
}

func (this *ClientFixture) Setup() {
	this.ctx = context.Background()
	this.storage = newFakeStorage()
	this.client = New(withStorage(this.storage), WithMaxRetry(0), WithProgress(func(progress Progress) {
		this.lock.Lock()
		defer this.lock.Unlock()
		this.progress = append(this.progress, progress)
	}))
	this.source = this.tempDir()
	this.target = this.tempDir()
	this.remote = URL{Scheme: "gcs", Host: "bucket", Path: "/prefix"}
	this.writeSource("hello.txt", "Hello, World!")
	this.writeSource("nested/goodbye.txt", "Goodbye!")
}

func (this *ClientFixture) Teardown() {
	_ = os.RemoveAll(this.source)
	_ = os.RemoveAll(this.target)
}

func (this *ClientFixture) tempDir() string {
	path, err := os.MkdirTemp("", "satisfy-client-test-*")
	this.So(err, should.BeNil)
	return path
}

func (this *ClientFixture) writeSource(name, content string) {
	path := filepath.Join(this.source, name)
	this.So(os.MkdirAll(filepath.Dir(path), 0755), should.BeNil)
	this.So(os.WriteFile(path, []byte(content), 0644), should.BeNil)
}

func (this *ClientFixture) pkg(version string, tags ...string) PackageConfig {
	return PackageConfig{
		CompressionAlgorithm: "zstd",
		CompressionLevel:     1,
		SourceDirectory:      this.source,
		PackageName:          "team/package",
		PackageVersion:       version,
		RemoteAddress:        this.remote,
		Tags:                 tags,
	}
}

func (this *ClientFixture) ref() PackageRef {
	return PackageRef{RemoteAddress: this.remote, Name: "team/package"}
}

func (this *ClientFixture) dependency(version string) Dependency {
	return Dependency{
		PackageName:    "team/package",
		PackageVersion: version,
		RemoteAddress:  this.remote,
		LocalDirectory: this.target,
	}
}

func (this *ClientFixture) readTarget(name string) string {
	raw, err := os.ReadFile(filepath.Join(this.target, name))
	this.So(err, should.BeNil)
	return string(raw)
}

func (this *ClientFixture) TestUploadThenInstall() {
	manifest, err := this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})
	this.So(err, should.BeNil)
	this.So(manifest.Archive.Contents, should.HaveLength, 2)

	this.So(this.client.Install(this.ctx, this.dependency("1.0.0")), should.BeNil)

	this.So(this.readTarget("hello.txt"), should.Equal, "Hello, World!")
	this.So(this.readTarget("nested/goodbye.txt"), should.Equal, "Goodbye!")
	this.So(this.progress, should.NotBeEmpty)
}

func (this *ClientFixture) TestUploadExistingVersionFailsWithErrPackageExists() {
	_, err := this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})
	this.So(err, should.BeNil)

	_, err = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})
	this.So(errors.Is(err, ErrPackageExists), should.BeTrue)

	_, err = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0"), Overwrite: true})
	this.So(err, should.BeNil)
}

func (this *ClientFixture) TestExists() {
	exists, err := this.client.Exists(this.ctx, this.ref(), "1.0.0")
	this.So(err, should.BeNil)
	this.So(exists, should.BeFalse)

	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})

	exists, err = this.client.Exists(this.ctx, this.ref(), "1.0.0")
	this.So(err, should.BeNil)
	this.So(exists, should.BeTrue)
}

func (this *ClientFixture) TestExistsLatestReportsWhetherAnyVersionWasUploaded() {
	exists, err := this.client.Exists(this.ctx, this.ref(), "latest")
	this.So(err, should.BeNil)
	this.So(exists, should.BeFalse)

	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})

	exists, err = this.client.Exists(this.ctx, this.ref(), "latest")
	this.So(err, should.BeNil)
	this.So(exists, should.BeTrue)
}

func (this *ClientFixture) TestIncompletePackageRefIsRejected() {
	for _, ref := range []PackageRef{
		{RemoteAddress: this.remote},
		{RemoteAddress: this.remote, Name: "/"},
		{Name: "team/package"},
	} {
		_, err := this.client.Exists(this.ctx, ref, "1.0.0")
		this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
		_, err = this.client.Latest(this.ctx, ref)
		this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
	}
	_, err := this.client.Exists(this.ctx, this.ref(), "")
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

func (this *ClientFixture) TestUploadCanExcludeInstalledManifests() {
	this.writeSource("manifest_team___package.json", "{}")
	this.writeSource("nested/manifest_other.json", "{}")
	this.writeSource("nested/manifest.json", "kept: not an installed manifest")

	manifest, err := this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0"), ExcludeInstalledManifests: true})
	this.So(err, should.BeNil)
	this.So(archivePaths(manifest), should.Resemble, []string{"hello.txt", "nested/goodbye.txt", "nested/manifest.json"})

	manifest, err = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("2.0.0")})
	this.So(err, should.BeNil)
	this.So(archivePaths(manifest), should.HaveLength, 5)
}

func archivePaths(manifest Manifest) (paths []string) {
	for _, item := range manifest.Archive.Contents {
		paths = append(paths, item.Path)
	}
	return paths
}

func (this *ClientFixture) TestUploadRequiresExactlyOneSource() {
	pkg := this.pkg("1.0.0")
	pkg.SourceFile = filepath.Join(this.source, "hello.txt")
	_, err := this.client.Upload(this.ctx, UploadRequest{Package: pkg})
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
	this.So(err.Error(), should.ContainSubstring, "only one of")
	this.So(this.storage.objects, should.BeEmpty)
}

func (this *ClientFixture) TestInvalidUploadIsRejectedBeforeAnyIO() {
	pkg := this.pkg("1.0.0")
	pkg.RemoteAddress = URL{}
	_, err := this.client.Upload(this.ctx, UploadRequest{Package: pkg})
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
	this.So(this.storage.objects, should.BeEmpty)
}

func (this *ClientFixture) TestLatestAndTags() {
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0", "stable")})
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("2.0.0", "beta")})

	latest, err := this.client.Latest(this.ctx, this.ref())
	this.So(err, should.BeNil)
	this.So(latest, should.Equal, "2.0.0")

	tags, err := this.client.Tags(this.ctx, this.ref())
	this.So(err, should.BeNil)
	this.So(tags, should.Resemble, []Tag{{Name: "beta", Version: "2.0.0"}, {Name: "stable", Version: "1.0.0"}})
}

func (this *ClientFixture) TestInstallByTag() {
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0", "stable")})

	this.So(this.client.Install(this.ctx, this.dependency("stable")), should.BeNil)
	this.So(this.readTarget("hello.txt"), should.Equal, "Hello, World!")
}

func (this *ClientFixture) TestInstalledManifestNamesTheResolvedVersion() {
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0", "stable")})
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("2.0.0")})

	_, err := InstalledManifest(this.target, "team/package")
	this.So(errors.Is(err, fs.ErrNotExist), should.BeTrue)

	this.So(this.client.Install(this.ctx, this.dependency("stable")), should.BeNil)
	manifest, err := InstalledManifest(this.target, "team/package")
	this.So(err, should.BeNil)
	this.So(manifest.Name, should.Equal, "team/package")
	this.So(manifest.Version, should.Equal, "1.0.0")

	this.So(this.client.Install(this.ctx, this.dependency("latest")), should.BeNil)
	manifest, err = InstalledManifest(this.target, "team/package")
	this.So(err, should.BeNil)
	this.So(manifest.Version, should.Equal, "2.0.0")
}

func (this *ClientFixture) TestInstalledManifestRequiresDirectoryAndName() {
	_, err := InstalledManifest("", "team/package")
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
	_, err = InstalledManifest(this.target, "")
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

func (this *ClientFixture) TestModifyTags() {
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0", "stable")})
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("2.0.0")})

	err := this.client.ModifyTags(this.ctx, TagModification{
		PackageName:   "team/package",
		RemoteAddress: this.remote,
		Add:           []Tag{{Name: "stable", Version: "2.0.0"}},
	})
	this.So(err, should.BeNil)

	tags, _ := this.client.Tags(this.ctx, this.ref())
	this.So(tags, should.Resemble, []Tag{{Name: "stable", Version: "2.0.0"}})
}

func (this *ClientFixture) TestModifyTagsRejectsDuplicateAndConflictingTags() {
	for _, modification := range []TagModification{
		{Add: []Tag{{Name: "a", Version: "1"}, {Name: "a", Version: "2"}}},
		{Delete: []Tag{{Name: "a"}, {Name: "a"}}},
		{Add: []Tag{{Name: "a", Version: "1"}}, Delete: []Tag{{Name: "a"}}},
	} {
		modification.PackageName, modification.RemoteAddress = "team/package", this.remote
		this.So(errors.Is(this.client.ModifyTags(this.ctx, modification), ErrInvalidConfig), should.BeTrue)
	}
}

func (this *ClientFixture) TestModifyTagsRejectsUnknownVersion() {
	_, _ = this.client.Upload(this.ctx, UploadRequest{Package: this.pkg("1.0.0")})

	err := this.client.ModifyTags(this.ctx, TagModification{
		PackageName:   "team/package",
		RemoteAddress: this.remote,
		Add:           []Tag{{Name: "stable", Version: "9.9.9"}},
	})
	this.So(errors.Is(err, ErrNotFound), should.BeTrue)
}

func (this *ClientFixture) TestUnknownPackageIsNotFound() {
	_, err := this.client.Latest(this.ctx, PackageRef{RemoteAddress: this.remote, Name: "no/such/package"})
	this.So(errors.Is(err, ErrNotFound), should.BeTrue)
	code, found := StatusCode(err)
	this.So(code, should.Equal, http.StatusNotFound)
	this.So(found, should.BeTrue)

	err = this.client.Install(this.ctx, Dependency{
		PackageName:    "no/such/package",
		PackageVersion: "1.0.0",
		RemoteAddress:  this.remote,
		LocalDirectory: this.target,
	})
	this.So(errors.Is(err, ErrNotFound), should.BeTrue)
}

func (this *ClientFixture) TestInvalidDependencyIsRejected() {
	err := this.client.Install(this.ctx, Dependency{PackageName: "p"})
	this.So(errors.Is(err, ErrInvalidConfig), should.BeTrue)
}

///////////////////////////////////////////////////////////////////////////////

// fakeStorage keeps uploaded objects in memory, keyed by address.
type fakeStorage struct {
	lock    sync.Mutex
	objects map[string][]byte
}

func newFakeStorage() *fakeStorage {
	return &fakeStorage{objects: make(map[string][]byte)}
}

func (this *fakeStorage) Upload(_ context.Context, request model.UploadRequest) error {
	raw, err := io.ReadAll(request.Body)
	if err != nil {
		return err
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	this.objects[request.RemoteAddress.String()] = raw
	return nil
}

func (this *fakeStorage) object(address url.URL) ([]byte, error) {
	this.lock.Lock()
	defer this.lock.Unlock()
	raw, found := this.objects[address.String()]
	if !found {
		return nil, model.NewStatusCodeError(http.StatusNotFound, []int{http.StatusOK}, address)
	}
	return raw, nil
}

func (this *fakeStorage) Download(_ context.Context, address url.URL) (io.ReadCloser, error) {
	raw, err := this.object(address)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(raw)), nil
}

func (this *fakeStorage) Seek(_ context.Context, address url.URL, start, end int64) (io.ReadCloser, error) {
	raw, err := this.object(address)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(raw[start : end+1])), nil
}

func (this *fakeStorage) Size(_ context.Context, address url.URL) (int64, error) {
	raw, err := this.object(address)
	return int64(len(raw)), err
}

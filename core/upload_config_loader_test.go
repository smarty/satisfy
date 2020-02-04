package core

import (
	"bytes"
	"encoding/json"
	"testing"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gunit"
)

func TestUploadConfigLoaderFixture(t *testing.T) {
	gunit.Run(new(UploadConfigLoaderFixture), t)
}

type UploadConfigLoaderFixture struct {
	*gunit.Fixture

	loader      *UploadConfigLoader
	storage     *inMemoryFileSystem
	environment FakeEnvironment
	stdin       *bytes.Buffer
}

func (this *UploadConfigLoaderFixture) Setup() {
	this.stdin = new(bytes.Buffer)
	this.storage = newInMemoryFileSystem()
	this.environment = make(FakeEnvironment)
	this.loader = NewUploadConfigLoader(this.storage, this.environment, this.stdin)
}

func (this *UploadConfigLoaderFixture) TestInvalidCLI() {
	args := []string{"-max-retry", "Hello, world!"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestValidJSONFromSpecifiedFile() {
	packageConfig := contracts.PackageConfig{
		CompressionAlgorithm: "algorithm",
		CompressionLevel:     42,
		SourceDirectory:      "source",
		PackageName:          "package",
		PackageVersion:       "version",
		RemoteAddressPrefix:  &contracts.URL{Scheme: "gcs", Host: "host", Path: "/path"},
	}
	raw, _ := json.Marshal(packageConfig)
	this.storage.WriteFile("config.json", raw)
	args := []string{
		"-max-retry", "10",
		"-json", "config.json",
		"-overwrite",
	}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.BeNil)
	this.So(config, should.Resemble, contracts.UploadConfig{
		MaxRetry:      10,
		JSONPath:      "config.json",
		Overwrite:     true,
		PackageConfig: packageConfig,
	})
}

//////////////////////////////////////////////////////////

type FakeEnvironment map[string]string

func (this FakeEnvironment) LookupEnv(key string) (value string, set bool) {
	value, set = this[key]
	return value, set
}

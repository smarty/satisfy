package core

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
	"github.com/smarty/satisfy/contracts"
)

func TestTagsConfigLoaderFixture(t *testing.T) {
	gunit.Run(new(TagsConfigLoaderFixture), t)
}

type TagsConfigLoaderFixture struct {
	*gunit.Fixture

	loader       *TagsConfigLoader
	storage      *inMemoryFileSystem
	environment  FakeEnvironment
	stdin        *bytes.Buffer
	modification contracts.TagModificationConfig
}

func (this *TagsConfigLoaderFixture) Setup() {
	this.stdin = new(bytes.Buffer)
	this.storage = newInMemoryFileSystem()
	this.environment = make(FakeEnvironment)
	this.loader = NewTagsConfigLoader(this.storage, this.environment, this.stdin, io.Discard)
	credentialsPath := "/path/to/google-credentials.json"
	this.environment["GOOGLE_APPLICATION_CREDENTIALS"] = credentialsPath
	this.storage.WriteFile(strings.TrimSpace(credentialsPath), []byte(googleCredentialsJSON))
	this.modification = contracts.TagModificationConfig{
		PackageName:   "cat-sound-data",
		RemoteAddress: &contracts.URL{Scheme: "gcs", Host: "host", Path: "/path"},
		Add: []contracts.Tag{
			{Name: "stable", Version: "2026.02.A"},
			{Name: "marks-favorite", Version: "2026.01.B"},
		},
		Delete: []contracts.Tag{
			{Name: "active-build-test"},
		},
	}
}

func (this *TagsConfigLoaderFixture) prepareConfigFile() {
	raw, _ := json.Marshal(this.modification)
	this.storage.WriteFile("tags.json", raw)
}

func (this *TagsConfigLoaderFixture) loadConfig() (contracts.TagsConfig, error) {
	this.prepareConfigFile()
	return this.loader.LoadConfig([]string{"-json", "tags.json"})
}

func (this *TagsConfigLoaderFixture) TestValidConfigFromSpecifiedFile() {
	this.prepareConfigFile()

	config, err := this.loader.LoadConfig([]string{"-max-retry", "10", "-json", "tags.json"})

	this.So(err, should.BeNil)
	this.So(config.MaxRetry, should.Equal, 10)
	this.So(config.JSONPath, should.Equal, "tags.json")
	this.So(config.GoogleCredentials, should.Resemble, parsedGoogleCredentials)
	this.So(config.Modification, should.Resemble, this.modification)
}

func (this *TagsConfigLoaderFixture) TestValidConfigFromStdIn() {
	raw, _ := json.Marshal(this.modification)
	this.stdin.Write(raw)

	config, err := this.loader.LoadConfig([]string{"-json", "_STDIN_"})

	this.So(err, should.BeNil)
	this.So(config.Modification, should.Resemble, this.modification)
}

func (this *TagsConfigLoaderFixture) TestInvalidCLI() {
	config, err := this.loader.LoadConfig([]string{"-max-retry", "Hello, world!"})

	this.So(err, should.NotBeNil)
	this.So(config, should.BeZeroValue)
}

func (this *TagsConfigLoaderFixture) TestMalformedJSON() {
	this.storage.WriteFile("tags.json", []byte("Invalid JSON"))

	config, err := this.loader.LoadConfig([]string{"-json", "tags.json"})

	this.So(err, should.NotBeNil)
	this.So(config.Modification, should.BeZeroValue)
}

func (this *TagsConfigLoaderFixture) TestNegativeMaxRetry() {
	this.prepareConfigFile()

	_, err := this.loader.LoadConfig([]string{"-max-retry", "-10", "-json", "tags.json"})

	this.So(err, should.Resemble, maxRetryErr)
}

func (this *TagsConfigLoaderFixture) TestBlankPackageName() {
	this.modification.PackageName = ""

	_, err := this.loadConfig()

	this.So(err, should.Resemble, blankPackageNameErr)
}

func (this *TagsConfigLoaderFixture) TestNilRemoteAddress() {
	this.modification.RemoteAddress = nil

	_, err := this.loadConfig()

	this.So(err, should.Resemble, nilRemoteAddressPrefixErr)
}

func (this *TagsConfigLoaderFixture) TestNoModifications() {
	this.modification.Add = nil
	this.modification.Delete = nil

	_, err := this.loadConfig()

	this.So(err, should.Resemble, noTagModificationsErr)
}

func (this *TagsConfigLoaderFixture) TestBlankAddedTagName() {
	this.modification.Add = append(this.modification.Add, contracts.Tag{Name: "", Version: "2026.01.A"})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, blankTagNameErr)
}

func (this *TagsConfigLoaderFixture) TestReservedAddedTagName() {
	this.modification.Add = append(this.modification.Add, contracts.Tag{Name: "latest", Version: "2026.01.A"})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, reservedTagNameErr)
}

func (this *TagsConfigLoaderFixture) TestBlankAddedTagVersion() {
	this.modification.Add = append(this.modification.Add, contracts.Tag{Name: "stale", Version: ""})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, blankTagVersionErr)
}

func (this *TagsConfigLoaderFixture) TestDuplicateAddedTagName() {
	this.modification.Add = append(this.modification.Add, contracts.Tag{Name: "stable", Version: "2026.01.A"})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, duplicateTagNameErr)
}

func (this *TagsConfigLoaderFixture) TestBlankDeletedTagName() {
	this.modification.Delete = append(this.modification.Delete, contracts.Tag{Name: ""})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, blankTagNameErr)
}

func (this *TagsConfigLoaderFixture) TestDuplicateDeletedTagName() {
	this.modification.Delete = append(this.modification.Delete, contracts.Tag{Name: "active-build-test"})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, duplicateTagNameErr)
}

func (this *TagsConfigLoaderFixture) TestTagNameInBothAddAndDelete() {
	this.modification.Delete = append(this.modification.Delete, contracts.Tag{Name: "stable"})

	_, err := this.loadConfig()

	this.So(err, should.Resemble, conflictingTagNameErr)
}

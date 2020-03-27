package core

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/gcs"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/satisfy/contracts"
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
	pkgConfig   *FakePackageConfig
}

func (this *UploadConfigLoaderFixture) Setup() {
	this.stdin = new(bytes.Buffer)
	this.storage = newInMemoryFileSystem()
	this.environment = make(FakeEnvironment)
	this.loader = NewUploadConfigLoader(this.storage, this.environment, this.stdin)
	credentialsPath := "  /path/to/google-credentials.json  "
	this.environment["GOOGLE_APPLICATION_CREDENTIALS"] = credentialsPath
	this.storage.WriteFile(strings.TrimSpace(credentialsPath), []byte(googleCredentialsJSON))
	this.pkgConfig = NewFakePackageConfig()
}

func (this *UploadConfigLoaderFixture) TestInvalidCLI() {
	args := []string{"-max-retry", "Hello, world!"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestValidJSONFromSpecifiedFile() {
	packageConfig := this.prepareValidJSONConfigFile()
	args := []string{
		"-max-retry", "10",
		"-json", "config.json",
		"-overwrite",
	}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.BeNil)
	this.So(config, should.Resemble, contracts.UploadConfig{
		GoogleCredentials: parsedGoogleCredentials,
		MaxRetry:          10,
		JSONPath:          "config.json",
		Overwrite:         true,
		PackageConfig:     packageConfig,
	})
}

func (this *UploadConfigLoaderFixture) TestInValidJSONFromSpecifiedFile() {
	this.storage.WriteFile("config.json", []byte("Invalid JSON"))
	args := []string{"-json", "config.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.PackageConfig, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestValidJSONFromStdIn() {
	packageConfig := contracts.PackageConfig{
		CompressionAlgorithm: "algorithm",
		CompressionLevel:     42,
		SourceDirectory:      "source",
		PackageName:          "package",
		PackageVersion:       "version",
		RemoteAddressPrefix:  &contracts.URL{Scheme: "gcs", Host: "host", Path: "/path"},
	}
	raw, _ := json.Marshal(packageConfig)
	this.stdin.Write(raw)
	args := []string{"-json", "_STDIN_"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.BeNil)
	this.So(config.PackageConfig, should.Resemble, packageConfig)
}

func (this *UploadConfigLoaderFixture) TestSpecifiedJSONFileNotFound() {
	args := []string{"-json", "not-found.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.PackageConfig, should.BeZeroValue)

}

func (this *UploadConfigLoaderFixture) TestGoogleCredentialsEnvironmentVariableMissing() {
	delete(this.environment, "GOOGLE_APPLICATION_CREDENTIALS")
	this.storage.WriteFile("/path/to/google-credentials.json", []byte(googleCredentialsJSON))
	_ = this.prepareValidJSONConfigFile()
	args := []string{"-json", "config.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.GoogleCredentials, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestGoogleCredentialsEnvironmentVariableBlank() {
	this.environment["GOOGLE_APPLICATION_CREDENTIALS"] = "  "
	_ = this.prepareValidJSONConfigFile()
	args := []string{"-json", "config.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.GoogleCredentials, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestGoogleCredentialsFileIsMissing() {
	this.storage.Delete("/path/to/google-credentials.json")
	_ = this.prepareValidJSONConfigFile()
	args := []string{"-json", "config.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.GoogleCredentials, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestGoogleCredentialsFileIsMalformed() {
	this.storage.WriteFile("/path/to/google-credentials.json", []byte(badGoogleCredentialsJSON))
	_ = this.prepareValidJSONConfigFile()
	args := []string{"-json", "config.json"}

	config, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.NotBeNil)
	this.So(config.GoogleCredentials, should.BeZeroValue)
}

func (this *UploadConfigLoaderFixture) TestValidateJSONNegativeMaxRetries() {
	_ = this.prepareValidJSONConfigFile()
	args := []string{
		"-max-retry", "-10",
		"-json", "config.json",
	}

	_, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.Resemble, maxRetryErr)
}

func (this *UploadConfigLoaderFixture) TestValidateJSONPathIsNotBlank() {
	_ = this.prepareValidJSONConfigFile()
	_, err := this.loader.LoadConfig("upload", []string{"-json", ""})

	this.So(err, should.Resemble, blankJSONPathErr)
}

func (this *UploadConfigLoaderFixture) TestValidateCompressionAlgorithmIsNotBlank() {
	this.pkgConfig.CompressionAlgorithm = ""
	raw, _ := json.Marshal(this.pkgConfig.configure())
	this.storage.WriteFile("config.json", raw)
	_, err := this.loader.LoadConfig("upload", []string{"-json", "config.json"})

	this.So(err, should.Resemble, blankCompressionAlgorithmErr)
}

func (this *UploadConfigLoaderFixture) TestValidateSourceDirectoryIsNotBlank() {
	this.pkgConfig.SourceDirectory = ""
	raw, _ := json.Marshal(this.pkgConfig.configure())
	this.storage.WriteFile("config.json", raw)
	args := []string{"-json", "config.json"}

	_, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.Resemble, blankSourceDirectoryErr)
}

func (this *UploadConfigLoaderFixture) TestValidatePackageNameIsNotBlank() {
	this.pkgConfig.PackageName = ""
	raw, _ := json.Marshal(this.pkgConfig.configure())
	this.storage.WriteFile("config.json", raw)
	args := []string{"-json", "config.json"}

	_, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.Resemble, blankPackageNameErr)
}

func (this *UploadConfigLoaderFixture) TestValidatePackageVersionIsNotBlank() {
	this.pkgConfig.PackageVersion = ""
	raw, _ := json.Marshal(this.pkgConfig.configure())
	this.storage.WriteFile("config.json", raw)
	args := []string{"-json", "config.json"}

	_, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.Resemble, blankPackageVersionErr)
}

func (this *UploadConfigLoaderFixture) TestValidateRemoteAddressPrefixIsNotNil() {
	this.pkgConfig.RemoteAddressPrefix = nil
	raw, _ := json.Marshal(this.pkgConfig.configure())
	this.storage.WriteFile("config.json", raw)
	args := []string{"-json", "config.json"}

	_, err := this.loader.LoadConfig("upload", args)

	this.So(err, should.Resemble, nilRemoteAddressPrefixErr)
}

func (this *UploadConfigLoaderFixture) prepareValidJSONConfigFile() contracts.PackageConfig {
	packageConfig := this.pkgConfig.configure()
	raw, _ := json.Marshal(packageConfig)
	this.storage.WriteFile("config.json", raw)
	return packageConfig
}

//////////////////////////////////////////////////////////

type FakePackageConfig struct {
	CompressionAlgorithm string
	SourceDirectory      string
	PackageName          string
	PackageVersion       string
	RemoteAddressPrefix  *contracts.URL
}

func NewFakePackageConfig() *FakePackageConfig {
	return &FakePackageConfig{
		CompressionAlgorithm: "algorithm",
		SourceDirectory:      "source",
		PackageName:          "package",
		PackageVersion:       "version",
		RemoteAddressPrefix:  &contracts.URL{Scheme: "gcs", Host: "host", Path: "/path"},
	}
}

func (this *FakePackageConfig) configure() contracts.PackageConfig {
	return contracts.PackageConfig{
		CompressionAlgorithm: this.CompressionAlgorithm,
		CompressionLevel:     42,
		SourceDirectory:      this.SourceDirectory,
		PackageName:          this.PackageName,
		PackageVersion:       this.PackageVersion,
		RemoteAddressPrefix:  this.RemoteAddressPrefix,
	}
}

type FakeEnvironment map[string]string

func (this FakeEnvironment) LookupEnv(key string) (value string, set bool) {
	value, set = this[key]
	return value, set
}

const (
	// This is a deleted key which does not provide access to any google resources.
	googleCredentialsJSON = `{
  "type": "service_account",
  "project_id": "satisfy-266121",
  "private_key_id": "16ad5ad2d5f070050dcdd3a4addfaea0c4ecffbf",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDNyKkPwT7JUtOd\nfncvMCCTIAgC1B1pshDLjjVrdmveC12lrHfDTHFm89O8JJ49CgBXtayiUFEWFhc6\nJVJVhfDL/SWGVEfz5g8qeB0dLgYnHQs48dlxFRJK7gPp9b/gCoACbs2dlRZHVVTk\nsvseAjbVHlKyDcxVoQePDm/Xg4lQi55vJkJvJZKKPzDTneQG5DChxwMpCFxj8bCJ\nVxQ1zrp51ZeLIZTS1Ij4HF7Mt0EJC0OYX3N/F5RPhj9E7MmZU0BJH0KAfeApjgYa\njieKiU+hqBkqUe866n36NlKbop3jF2vps7kCIZe2ZSr1RzY6p/wgHav9L64fnVCa\n9/bFZQXFAgMBAAECggEALuQB7gadPXfDo5gdJWIEkjHS0X4vA5YhMJkDgCy4UJzr\nZmSB170z+/8kaLM5YXRFdrb9kvDVQUCgY0380GMYZwsUgWL0EFYEb6t2Ct+hZElA\ndOXbI+Lmy68nsiie47jQyX0hGj7OGEwP75r/EKv1faOOuWbegEaUt9rUzll5MSJu\nYLNRJkffAFeI+nodD32laeQ9bdqgJtBtMcKNJV4pH4xl7fs6tyaOORwVn9XKJgNq\nV5SldKLScIThpPAl7q4adAX41OKqBOvUUXs2FdbQ1faCmX9FH+omUGm0vUbyCkrM\nNkZy3oYdfX1EOev1dmJfEck9S7VuitOA9Mfl7RwKIQKBgQD60sDUiAuMunrVik3n\n4g9JkRAvxkAeS/Iy6JYgqok1LxwUB1yGjqES6CW5ts0KuIh2SDHjNDBgI4RZkNQp\n/qAje36+Y5n5VMw9zbt7tUEK0ZM+34Fw27UDaykNcZtkLnAJxpCJmdQT88SsiB22\nXdnyoDBJ4b05wjjD403pqwNBlQKBgQDSB+/4Z71nlMUYx/yRbdJTAxMVAaI+6Lea\nUlJh/K6jwDYojsxfTUX+nAOGCHWZdmUn/3qlH+nD7Kjl+OkQN6hEek5uT1DCXf7D\nauzJdG2NFN55n+d/PATVH1dgFBUbBCfeGDkvcTM55jgiWZT+mdcA5mRrpeMpzGHW\nF3xWMwIHcQKBgQDBIczHGaZDC0gP6znHpkJ9NAzRrIasjXAGER+gMZAK+qZVKcHt\n/h867rQ1xvMlISg6Y6a+Ou5Q6Kg9Sw6C84QdLjdOpGToHopRwHtvawaVLQCDNhh2\nbUZ5RmdK6cJsJnGwpUugGGm7n0U+UGUIikWK1Bu6l+5bbhjFhN32Ye7U7QKBgG6H\ngNDv/ywYjZTaAd+itNG8x3kBkBmdLKpI8lPgvyMrzxSO+ZyZtOElx3Ds2L53IQro\nlul5HvNdgxDrafN/5syKtOW2VeDDyIOcrJnj7JcXSXEmJpS9yClEQh4s02KRUE2/\n37BI2VV6A0aIcDGAUjaGCIjiFubzSPV7DJLsav/xAoGAXiKkhhJNy086WEmQYuHL\nrnRpcSXgddRrlBcfpDwL4QpL4SNkWszVwlObUOGiUvYuIwaPabAJIwXW86w4RiGB\nFaeCFT5CuGvPCxdwai4WCI6oBDz3/4yvnn78eDaWTQQKrNcaQ0H9MEHhQcUHdgwJ\nEElQEby32BYq1erAS0GUa7Q=\n-----END PRIVATE KEY-----\n",
  "client_email": "testing@satisfy-266121.iam.gserviceaccount.com",
  "client_id": "116935078055174507184",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/testing%40satisfy-266121.iam.gserviceaccount.com"
}
`
	badGoogleCredentialsJSON = `{
  "private_key": "Invalid Private Key",
}
`
)

var parsedGoogleCredentials, _ = gcs.ParseCredentialsFromJSON([]byte(googleCredentialsJSON))

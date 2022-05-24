package core

import (
	"encoding/base64"
	"errors"
	"strings"

	"github.com/smartystreets/gcs"
	"github.com/smartystreets/satisfy/contracts"
)

type CredentialParser struct {
	storage     contracts.FileReader
	environment contracts.Environment
}

func NewGoogleCredentialParser(storage contracts.FileReader, environment contracts.Environment) CredentialParser {
	return CredentialParser{storage: storage, environment: environment}
}

func (this CredentialParser) Parse() (gcs.Credentials, error) {
	if value, found := this.environment.LookupEnv("GOOGLE_OAUTH_ACCESS_TOKEN"); found {
		if !strings.HasPrefix(value, "Bearer ") {
			value = "Bearer " + strings.TrimSuffix(value, ".")
		}

		return gcs.Credentials{BearerToken: value}, nil
	}

	if inlineCredential, found := this.environment.LookupEnv("GOOGLE_CREDENTIALS"); found {
		return ParseCredential(base64.StdEncoding.DecodeString(inlineCredential))
	}

	googleCredentialsPath, found := this.environment.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	googleCredentialsPath = strings.TrimSpace(googleCredentialsPath)
	if !found || googleCredentialsPath == "" {
		return gcs.Credentials{}, errors.New("the GOOGLE_APPLICATION_CREDENTIALS is required")
	}

	return ParseCredential(this.storage.ReadFile(googleCredentialsPath))
}

func ParseCredential(value []byte, _ error) (gcs.Credentials, error) {
	return gcs.ParseCredentialsFromJSON(value)
}

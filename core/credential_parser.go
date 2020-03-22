package core

import (
	"encoding/base64"
	"errors"
	"strings"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/gcs"
)

type CredentialParser struct {
	storage     contracts.FileReader
	environment contracts.Environment
}

func NewGoogleCredentialParser(storage contracts.FileReader, environment contracts.Environment) CredentialParser {
	return CredentialParser{storage: storage, environment: environment}
}

func (this CredentialParser) Parse() (gcs.Credentials, error) {
	if inlineCredential, found := this.environment.LookupEnv("GOOGLE_CREDENTIALS"); found {
		return parseCredential(base64.StdEncoding.DecodeString(inlineCredential))
	}

	googleCredentialsPath, found := this.environment.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	googleCredentialsPath = strings.TrimSpace(googleCredentialsPath)
	if !found || googleCredentialsPath == "" {
		return gcs.Credentials{}, errors.New("the GOOGLE_APPLICATION_CREDENTIALS is required")
	}

	return parseCredential(this.storage.ReadFile(googleCredentialsPath))
}

func parseCredential(value []byte, err error) (gcs.Credentials, error) {
	return gcs.ParseCredentialsFromJSON(value)
}

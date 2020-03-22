package core

import (
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
	googleCredentialsPath, found := this.environment.LookupEnv("GOOGLE_APPLICATION_CREDENTIALS")
	googleCredentialsPath = strings.TrimSpace(googleCredentialsPath)
	if !found || googleCredentialsPath == "" {
		return gcs.Credentials{}, errors.New("the GOOGLE_APPLICATION_CREDENTIALS is required")
	}
	data, err := this.storage.ReadFile(googleCredentialsPath)
	if err != nil {
		return gcs.Credentials{}, err
	}
	credentials, err := gcs.ParseCredentialsFromJSON(data)
	if err != nil {
		return gcs.Credentials{}, err
	}
	return credentials, nil
}

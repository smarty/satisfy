package shell

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type GoogleCloudStorageClient struct {
	client         *http.Client
	credentials    gcs.Credentials
	expectedStatus int
}

func NewGoogleCloudStorageClient(client *http.Client, credentials gcs.Credentials, expectedStatus int) *GoogleCloudStorageClient {
	return &GoogleCloudStorageClient{client: client, credentials: credentials, expectedStatus: expectedStatus}
}

func (this *GoogleCloudStorageClient) Upload(request contracts.UploadRequest) error {
	gcsRequest, err := gcs.NewRequest("PUT",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(request.RemoteAddress.Host),
		gcs.WithResource(request.RemoteAddress.Path),
		gcs.PutWithContent(request.Body),
		gcs.PutWithContentLength(request.Size),
		gcs.PutWithContentMD5(request.Checksum),
		gcs.PutWithContentType(request.ContentType),
	)
	if err != nil {
		return err
	}
	response, err := this.client.Do(gcsRequest)
	if err != nil {
		return fmt.Errorf("http error: %s (%w)", err, contracts.RetryErr)
	}
	if response.StatusCode != this.expectedStatus {
		return fmt.Errorf("unexpected status code: %s", response.Status)
	}
	return nil
}

func (this *GoogleCloudStorageClient) Download(request url.URL) (io.ReadCloser, error) {
	gcsRequest, err := gcs.NewRequest("GET",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(request.Host),
		gcs.WithResource(request.Path),
	)
	if err != nil {
		return nil, err
	}
	response, err := this.client.Do(gcsRequest)
	if err != nil {
		return nil, fmt.Errorf("http error: %s (%w)", err, contracts.RetryErr)
	}
	if response.StatusCode == http.StatusOK {
		return nil, fmt.Errorf("file exists")
	}
	if response.StatusCode != this.expectedStatus {
		return nil, fmt.Errorf("unexpected status code: %s", response.Status)
	}
	return response.Body, nil
}

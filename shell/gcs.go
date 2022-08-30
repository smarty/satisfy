package shell

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smartystreets/gcs"
	"github.com/smartystreets/satisfy/contracts"
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
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != this.expectedStatus {
		if this.isSafeRetryStatus(response.StatusCode) {
			return fmt.Errorf("http error: %d (%w)", response.StatusCode, contracts.RetryErr)
		}
		return contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request.RemoteAddress)
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
	if response.StatusCode != this.expectedStatus {
		return nil, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request)
	}
	return response.Body, nil
}

func (this *GoogleCloudStorageClient) isSafeRetryStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

package shell

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/contracts"
)

type GoogleCloudStorageClient struct {
	client         *http.Client
	credentials    gcs.Credentials
	expectedStatus []int
}

func NewGoogleCloudStorageClient(client *http.Client, credentials gcs.Credentials, expectedStatus []int) *GoogleCloudStorageClient {
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

	_, _ = io.Copy(io.Discard, response.Body)

	if this.isExpectedStatus(response.StatusCode) == false {
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
	if this.isExpectedStatus(response.StatusCode) == false {
		return nil, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request)
	}
	return response.Body, nil
}

func (this *GoogleCloudStorageClient) Seek(request url.URL, start, end int64) (io.ReadCloser, error) {
	gcsRequest, err := gcs.NewRequest("GET",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(request.Host),
		gcs.WithResource(request.Path),
	)
	if err != nil {
		return nil, err
	}
	gcsRequest.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	response, err := this.client.Do(gcsRequest)
	if err != nil {
		return nil, fmt.Errorf("http error: %s (%w)", err, contracts.RetryErr)
	}
	if this.isExpectedStatus(response.StatusCode) == false {
		return nil, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request)
	}
	return response.Body, nil
}

// Size uses an HTTP HEAD to find out how many bytes are available in total.
func (this *GoogleCloudStorageClient) Size(request url.URL) (int64, error) {
	gcsRequest, err := gcs.NewRequest("HEAD",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(request.Host),
		gcs.WithResource(request.Path),
	)
	if err != nil {
		return 0, err
	}
	response, err := this.client.Do(gcsRequest)
	if err != nil {
		return 0, fmt.Errorf("http error: %s (%w)", err, contracts.RetryErr)
	}
	if this.isExpectedStatus(response.StatusCode) == false {
		return 0, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request)
	}
	return response.ContentLength, nil
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

func (this *GoogleCloudStorageClient) isExpectedStatus(statusCode int) bool {
	for _, expectedStatus := range this.expectedStatus {
		if expectedStatus == statusCode {
			return true
		}
	}
	return false
}

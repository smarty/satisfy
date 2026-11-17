package shell

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"

	"github.com/smarty/gcs"
	"github.com/smarty/satisfy/v2/contracts"
)

type GoogleCloudStorageClient struct {
	client         *http.Client
	credentials    CredentialsSource
	expectedStatus []int
}

func NewGoogleCloudStorageClient(client *http.Client, credentials CredentialsSource, expectedStatus []int) *GoogleCloudStorageClient {
	return &GoogleCloudStorageClient{client: client, credentials: credentials, expectedStatus: expectedStatus}
}

func (this *GoogleCloudStorageClient) newRequest(ctx context.Context, method string, address url.URL, options ...gcs.Option) (*http.Request, error) {
	credentials, err := this.credentials.Credentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not obtain Google credentials: %w", err)
	}
	options = append([]gcs.Option{
		gcs.WithContext(ctx),
		gcs.WithCredentials(credentials),
		gcs.WithBucket(address.Host),
		gcs.WithResource(address.Path),
	}, options...)
	return gcs.NewRequest(method, options...)
}

func (this *GoogleCloudStorageClient) do(request *http.Request) (*http.Response, error) {
	response, err := this.client.Do(request)
	if err != nil {
		if ctxErr := request.Context().Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, fmt.Errorf("http error: %w (%w)", err, contracts.ErrRetryable)
	}
	return response, nil
}

func (this *GoogleCloudStorageClient) Upload(ctx context.Context, request contracts.UploadRequest) error {
	gcsRequest, err := this.newRequest(ctx, "PUT", request.RemoteAddress,
		gcs.PutWithContent(withoutClose{Reader: request.Body}),
		gcs.PutWithContentLength(request.Size),
		gcs.PutWithContentMD5(request.Checksum),
		gcs.PutWithContentType(request.ContentType),
	)
	if err != nil {
		return err
	}
	response, err := this.do(gcsRequest)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()

	_, _ = io.Copy(io.Discard, response.Body)

	if !this.isExpectedStatus(response.StatusCode) {
		if this.isSafeRetryStatus(response.StatusCode) {
			return fmt.Errorf("http error: %d (%w)", response.StatusCode, contracts.ErrRetryable)
		}
		return contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, request.RemoteAddress)
	}
	return nil
}

func (this *GoogleCloudStorageClient) Download(ctx context.Context, address url.URL) (io.ReadCloser, error) {
	gcsRequest, err := this.newRequest(ctx, "GET", address)
	if err != nil {
		return nil, err
	}
	return this.body(gcsRequest, address)
}

func (this *GoogleCloudStorageClient) Seek(ctx context.Context, address url.URL, start, end int64) (io.ReadCloser, error) {
	gcsRequest, err := this.newRequest(ctx, "GET", address)
	if err != nil {
		return nil, err
	}
	gcsRequest.Header.Add("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	return this.body(gcsRequest, address)
}

func (this *GoogleCloudStorageClient) body(request *http.Request, address url.URL) (io.ReadCloser, error) {
	response, err := this.do(request)
	if err != nil {
		return nil, err
	}
	if !this.isExpectedStatus(response.StatusCode) {
		_, _ = io.Copy(io.Discard, response.Body)
		_ = response.Body.Close()
		return nil, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, address)
	}
	return response.Body, nil
}

// Size uses an HTTP HEAD to find out how many bytes are available in total.
func (this *GoogleCloudStorageClient) Size(ctx context.Context, address url.URL) (int64, error) {
	gcsRequest, err := this.newRequest(ctx, "HEAD", address)
	if err != nil {
		return 0, err
	}
	response, err := this.do(gcsRequest)
	if err != nil {
		return 0, err
	}
	_ = response.Body.Close()
	if !this.isExpectedStatus(response.StatusCode) {
		return 0, contracts.NewStatusCodeError(response.StatusCode, this.expectedStatus, address)
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
	return slices.Contains(this.expectedStatus, statusCode)
}

// withoutClose hides any Close method on an upload body: net/http closes
// request bodies after sending, which would prevent the body from being
// rewound and sent again on retry.
type withoutClose struct{ io.Reader }

package remote

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type GoogleCloudStorageClient struct {
	client      *http.Client
	credentials gcs.Credentials
	bucket      string
}

func NewGoogleCloudStorageClient(client *http.Client, credentials gcs.Credentials, bucket string) *GoogleCloudStorageClient {
	return &GoogleCloudStorageClient{client: client, credentials: credentials, bucket: bucket}
}

func (this *GoogleCloudStorageClient) Upload(request contracts.UploadRequest) error {
	gcsRequest, err := gcs.NewRequest("PUT",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(this.bucket),
		gcs.WithResource(request.Path),
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
		return err
	}
	if response.StatusCode != http.StatusOK {
		this.dump(gcsRequest, response)
		return fmt.Errorf("non 200 status code: %s", response.Status)
	}
	return nil
}

func (this *GoogleCloudStorageClient) Download(request contracts.DownloadRequest) (io.ReadCloser, error) {
	gcsRequest, err := gcs.NewRequest("GET",
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(this.bucket),
		gcs.WithResource(request.Path),
	)
	if err != nil {
		return nil, err
	}
	response, err := this.client.Do(gcsRequest)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		this.dump(gcsRequest, response)
		return nil, fmt.Errorf("non 200 status code: %s", response.Status)
	}
	return response.Body, nil
}

func (this *GoogleCloudStorageClient) dump(request *http.Request, response *http.Response) {
	requestDump, _ := httputil.DumpRequestOut(request, false)
	responseDump, _ := httputil.DumpResponse(response, true)
	log.Printf("non 200 status code: \nrequest: \n%s\nresponse:\n%s", requestDump, responseDump)
}

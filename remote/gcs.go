package remote

import (
	"fmt"
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
		requestDump, _ := httputil.DumpRequestOut(gcsRequest, false)
		dump, _ := httputil.DumpResponse(response, true)
		log.Printf("non 200 status code: \nrequest: \n%s\nresponse:\n%s", requestDump, dump)
		return fmt.Errorf("non 200 status code: %s", response.Status)
	}
	return nil
}

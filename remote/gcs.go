package remote

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/smartystreets/gcs"

	"bitbucket.org/smartystreets/satisfy/contracts"
)

type GoogleCloudStorageUploader struct {
	client      *http.Client
	credentials gcs.Credentials
	bucket      string
}

func NewGoogleCloudStorageUploader(client *http.Client, credentials gcs.Credentials, bucket string) *GoogleCloudStorageUploader {
	return &GoogleCloudStorageUploader{client: client, credentials: credentials, bucket: bucket}
}

func (this *GoogleCloudStorageUploader) Upload(request contracts.UploadRequest) error {
	gcsRequest, err := gcs.NewRequest("PUT",
		//gcs.WithEndpoint("scheme", "host"),
		gcs.WithCredentials(this.credentials),
		gcs.WithBucket(this.bucket),
		gcs.WithResource(request.Path),
		gcs.PutWithContent(request.Body),
		gcs.PutWithContentLength(request.Size),
		//gcs.PutWithContentMD5(request.Checksum), // TODO: get this working...
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
		dump, _ := httputil.DumpResponse(response, true)
		log.Printf("non 200 status code: %s", dump)
		return fmt.Errorf("non 200 status code: %s", response.Status)
	}
	return nil
}

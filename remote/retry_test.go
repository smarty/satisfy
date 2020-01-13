package remote

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"testing"
	"time"

	"bitbucket.org/smartystreets/satisfy/contracts"
	"github.com/smartystreets/assertions/should"
	"github.com/smartystreets/clock"
	"github.com/smartystreets/gunit"
	"github.com/smartystreets/logging"
)

func TestRetryFixture(t *testing.T) {
	gunit.Run(new(RetryFixture), t)
}

type RetryFixture struct {
	*gunit.Fixture
	retryUploader *RetryUploader
	fakeUploader  *FakeUploader
}

func (this *RetryFixture) Setup() {
	this.fakeUploader = &FakeUploader{}
	this.retryUploader = NewRetryUploader(this.fakeUploader, 4)
	this.retryUploader.sleeper = clock.StayAwake()
	this.retryUploader.logger = logging.Capture()
}

func (this *RetryFixture) TestUploadCallsInner() {
	sent := contracts.UploadRequest{ContentType: "test", Body: NewFakeBody("Hello, World!")}

	err := this.retryUploader.Upload(sent)

	this.So(err, should.BeNil)
	this.So(this.fakeUploader.receivedContent, should.Resemble, []byte("Hello, World!"))
	this.So(this.fakeUploader.received.ContentType, should.Equal, "test")
}

var anError = errors.New("this is an error")

func (this *RetryFixture) TestRetryOnError() {
	this.fakeUploader.error = anError
	err := this.retryUploader.Upload(contracts.UploadRequest{Body: NewFakeBody("Hello, World!")})
	this.So(err, should.Equal, anError)
	this.So(this.fakeUploader.attempts, should.Equal, 5)
	this.So(this.retryUploader.sleeper.Naps, should.Resemble, []time.Duration{
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
		time.Second * 3,
	})
}

func (this *RetryFixture) TestRetryMadeImpossibleBySeekError() {
	this.fakeUploader.error = errors.New("upload error")
	body := NewFakeBody("Hello, World!")
	body.seekError = anError

	err := this.retryUploader.Upload(contracts.UploadRequest{Body: body})

	this.So(err, should.Equal, anError)
	this.So(this.fakeUploader.attempts, should.Equal, 1)
}

/////////////////////////////////////////////////////////////////////////////////

type FakeBody struct {
	content     []byte
	readyToRead bool
	seekError   error
}

func NewFakeBody(content string) *FakeBody {
	return &FakeBody{
		content:     []byte(content),
		readyToRead: true,
	}
}

func (this *FakeBody) Seek(offset int64, whence int) (int64, error) {
	this.readyToRead = true
	return 0, this.seekError
}

func (this *FakeBody) Read(p []byte) (n int, err error) {
	if !this.readyToRead {
		log.Panic("Not ready to read!")
	}
	this.readyToRead = false
	return copy(p, this.content), io.EOF
}

///////////////////////////////////////////////////////////////////////////////////

type FakeUploader struct {
	received        contracts.UploadRequest
	error           error
	attempts        int
	receivedContent []byte
}

func (this *FakeUploader) Upload(request contracts.UploadRequest) error {
	this.receivedContent, _ = ioutil.ReadAll(request.Body)
	this.received = request
	this.attempts++
	return this.error
}

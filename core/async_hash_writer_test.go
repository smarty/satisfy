package core

import (
	"crypto/md5"
	"strings"
	"testing"

	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestAsyncHashWriterFixture(t *testing.T) {
	gunit.Run(new(AsyncHashWriterFixture), t)
}

type AsyncHashWriterFixture struct {
	*gunit.Fixture
}

func (this *AsyncHashWriterFixture) TestHashMatchesSynchronousHashEvenWhenCallerReusesBuffer() {
	stuff := strings.Repeat("Hello, World!", 10_000)
	expected := md5.Sum([]byte(stuff))
	hasher := md5.New()
	writer := NewAsyncHashWriter(hasher)

	buffer := make([]byte, 1000)
	for remaining := stuff; len(remaining) > 0; {
		count := copy(buffer, remaining)
		written, err := writer.Write(buffer[:count])
		this.So(err, should.BeNil)
		this.So(written, should.Equal, count)
		clear(buffer) // the writer must have copied the data already
		remaining = remaining[count:]
	}
	this.So(writer.Close(), should.BeNil)

	this.So(hasher.Sum(nil), should.Resemble, expected[:])
}

func (this *AsyncHashWriterFixture) TestCloseIsIdempotentAndHandlesNoWrites() {
	hasher := md5.New()
	writer := NewAsyncHashWriter(hasher)

	this.So(writer.Close(), should.BeNil)
	this.So(writer.Close(), should.BeNil)

	expected := md5.Sum(nil)
	this.So(hasher.Sum(nil), should.Resemble, expected[:])
}

func (this *AsyncHashWriterFixture) TestWriteAfterCloseReturnsErrorInsteadOfPanicking() {
	hasher := md5.New()
	writer := NewAsyncHashWriter(hasher)
	_, _ = writer.Write([]byte("Hello"))
	this.So(writer.Close(), should.BeNil)

	written, err := writer.Write([]byte("World"))

	this.So(written, should.Equal, 0)
	this.So(err, should.Equal, ErrAsyncHashWriterClosed)
	expected := md5.Sum([]byte("Hello"))
	this.So(hasher.Sum(nil), should.Resemble, expected[:])
}

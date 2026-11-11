package shell

import (
	"bytes"
	"errors"
	"io"
	"math/rand/v2"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/smarty/assertions/should"
	"github.com/smarty/gunit"
)

func TestParallelZstdWriterFixture(t *testing.T) {
	gunit.Run(new(ParallelZstdWriterFixture), t)
}

type ParallelZstdWriterFixture struct {
	*gunit.Fixture
	buffer *bytes.Buffer
}

func (this *ParallelZstdWriterFixture) Setup() {
	this.buffer = bytes.NewBuffer(nil)
}

func (this *ParallelZstdWriterFixture) TestOutputSpanningManyFramesDecodesToOriginalInput() {
	input := parallelZstdFixtureInput(256 << 10)
	writer := this.newWriter(this.buffer, 4<<10)

	// Uneven writes exercise chunk boundaries that don't line up with Write calls.
	for remaining := input; len(remaining) > 0; {
		count := min(len(remaining), 1+rand.IntN(10_000))
		written, err := writer.Write(remaining[:count])
		this.So(err, should.BeNil)
		this.So(written, should.Equal, count)
		remaining = remaining[count:]
	}
	this.So(writer.Close(), should.BeNil)

	this.So(this.countFrames(this.buffer.Bytes()), should.Equal, 64)
	this.So(this.decode(this.buffer.Bytes()), should.Equal, input)
}

func (this *ParallelZstdWriterFixture) TestPartialFinalChunkIsFlushedOnClose() {
	input := parallelZstdFixtureInput(10<<10 + 7)
	writer := this.newWriter(this.buffer, 4<<10)

	_, _ = writer.Write(input)
	this.So(writer.Close(), should.BeNil)

	this.So(this.countFrames(this.buffer.Bytes()), should.Equal, 3)
	this.So(this.decode(this.buffer.Bytes()), should.Equal, input)
}

func (this *ParallelZstdWriterFixture) TestEmptyInputYieldsValidEmptyStream() {
	writer := this.newWriter(this.buffer, 4<<10)

	this.So(writer.Close(), should.BeNil)

	this.So(this.buffer.Len(), should.BeGreaterThan, 0)
	this.So(this.decode(this.buffer.Bytes()), should.BeEmpty)
}

func (this *ParallelZstdWriterFixture) TestCloseIsIdempotent() {
	writer := this.newWriter(this.buffer, 4<<10)
	_, _ = writer.Write([]byte("Hello World"))

	this.So(writer.Close(), should.BeNil)
	this.So(writer.Close(), should.BeNil)

	this.So(string(this.decode(this.buffer.Bytes())), should.Equal, "Hello World")
}

func (this *ParallelZstdWriterFixture) TestTargetWriteFailureIsReported() {
	failure := errors.New("disk full")
	writer := this.newWriter(&failingWriter{err: failure}, 1<<10)

	_, _ = writer.Write(parallelZstdFixtureInput(64 << 10))
	err := writer.Close()

	this.So(errors.Is(err, failure), should.BeTrue)
	_, err = writer.Write([]byte("more"))
	this.So(errors.Is(err, failure), should.BeTrue)
}

func (this *ParallelZstdWriterFixture) TestNonPositiveChunkSizeUsesDefault() {
	writer := this.newWriter(this.buffer, 0)
	this.So(writer.chunkSize, should.Equal, DefaultParallelZstdChunkSize)
	this.So(writer.Close(), should.BeNil)
}

func (this *ParallelZstdWriterFixture) newWriter(target io.Writer, chunkSize int) *ParallelZstdWriter {
	writer, err := NewParallelZstdWriter(target, zstd.SpeedFastest, chunkSize)
	this.So(err, should.BeNil)
	return writer
}

func (this *ParallelZstdWriterFixture) decode(compressed []byte) []byte {
	decoder, err := zstd.NewReader(bytes.NewReader(compressed))
	this.So(err, should.BeNil)
	defer decoder.Close()
	decoded, err := io.ReadAll(decoder)
	this.So(err, should.BeNil)
	return decoded
}

// countFrames walks the frame and block headers (RFC 8878 §3.1) to confirm
// the writer produced independent frames rather than one long stream.
func (this *ParallelZstdWriterFixture) countFrames(compressed []byte) (frames int) {
	for len(compressed) > 0 {
		var header zstd.Header
		if err := header.Decode(compressed); err != nil {
			this.So(err, should.BeNil)
			return frames
		}
		frames++
		offset := header.HeaderSize
		for last := false; !last; {
			block := int(compressed[offset]) | int(compressed[offset+1])<<8 | int(compressed[offset+2])<<16
			last = block&1 == 1
			size := block >> 3
			if (block>>1)&3 == 1 { // RLE block stores a single byte
				size = 1
			}
			offset += 3 + size
		}
		if header.HasCheckSum {
			offset += 4
		}
		compressed = compressed[offset:]
	}
	return frames
}

func parallelZstdFixtureInput(size int) []byte {
	random := rand.New(rand.NewPCG(1, 2))
	words := []string{"satisfy ", "archive ", "zstd ", "frame ", "chunk ", "parallel\n"}
	input := make([]byte, 0, size+16)
	for len(input) < size {
		input = append(input, words[random.IntN(len(words))]...)
	}
	return input[:size]
}

type failingWriter struct{ err error }

func (this *failingWriter) Write([]byte) (int, error) { return 0, this.err }

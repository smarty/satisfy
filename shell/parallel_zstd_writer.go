package shell

import (
	"errors"
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// DefaultParallelZstdChunkSize balances throughput against compression ratio:
// each chunk becomes an independent zstd frame, so smaller chunks parallelize
// better but cannot reference data in neighboring chunks.
const DefaultParallelZstdChunkSize = 8 << 20

// MaxParallelZstdConcurrency caps the worker count so memory stays bounded on
// hosts with many cores. Each worker holds an input chunk, an output frame,
// and its own encoder state; beyond this many workers, archive building is
// limited by per-file hashing rather than compression anyway.
const MaxParallelZstdConcurrency = 16

var ErrParallelZstdWriterClosed = errors.New("parallel zstd writer: write after close")

// ParallelZstdWriter compresses its input on several cores by splitting it into
// fixed-size chunks and encoding each chunk as an independent zstd frame.
// Frames are written to the underlying writer in input order. A sequence of
// concatenated frames is a valid zstd stream, so the output can be read by any
// standard zstd decoder (including older versions of satisfy).
//
// At most concurrency+2 chunks are held in memory at once: one being filled by
// Write, up to concurrency queued or encoding, and one being written out.
//
// Like zstd.Encoder, a ParallelZstdWriter is not safe for concurrent use.
type ParallelZstdWriter struct {
	encoder   *zstd.Encoder
	target    io.Writer
	chunkSize int
	buffer    []byte
	inputs    sync.Pool
	outputs   sync.Pool
	jobs      chan *parallelZstdFrame
	ordered   chan *parallelZstdFrame
	workers   sync.WaitGroup
	finished  chan struct{}
	closed    bool
	closeErr  error
	mutex     sync.Mutex
	err       error
}

type parallelZstdFrame struct {
	input   []byte
	output  []byte
	encoded chan struct{}
}

// NewParallelZstdWriter sizes its worker pool to the available cores, reduced
// only when the memory available to this process could not hold that many
// workers (see ParallelZstdConcurrency).
func NewParallelZstdWriter(target io.Writer, level zstd.EncoderLevel, chunkSize int) (*ParallelZstdWriter, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultParallelZstdChunkSize
	}
	cores := min(runtime.GOMAXPROCS(0), MaxParallelZstdConcurrency)
	concurrency := ParallelZstdConcurrency(cores, level, chunkSize, AvailableMemory())
	return NewParallelZstdWriterWithConcurrency(target, level, chunkSize, concurrency)
}

// NewParallelZstdWriterWithConcurrency uses exactly the given number of
// workers (minimum 1).
func NewParallelZstdWriterWithConcurrency(target io.Writer, level zstd.EncoderLevel, chunkSize, concurrency int) (*ParallelZstdWriter, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultParallelZstdChunkSize
	}
	concurrency = max(concurrency, 1)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(concurrency))
	if err != nil {
		return nil, err
	}
	this := &ParallelZstdWriter{
		encoder:   encoder,
		target:    target,
		chunkSize: chunkSize,
		jobs:      make(chan *parallelZstdFrame, concurrency),
		ordered:   make(chan *parallelZstdFrame, concurrency),
		finished:  make(chan struct{}),
	}
	this.inputs.New = func() any { buffer := make([]byte, 0, chunkSize); return &buffer }
	this.outputs.New = func() any { buffer := make([]byte, 0, encoder.MaxEncodedSize(chunkSize)); return &buffer }
	this.buffer = takeBuffer(&this.inputs)
	for range concurrency {
		this.workers.Go(this.encodeFrames)
	}
	go this.writeFramesInOrder()
	return this, nil
}

// Write buffers p and hands each completed chunk to the workers. Because data
// is written to the target asynchronously, a target failure is reported by a
// later Write (as soon as the next chunk is dispatched) or by Close.
func (this *ParallelZstdWriter) Write(p []byte) (int, error) {
	if this.closed {
		return 0, ErrParallelZstdWriterClosed
	}
	written := 0
	for len(p) > 0 {
		if err := this.failure(); err != nil {
			return written, err
		}
		count := min(this.chunkSize-len(this.buffer), len(p))
		this.buffer = append(this.buffer, p[:count]...)
		p = p[count:]
		written += count
		if len(this.buffer) == this.chunkSize {
			this.dispatch()
		}
	}
	return written, nil
}

func (this *ParallelZstdWriter) Close() error {
	if this.closed {
		return this.closeErr
	}
	this.closed = true
	this.dispatch()
	close(this.jobs)
	close(this.ordered)
	<-this.finished
	this.workers.Wait()
	this.closeErr = errors.Join(this.failure(), this.encoder.Close())
	return this.closeErr
}

// dispatch queues the current chunk for encoding and reserves its place in the
// output order. Blocking on the bounded ordered queue is what caps memory use.
func (this *ParallelZstdWriter) dispatch() {
	if len(this.buffer) == 0 {
		return
	}
	frame := &parallelZstdFrame{input: this.buffer, encoded: make(chan struct{})}
	this.buffer = takeBuffer(&this.inputs)
	this.ordered <- frame
	this.jobs <- frame
}

func (this *ParallelZstdWriter) encodeFrames() {
	for frame := range this.jobs {
		if this.failure() == nil { // after a failure, skip the (wasted) compression work
			frame.output = this.encoder.EncodeAll(frame.input, takeBuffer(&this.outputs))
		}
		giveBuffer(&this.inputs, frame.input)
		frame.input = nil
		close(frame.encoded)
	}
}

func (this *ParallelZstdWriter) writeFramesInOrder() {
	defer close(this.finished)
	wroteAny := false
	for frame := range this.ordered {
		<-frame.encoded
		if frame.output != nil {
			this.writeFrame(frame.output)
			giveBuffer(&this.outputs, frame.output)
		}
		wroteAny = true
	}
	if !wroteAny {
		// Match the single-stream encoder: empty input still yields a valid frame.
		this.writeFrame(this.encoder.EncodeAll(nil, nil))
	}
}

func (this *ParallelZstdWriter) writeFrame(frame []byte) {
	if this.failure() != nil {
		return // keep draining so workers and Close are never blocked
	}
	if _, err := this.target.Write(frame); err != nil {
		this.mutex.Lock()
		this.err = err
		this.mutex.Unlock()
	}
}

func (this *ParallelZstdWriter) failure() error {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	return this.err
}

func takeBuffer(pool *sync.Pool) []byte {
	return (*pool.Get().(*[]byte))[:0]
}
func giveBuffer(pool *sync.Pool, buffer []byte) {
	buffer = buffer[:0]
	pool.Put(&buffer)
}

package shell

import (
	"io"
	"runtime"
	"sync"

	"github.com/klauspost/compress/zstd"
)

// DefaultParallelZstdChunkSize balances throughput against compression ratio:
// each chunk becomes an independent zstd frame, so smaller chunks parallelize
// better but cannot reference data in neighboring chunks.
const DefaultParallelZstdChunkSize = 8 << 20

// ParallelZstdWriter compresses its input on all available cores by splitting
// it into fixed-size chunks and encoding each chunk as an independent zstd
// frame. Frames are written to the underlying writer in input order. A
// sequence of concatenated frames is a valid zstd stream, so the output can be
// read by any standard zstd decoder (including older versions of satisfy).
type ParallelZstdWriter struct {
	encoder   *zstd.Encoder
	target    io.Writer
	chunkSize int
	buffer    []byte
	buffers   sync.Pool
	pending   chan chan []byte
	workers   chan struct{}
	finished  chan struct{}
	closeOnce sync.Once
	closeErr  error
	mutex     sync.Mutex
	err       error
}

func NewParallelZstdWriter(target io.Writer, level zstd.EncoderLevel, chunkSize int) (*ParallelZstdWriter, error) {
	if chunkSize <= 0 {
		chunkSize = DefaultParallelZstdChunkSize
	}
	concurrency := runtime.GOMAXPROCS(0)
	encoder, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(level), zstd.WithEncoderConcurrency(concurrency))
	if err != nil {
		return nil, err
	}
	this := &ParallelZstdWriter{
		encoder:   encoder,
		target:    target,
		chunkSize: chunkSize,
		pending:   make(chan chan []byte, concurrency),
		workers:   make(chan struct{}, concurrency),
		finished:  make(chan struct{}),
	}
	this.buffers.New = func() any { buffer := make([]byte, 0, chunkSize); return &buffer }
	this.buffer = this.nextBuffer()
	go this.writeFramesInOrder()
	return this, nil
}

func (this *ParallelZstdWriter) Write(p []byte) (int, error) {
	if err := this.failure(); err != nil {
		return 0, err
	}
	written := len(p)
	for len(p) > 0 {
		count := min(this.chunkSize-len(this.buffer), len(p))
		this.buffer = append(this.buffer, p[:count]...)
		p = p[count:]
		if len(this.buffer) == this.chunkSize {
			this.dispatch()
		}
	}
	return written, nil
}

func (this *ParallelZstdWriter) Close() error {
	this.closeOnce.Do(func() {
		this.dispatch()
		close(this.pending)
		<-this.finished
		this.closeErr = this.failure()
		_ = this.encoder.Close()
	})
	return this.closeErr
}

// dispatch hands the current chunk to a worker. Workers are bounded, and so is
// the number of chunks awaiting their turn to be written, which caps memory use.
func (this *ParallelZstdWriter) dispatch() {
	if len(this.buffer) == 0 {
		return
	}
	chunk := this.buffer
	this.buffer = this.nextBuffer()

	frame := make(chan []byte, 1)
	this.pending <- frame
	this.workers <- struct{}{}
	go func() {
		defer func() { <-this.workers }()
		frame <- this.encoder.EncodeAll(chunk, make([]byte, 0, len(chunk)/2))
		chunk = chunk[:0]
		this.buffers.Put(&chunk)
	}()
}

func (this *ParallelZstdWriter) writeFramesInOrder() {
	defer close(this.finished)
	wroteAny := false
	for frame := range this.pending {
		this.writeFrame(<-frame)
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

func (this *ParallelZstdWriter) nextBuffer() []byte {
	return (*this.buffers.Get().(*[]byte))[:0]
}

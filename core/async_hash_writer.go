package core

import (
	"hash"
	"sync"
)

// AsyncHashWriter feeds a hash on its own goroutine so hashing overlaps with
// whatever else consumes the same data (e.g. archiving and compression)
// instead of running in series with it. Each Write copies its input, so
// callers may reuse their buffers immediately. Close must be called before
// reading the hash's Sum.
type AsyncHashWriter struct {
	hasher   hash.Hash
	pending  chan []byte
	buffers  sync.Pool
	finished chan struct{}
	once     sync.Once
}

// asyncHashQueueDepth bounds how far the hasher may fall behind, which caps
// memory at roughly asyncHashQueueDepth × the caller's write size.
const asyncHashQueueDepth = 16

func NewAsyncHashWriter(hasher hash.Hash) *AsyncHashWriter {
	this := &AsyncHashWriter{
		hasher:   hasher,
		pending:  make(chan []byte, asyncHashQueueDepth),
		finished: make(chan struct{}),
	}
	go this.hashInBackground()
	return this
}

func (this *AsyncHashWriter) Write(p []byte) (int, error) {
	buffer := this.takeBuffer()
	buffer = append(buffer, p...)
	this.pending <- buffer
	return len(p), nil
}

// Close waits until every written byte has been hashed. It is safe to call
// more than once.
func (this *AsyncHashWriter) Close() error {
	this.once.Do(func() {
		close(this.pending)
		<-this.finished
	})
	return nil
}

func (this *AsyncHashWriter) hashInBackground() {
	defer close(this.finished)
	for buffer := range this.pending {
		_, _ = this.hasher.Write(buffer)
		buffer = buffer[:0]
		this.buffers.Put(&buffer)
	}
}

func (this *AsyncHashWriter) takeBuffer() []byte {
	if buffer, ok := this.buffers.Get().(*[]byte); ok {
		return (*buffer)[:0]
	}
	return nil
}

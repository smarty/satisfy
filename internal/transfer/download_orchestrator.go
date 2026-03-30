package transfer

import (
	"sync"

	"github.com/smarty/satisfy/contracts"
)

type downloadOrchestrator struct {
	results chan error
	events  chan contracts.Event
	done    chan struct{}
	once    sync.Once
}

func newDownloadOrchestrator(count int) *downloadOrchestrator {
	return &downloadOrchestrator{
		results: make(chan error, count),
		events:  make(chan contracts.Event),
		done:    make(chan struct{}),
	}
}

func (this *downloadOrchestrator) emitEvent(e contracts.Event) {
	select {
	case this.events <- e:
	case <-this.done:
	}
}

func (this *downloadOrchestrator) emitError(err error) {
	select {
	case this.results <- err:
	case <-this.done:
	}
}

func (this *downloadOrchestrator) cancel() {
	this.once.Do(func() { close(this.done) })
}

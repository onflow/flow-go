package fixtures

import (
	"github.com/onflow/flow-go/ledger/complete"
)

type NoopCompactor struct {
	stopCh       chan struct{}
	trieUpdateCh <-chan *complete.WALTrieUpdate
}

func NewNoopCompactor(l *complete.Ledger) *NoopCompactor {
	return &NoopCompactor{
		stopCh:       make(chan struct{}),
		trieUpdateCh: l.TrieUpdateChan(),
	}
}

func (c *NoopCompactor) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	go c.run()
	return ch
}

func (c *NoopCompactor) Done() <-chan struct{} {
	close(c.stopCh)
	return c.stopCh
}

func (c *NoopCompactor) run() {
	for {
		select {
		case <-c.stopCh:
			return
		case update, ok := <-c.trieUpdateCh:
			if !ok {
				continue
			}

			// Send result of WAL update
			update.ResultCh <- nil

			// Wait for trie update
			<-update.TrieCh
		}
	}
}

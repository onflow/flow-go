package fixtures

import (
	"github.com/onflow/flow-go/ledger/complete"
)

// NoopPayloadlessCompactor is the payloadless analog of [NoopCompactor]: it
// drains a [complete.PayloadlessLedger]'s trie-update channel without writing
// to a WAL or producing checkpoints, so unit tests using a channel-backed
// ledger don't deadlock waiting for a real compactor.
type NoopPayloadlessCompactor struct {
	stopCh       chan struct{}
	trieUpdateCh <-chan *complete.WALPayloadlessTrieUpdate
}

// NewNoopPayloadlessCompactor wires the noop compactor to the ledger's
// trie-update channel. The ledger must have been constructed with a non-nil
// WAL so its channel is non-nil.
func NewNoopPayloadlessCompactor(l *complete.PayloadlessLedger) *NoopPayloadlessCompactor {
	return &NoopPayloadlessCompactor{
		stopCh:       make(chan struct{}),
		trieUpdateCh: l.TrieUpdateChan(),
	}
}

// Ready starts the drain goroutine and returns an already-closed channel.
func (c *NoopPayloadlessCompactor) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	go c.run()
	return ch
}

// Done stops the drain goroutine and returns an already-closed channel.
func (c *NoopPayloadlessCompactor) Done() <-chan struct{} {
	close(c.stopCh)
	return c.stopCh
}

func (c *NoopPayloadlessCompactor) run() {
	for {
		select {
		case <-c.stopCh:
			return
		case update, ok := <-c.trieUpdateCh:
			if !ok {
				continue
			}
			// Acknowledge the WAL write so the ledger's Set returns.
			update.ResultCh <- nil
			// Drain the trie that the ledger sends after computing its new state.
			<-update.TrieCh
		}
	}
}

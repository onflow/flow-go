package fixtures

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/trie"
	"github.com/onflow/flow-go/ledger/complete/wal"
)

type NoopWAL struct{}

func (w *NoopWAL) Ready() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (w *NoopWAL) Done() <-chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}

func (w *NoopWAL) NewCheckpointer() (*wal.Checkpointer, error) {
	return nil, nil
}

func (w *NoopWAL) PauseRecord() {}

func (w *NoopWAL) UnpauseRecord() {}

func (w *NoopWAL) RecordUpdate(update *ledger.TrieUpdate) error { return nil }

func (w *NoopWAL) RecordDelete(rootHash ledger.RootHash) error { return nil }

func (w *NoopWAL) ReplayOnForest(forest *mtrie.Forest) error { return nil }

func (w *NoopWAL) Segments() (first, last int, err error) { return 0, 0, nil }

func (w *NoopWAL) Replay(checkpointFn func(tries []*trie.MTrie) error, updateFn func(update *ledger.TrieUpdate) error, deleteFn func(ledger.RootHash) error) error {
	return nil
}

func (w *NoopWAL) ReplayLogsOnly(checkpointFn func(tries []*trie.MTrie) error, updateFn func(update *ledger.TrieUpdate) error, deleteFn func(rootHash ledger.RootHash) error) error {
	return nil
}

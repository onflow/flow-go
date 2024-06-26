package committer

import (
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

type NoopViewCommitter struct {
}

func NewNoopViewCommitter() *NoopViewCommitter {
	return &NoopViewCommitter{}
}

func (NoopViewCommitter) CommitView(
	_ *snapshot.ExecutionSnapshot,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (
	flow.StateCommitment,
	[]byte,
	*ledger.TrieUpdate,
	execution.ExtendableStorageSnapshot,
	error,
) {

	trieUpdate := &ledger.TrieUpdate{
		RootHash: ledger.RootHash(baseStorageSnapshot.Commitment()),
	}
	return baseStorageSnapshot.Commitment(), []byte{}, trieUpdate, baseStorageSnapshot, nil
}

package committer

import (
	"github.com/onflow/flow-go/engine/execution/storehouse"
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
	baseStorageSnapshot storehouse.ExtendableStorageSnapshot,
) (
	flow.StateCommitment,
	[]byte,
	*ledger.TrieUpdate,
	storehouse.ExtendableStorageSnapshot,
	error,
) {
	return baseStorageSnapshot.Commitment(), nil, nil, nil, nil
}

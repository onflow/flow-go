package committer

import (
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
	s flow.StateCommitment,
) (
	flow.StateCommitment,
	[]byte,
	*ledger.TrieUpdate,
	error,
) {
	return s, nil, nil, nil
}

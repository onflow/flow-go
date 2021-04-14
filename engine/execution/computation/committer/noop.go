package committer

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type NoopViewCommitter struct {
}

func NewNoopViewCommitter() *NoopViewCommitter {
	return &NoopViewCommitter{}
}

func (n NoopViewCommitter) CommitView(_ state.View, s flow.StateCommitment) (flow.StateCommitment, []byte, error) {
	return s, nil, nil
}

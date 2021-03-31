package computation

import (
	"fmt"

	execState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type LedgerViewCommitter struct {
	ldg    ledger.Ledger
	tracer module.Tracer
}

func NewLedgerViewCommitter(ldg ledger.Ledger, tracer module.Tracer) *LedgerViewCommitter {
	return &LedgerViewCommitter{ldg: ldg, tracer: tracer}
}

func (s *LedgerViewCommitter) CommitView(view state.View, baseState flow.StateCommitment) (flow.StateCommitment, []byte, error) {
	return CommitView(s.ldg, view, baseState)
}

func CommitView(ldg ledger.Ledger, view state.View, baseState flow.StateCommitment) (flow.StateCommitment, []byte, error) {
	ids, values := view.RegisterUpdates()

	update, err := ledger.NewUpdate(
		baseState,
		execState.RegisterIDSToKeys(ids),
		execState.RegisterValuesToValues(values),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	newCommit, err := ldg.Set(update)
	if err != nil {
		return nil, nil, err
	}

	query, err := makeQuery(baseState, view.AllRegisters())
	if err != nil {
		return nil, nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	proof, err := ldg.Prove(query)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get proof: %w", err)
	}

	return newCommit, proof, nil
}

func makeQuery(commitment flow.StateCommitment, ids []flow.RegisterID) (*ledger.Query, error) {

	keys := make([]ledger.Key, len(ids))
	for i, id := range ids {
		keys[i] = execState.RegisterIDToKey(id)
	}

	return ledger.NewQuery(commitment, keys)
}

type NoopViewCommitter struct {
}

func NewNoopViewCommitter() *NoopViewCommitter {
	return &NoopViewCommitter{}
}

func (n NoopViewCommitter) CommitView(state.View, flow.StateCommitment) (flow.StateCommitment, []byte, error) {
	return nil, nil, nil
}

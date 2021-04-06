package computation

import (
	"fmt"
	"sync"

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

func (s *LedgerViewCommitter) CommitView(view state.View, baseState flow.StateCommitment) (newCommit flow.StateCommitment, proof []byte, err error) {
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proof, err2 = CollectProofs(s.ldg, view, baseState)
		wg.Done()
	}()

	newCommit, err1 = CommitView(s.ldg, view, baseState)
	wg.Wait()

	if err1 != nil {
		err = err1
	}
	if err2 != nil {
		err = err1
	}
	return
}

func CommitView(ldg ledger.Ledger, view state.View, baseState flow.StateCommitment) (newCommit flow.StateCommitment, err error) {
	ids, values := view.RegisterUpdates()
	update, err := ledger.NewUpdate(
		baseState,
		execState.RegisterIDSToKeys(ids),
		execState.RegisterValuesToValues(values),
	)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	return ldg.Set(update)
}

func CollectProofs(ldg ledger.Ledger, view state.View, baseState flow.StateCommitment) (proof []byte, err error) {
	allIds := view.AllRegisters()
	keys := make([]ledger.Key, len(allIds))
	for i, id := range allIds {
		keys[i] = execState.RegisterIDToKey(id)
	}

	query, err := ledger.NewQuery(baseState, keys)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	return ldg.Prove(query)
}

type NoopViewCommitter struct {
}

func NewNoopViewCommitter() *NoopViewCommitter {
	return &NoopViewCommitter{}
}

func (n NoopViewCommitter) CommitView(_ state.View, s flow.StateCommitment) (flow.StateCommitment, []byte, error) {
	return s, nil, nil
}

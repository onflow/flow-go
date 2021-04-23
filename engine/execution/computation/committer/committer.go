package committer

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"

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
		proof, err2 = s.collectProofs(view, baseState)
		wg.Done()
	}()

	newCommit, err1 = s.commitView(view, baseState)
	wg.Wait()

	if err1 != nil {
		err = multierror.Append(err, err1)
	}
	if err2 != nil {
		err = multierror.Append(err, err2)
	}
	return
}

func (s *LedgerViewCommitter) commitView(view state.View, baseState flow.StateCommitment) (newCommit flow.StateCommitment, err error) {
	ids, values := view.RegisterUpdates()
	update, err := ledger.NewUpdate(
		ledger.State(baseState),
		execState.RegisterIDSToKeys(ids),
		execState.RegisterValuesToValues(values),
	)
	if err != nil {
		return flow.DummyStateCommitment, fmt.Errorf("cannot create ledger update: %w", err)
	}

	state, err := s.ldg.Set(update)
	return flow.StateCommitment(state), err
}

func (s *LedgerViewCommitter) collectProofs(view state.View, baseState flow.StateCommitment) (proof []byte, err error) {
	allIds := view.AllRegisters()
	keys := make([]ledger.Key, len(allIds))
	for i, id := range allIds {
		keys[i] = execState.RegisterIDToKey(id)
	}

	query, err := ledger.NewQuery(ledger.State(baseState), keys)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	return s.ldg.Prove(query)
}

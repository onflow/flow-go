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

func (s *LedgerViewCommitter) CommitView(view state.View, baseState flow.StateCommitment) (newCommit flow.StateCommitment, proof []byte, trieUpdate *ledger.TrieUpdate, err error) {
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proof, err2 = s.collectProofs(view, baseState)
		wg.Done()
	}()

	newCommit, trieUpdate, err1 = s.commitView(view, baseState)
	wg.Wait()

	if err1 != nil {
		err = multierror.Append(err, err1)
	}
	if err2 != nil {
		err = multierror.Append(err, err2)
	}
	return
}

func (s *LedgerViewCommitter) commitView(view state.View, baseState flow.StateCommitment) (newCommit flow.StateCommitment, update *ledger.TrieUpdate, err error) {
	return execState.CommitDelta(s.ldg, view, baseState)
}

func (s *LedgerViewCommitter) collectProofs(view state.View, baseState flow.StateCommitment) (proof []byte, err error) {
	// get all deduplicated register IDs
	allIds := view.AllRegisters()
	keys := make([]ledger.Key, 0, len(allIds))
	for _, id := range allIds {
		keys = append(keys, execState.RegisterIDToKey(id))
	}

	query, err := ledger.NewQuery(ledger.State(baseState), keys)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	return s.ldg.Prove(query)
}

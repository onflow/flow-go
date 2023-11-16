package committer

import (
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"

	"github.com/onflow/flow-go/engine/execution"
	execState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

type LedgerViewCommitter struct {
	ledger ledger.Ledger
	tracer module.Tracer
}

func NewLedgerViewCommitter(
	ledger ledger.Ledger,
	tracer module.Tracer,
) *LedgerViewCommitter {
	return &LedgerViewCommitter{
		ledger: ledger,
		tracer: tracer,
	}
}

func (committer *LedgerViewCommitter) CommitView(
	snapshot *snapshot.ExecutionSnapshot,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (
	newCommit flow.StateCommitment,
	proof []byte,
	trieUpdate *ledger.TrieUpdate,
	newStorageSnapshot execution.ExtendableStorageSnapshot,
	err error,
) {
	var err1, err2 error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		proof, err2 = committer.collectProofs(snapshot, baseStorageSnapshot)
		wg.Done()
	}()

	newCommit, trieUpdate, err1 = execState.CommitDelta(
		committer.ledger,
		snapshot,
		baseStorageSnapshot)
	wg.Wait()

	if err1 != nil {
		err = multierror.Append(err, err1)
	}
	if err2 != nil {
		err = multierror.Append(err, err2)
	}
	return
}

func (committer *LedgerViewCommitter) collectProofs(
	snapshot *snapshot.ExecutionSnapshot,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (
	proof []byte,
	err error,
) {
	baseState := baseStorageSnapshot.Commitment()
	// Reason for including AllRegisterIDs (read and written registers) instead of ReadRegisterIDs (only read registers):
	// AllRegisterIDs returns deduplicated register IDs that were touched by both
	// reads and writes during the block execution.
	// Verification nodes only need the registers in the storage proof that were touched by reads
	// in order to execute transactions in a chunk. However, without the registers touched
	// by writes, especially the interim trie nodes for them, verification nodes won't be
	// able to reconstruct the trie root hash of the execution state post execution. That's why
	// the storage proof needs both read registers and write registers, which specifically is AllRegisterIDs
	allIds := snapshot.AllRegisterIDs()
	keys := make([]ledger.Key, 0, len(allIds))
	for _, id := range allIds {
		keys = append(keys, convert.RegisterIDToLedgerKey(id))
	}

	query, err := ledger.NewQuery(ledger.State(baseState), keys)
	if err != nil {
		return nil, fmt.Errorf("cannot create ledger query: %w", err)
	}

	return committer.ledger.Prove(query)
}

package committer

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	execState "github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// PayloadlessLedgerViewCommitter commits an execution snapshot to a
// payloadless ledger and returns a proof whose leaves are reconstructed back
// to full ledger payloads using values read from the pre-execution storage
// snapshot. The returned bytes are wire-compatible with the full mtrie's
// proof format: downstream consumers decode them with
// ledger.DecodeTrieBatchProof, identical to full-mode behaviour.
//
// It is the payloadless-mode counterpart of LedgerViewCommitter and satisfies
// the same computer.ViewCommitter interface. Mode is selected by which
// constructor is called at startup; there is no runtime mode flag.
type PayloadlessLedgerViewCommitter struct {
	ledger            ledger.PayloadlessLedger
	tracer            module.Tracer
	logger            zerolog.Logger
	pathFinderVersion uint8
}

// NewPayloadlessLedgerViewCommitter returns a committer that drives a
// payloadless ledger and emits reconstructed full-format proof bytes.
//
// pathFinderVersion must match the version used by the underlying ledger so
// the path → registerID mapping built during reconstruction agrees with the
// paths carried by the proof. In production this is
// complete.DefaultPathFinderVersion.
//
// logger receives a per-collection debug line reporting proof-generation
// timing (see collectProofs). It is emitted at debug level so production stays
// quiet; benchmark tooling passes a debug-level logger to surface the numbers.
func NewPayloadlessLedgerViewCommitter(
	ledger ledger.PayloadlessLedger,
	tracer module.Tracer,
	logger zerolog.Logger,
	pathFinderVersion uint8,
) *PayloadlessLedgerViewCommitter {
	return &PayloadlessLedgerViewCommitter{
		ledger:            ledger,
		tracer:            tracer,
		logger:            logger,
		pathFinderVersion: pathFinderVersion,
	}
}

// CommitView commits an execution snapshot and collects a payloadless proof.
// Concurrency: state commitment and proof collection run in parallel, matching LedgerViewCommitter.
func (committer *PayloadlessLedgerViewCommitter) CommitView(
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

	newCommit, trieUpdate, newStorageSnapshot, err1 = committer.commitDelta(snapshot, baseStorageSnapshot)
	wg.Wait()

	if err1 != nil {
		err = multierror.Append(err, err1)
	}
	if err2 != nil {
		err = multierror.Append(err, err2)
	}
	return
}

// commitDelta mirrors execState.CommitDelta but accepts a PayloadlessLedger
// directly so we don't have to widen the shared helper's ledger.Ledger
// dependency. The body is byte-for-byte equivalent to execState.CommitDelta
// up to the ledger.Set call.
func (committer *PayloadlessLedgerViewCommitter) commitDelta(
	ruh execState.RegisterUpdatesHolder,
	baseStorageSnapshot execution.ExtendableStorageSnapshot,
) (flow.StateCommitment, *ledger.TrieUpdate, execution.ExtendableStorageSnapshot, error) {

	updatedRegisters := ruh.UpdatedRegisters()
	keys, values := execState.RegisterEntriesToKeysValues(updatedRegisters)
	baseState := baseStorageSnapshot.Commitment()
	update, err := ledger.NewUpdate(ledger.State(baseState), keys, values)
	if err != nil {
		return flow.DummyStateCommitment, nil, nil, fmt.Errorf("cannot create ledger update: %w", err)
	}

	newState, trieUpdate, err := committer.ledger.Set(update)
	if err != nil {
		return flow.DummyStateCommitment, nil, nil, fmt.Errorf("could not update ledger: %w", err)
	}

	newCommit := flow.StateCommitment(newState)
	newStorageSnapshot := baseStorageSnapshot.Extend(newCommit, ruh.UpdatedRegisterSet())

	return newCommit, trieUpdate, newStorageSnapshot, nil
}

// collectProofs queries the payloadless ledger for a proof over all registers
// touched by the execution snapshot (read and written), then reconstructs the
// proof's leaf values from the pre-execution storage snapshot. The returned
// bytes encode a *ledger.TrieBatchProof — wire-compatible with the full
// committer's output.
func (committer *PayloadlessLedgerViewCommitter) collectProofs(
	execSnapshot *snapshot.ExecutionSnapshot,
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
	allIds := execSnapshot.AllRegisterIDs()

	// The proof is generated from baseState (pre-execution), so we read
	// pre-execution values to re-attach to leaves during reconstruction.
	valueReader := func(id flow.RegisterID) (flow.RegisterValue, error) {
		return baseStorageSnapshot.Get(id)
	}

	start := time.Now()
	proof, err = payloadless.ProveAndReconstruct(
		committer.ledger,
		ledger.State(baseState),
		allIds,
		valueReader,
		committer.pathFinderVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("could not collect payloadless proof: %w", err)
	}

	// Proof-generation cost per collection. This is the exact work the
	// ProveAndReconstruct TODO(perf) optimizes, so it is the ground-truth
	// measurement for before/after benchmarking. Debug level keeps production
	// quiet; benchmark tooling raises this logger to debug to surface it.
	committer.logger.Debug().
		Int("register_count", len(allIds)).
		Int("proof_bytes", len(proof)).
		Dur("prove_and_reconstruct_duration", time.Since(start)).
		Msg("payloadless proof generated")

	return proof, nil
}

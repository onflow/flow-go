package partial

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
)

// PathFinderVersion captures the version of path finder that the partial ledger uses
const PathFinderVersion = 0

// Ledger implements the ledger functionality for a limited subset of keys (partial ledger).
// Partial ledgers are designed to be constructed and verified by a collection of proofs from a complete ledger.
// The partial ledger uses a partial binary Merkle trie which holds intermediate hash value for the pruned branched and prevents updates to keys that were not part of proofs.
type Ledger struct {
	ptrie *ptrie.PSMT
	state ledger.State
	proof ledger.Proof
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(proof ledger.Proof, s ledger.State) (*Ledger, error) {

	// Decode proof encodings
	if len(proof) < 1 {
		return nil, fmt.Errorf("at least a proof is needed to be able to contruct a partial trie")
	}
	batchProof, err := encoding.DecodeTrieBatchProof(proof)
	if err != nil {
		return nil, fmt.Errorf("decoding proof failed: %w", err)
	}

	// decode proof
	psmt, err := ptrie.NewPSMT(s, pathfinder.PathByteSize, batchProof)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, ledger.NewErrLedgerConstruction(err)
	}

	return &Ledger{ptrie: psmt, proof: proof}, nil
}

// Ready implements interface module.ReadyDoneAware
func (l *Ledger) Ready() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}

// Done implements interface module.ReadyDoneAware
func (l *Ledger) Done() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

// InitState returns the initial state of the ledger
func (l *Ledger) InitState() ledger.State {
	return l.state
}

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	// TODO compare query.State() to the ledger sc
	paths, err := pathfinder.KeysToPaths(query.Keys(), PathFinderVersion)
	if err != nil {
		return nil, err
	}
	// TODO deal with failedPaths
	payloads, _, err := l.ptrie.Get(paths)
	if err != nil {
		return nil, err
	}
	values, err = pathfinder.PayloadsToValues(payloads)
	if err != nil {
		return nil, err
	}
	return values, err
}

// Set updates the ledger given an update
// it returns the state after update and errors (if any)
func (l *Ledger) Set(update *ledger.Update) (newState ledger.State, err error) {
	// TODO: add test case
	if update.Size() == 0 {
		// return current state root unchanged
		return update.State(), nil
	}

	trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, PathFinderVersion)
	if err != nil {
		return nil, err
	}

	// TODO handle failed paths
	newRootHash, _, err := l.ptrie.Update(trieUpdate.Paths, trieUpdate.Payloads)
	if err != nil {
		return nil, err
	}

	// TODO log info state
	return ledger.State(newRootHash), nil
}

// Prove provides proofs for a ledger query and errors (if any)
// TODO implement this by iterating over initial proofs to find the ones for the query
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {
	return nil, err
}

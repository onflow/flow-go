package partial

import (
	"fmt"

	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common/encoding"
	"github.com/dapperlabs/flow-go/ledger/common/pathfinder"
	"github.com/dapperlabs/flow-go/ledger/partial/ptrie"
)

// TODO(Ramtin) add metrics
// TODO(Ramtin) deal with PathByteSize (move it to pathfinder)
// TODO(Ramtin) add Path Finder Version

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
	paths, err := pathfinder.KeysToPaths(query.Keys(), 0)
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

	trieUpdate, err := pathfinder.UpdateToTrieUpdate(update, 0)
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

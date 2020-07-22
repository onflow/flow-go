package partial

import (
	"github.com/dapperlabs/flow-go/ledger"
	"github.com/dapperlabs/flow-go/ledger/common"
	"github.com/dapperlabs/flow-go/ledger/partial/ptrie"
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO (Ramtin) add metrics
// TODO deal with PathByteSize (move it to pathfinder)
// Path Finder Version

type Ledger struct {
	ptrie *ptrie.PSMT
	sc    ledger.StateCommitment
	proof ledger.Proof
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(proof ledger.Proof, sc ledger.StateCommitment) (*Ledger, error) {

	// decode proof
	// TODO fix the byte size
	psmt, err := ptrie.NewPSMT(sc, 32, proof)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, ledger.NewErrLedgerConstruction(err)
	}

	return &Ledger{ptrie: psmt}, nil
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

// InitStateCommitment returns the state commitment of the initial state of the ledger
func (l *Ledger) InitStateCommitment() flow.StateCommitment {
	return l.sc
}

// Get read the values of the given keys at the given state commitment
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	// TODO compare query.StateCommitment() to the ledger sc
	paths, err := common.KeysToPaths(query.Keys(), 0)
	if err != nil {
		return nil, err
	}
	// TODO deal with failedPaths
	payloads, _, err := l.ptrie.Get(paths)
	if err != nil {
		return nil, err
	}
	values, err = common.PayloadsToValues(payloads)
	if err != nil {
		return nil, err
	}
	return values, err
}

// Set updates the ledger given an update
// it returns a new state commitment (state after update) and errors (if any)
func (l *Ledger) Set(update *ledger.Update) (newStateCommitment ledger.StateCommitment, err error) {
	// TODO: add test case
	if update.Size() == 0 {
		// return current state root unchanged
		return update.StateCommitment(), nil
	}

	trieUpdate, err := common.UpdateToTrieUpdate(update, 0)
	if err != nil {
		return nil, err
	}

	// TODO handle failed paths
	newRootHash, _, err := l.ptrie.Update(trieUpdate.Paths, trieUpdate.Payloads)
	if err != nil {
		return nil, err
	}

	// TODO log info state commitments
	return ledger.StateCommitment(newRootHash), nil
}

// Prove provides proofs for a ledger query and errors (if any)
// TODO implement this by iterating over initial proofs to find the ones for the query
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {
	return nil, err
}

// TODO implement an approximate MemSize method
func (l *Ledger) MemSize() (int64, error) {
	return 0, nil
}

// DiskSize returns the amount of disk space used by the storage (in bytes)
func (l *Ledger) DiskSize() (int64, error) {
	return 0, nil
}

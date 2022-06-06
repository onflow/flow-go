package partial

import (
	"fmt"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/encoding"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/partial/ptrie"
)

const DefaultPathFinderVersion = 1

// Ledger implements the ledger functionality for a limited subset of keys (partial ledger).
// Partial ledgers are designed to be constructed and verified by a collection of proofs from a complete ledger.
// The partial ledger uses a partial binary Merkle trie which holds intermediate hash value for the pruned branched and prevents updates to keys that were not part of proofs.
type Ledger struct {
	ptrie             *ptrie.PSMT
	state             ledger.State
	proof             ledger.Proof
	pathFinderVersion uint8
}

// NewLedger creates a new in-memory trie-backed ledger storage with persistence.
func NewLedger(proof ledger.Proof, s ledger.State, pathFinderVer uint8) (*Ledger, error) {

	// Decode proof encodings
	if len(proof) < 1 {
		return nil, fmt.Errorf("at least a proof is needed to be able to contruct a partial trie")
	}
	batchProof, err := encoding.DecodeTrieBatchProof(proof)
	if err != nil {
		return nil, fmt.Errorf("decoding proof failed: %w", err)
	}

	// decode proof
	psmt, err := ptrie.NewPSMT(ledger.RootHash(s), batchProof)

	if err != nil {
		// TODO provide more details based on the error type
		return nil, ledger.NewErrLedgerConstruction(err)
	}

	return &Ledger{ptrie: psmt, proof: proof, state: s, pathFinderVersion: pathFinderVer}, nil
}

func (l *Ledger) PathFinderVersion() uint8 {
	return l.pathFinderVersion
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

// InitialState returns the initial state of the ledger
func (l *Ledger) InitialState() ledger.State {
	return l.state
}

// GetSingleValue reads value of a given key at the given state
func (l *Ledger) GetSingleValue(trieRead *ledger.TrieReadSingleValue) (value ledger.Value, err error) {
	payload, err := l.ptrie.GetSinglePayload(trieRead.Path)
	if err != nil {
		return nil, err
	}
	return payload.Value, err
}

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(trieRead *ledger.TrieRead) (values []ledger.Value, err error) {
	// TODO compare query.State() to the ledger sc
	payloads, err := l.ptrie.Get(trieRead.Paths)
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
func (l *Ledger) Set(trieUpdate *ledger.TrieUpdate) (newState ledger.State, err error) {
	// TODO: add test case
	if trieUpdate.Size() == 0 {
		// return current state root unchanged
		return trieUpdate.State(), nil
	}
	newRootHash, err := l.ptrie.Update(trieUpdate.Paths, trieUpdate.Payloads)
	if err != nil {
		// Returned error type is ledger.ErrMissingKeys
		return ledger.DummyState, err
	}

	// TODO log info state
	return ledger.State(newRootHash), nil
}

// Prove provides proofs for a ledger query and errors (if any)
// TODO implement this by iterating over initial proofs to find the ones for the query
func (l *Ledger) Prove(trieRead *ledger.TrieRead) (proof ledger.Proof, err error) {
	return nil, err
}

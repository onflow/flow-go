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

// Get read the values of the given keys at the given state
// it returns the values in the same order as given registerIDs and errors (if any)
func (l *Ledger) Get(query *ledger.Query) (values []ledger.Value, err error) {
	// TODO compare query.State() to the ledger sc
	paths, err := pathfinder.KeysToPaths(query.Keys(), l.pathFinderVersion)
	if err != nil {
		return nil, err
	}
	payloads, err := l.ptrie.Get(paths)
	if err != nil {
		if pErr, ok := err.(*ptrie.ErrMissingPath); ok {
			//store mappings and restore keys from missing paths
			pathToKey := make(map[ledger.Path]ledger.Key)

			for i, key := range query.Keys() {
				path := paths[i]
				pathToKey[path] = key
			}

			keys := make([]ledger.Key, 0, len(pErr.Paths))
			for _, path := range pErr.Paths {
				keys = append(keys, pathToKey[path])
			}
			return nil, &ledger.ErrMissingKeys{Keys: keys}
		}
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
func (l *Ledger) Set(update *ledger.Update) (newState ledger.State, trieUpdate *ledger.TrieUpdate, err error) {
	// TODO: add test case
	if update.Size() == 0 {
		// return current state root unchanged
		return update.State(), nil, nil
	}

	trieUpdate, err = pathfinder.UpdateToTrieUpdate(update, l.pathFinderVersion)
	if err != nil {
		return ledger.DummyState, nil, err
	}

	newRootHash, err := l.ptrie.Update(trieUpdate.Paths, trieUpdate.Payloads)
	if err != nil {
		if pErr, ok := err.(*ptrie.ErrMissingPath); ok {

			paths, err := pathfinder.KeysToPaths(update.Keys(), l.pathFinderVersion)
			if err != nil {
				return ledger.DummyState, nil, err
			}

			//store mappings and restore keys from missing paths
			pathToKey := make(map[ledger.Path]ledger.Key)

			for i, key := range update.Keys() {
				path := paths[i]
				pathToKey[path] = key
			}

			keys := make([]ledger.Key, 0, len(pErr.Paths))
			for _, path := range pErr.Paths {
				keys = append(keys, pathToKey[path])
			}
			return ledger.DummyState, nil, &ledger.ErrMissingKeys{Keys: keys}
		}
		return ledger.DummyState, nil, err
	}

	// TODO log info state
	return ledger.State(newRootHash), trieUpdate, nil
}

// Prove provides proofs for a ledger query and errors (if any)
// TODO implement this by iterating over initial proofs to find the ones for the query
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {
	return nil, err
}

package partial

import (
	"fmt"

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
		return chmodels.NewCFInvalidVerifiableChunk("error constructing partial trie", err, chIndex, execResID), nil
	}

	return &Ledger{ptrie: psmt}
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
	payloads, failedPaths, err := l.ptrie.Get(paths)
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

	// TODO here update the ptrie

	// TODO log info state commitments
	return ledger.StateCommitment(newRootHash), nil
}

// Prove provides proofs for a ledger query and errors (if any)
func (l *Ledger) Prove(query *ledger.Query) (proof ledger.Proof, err error) {

	// TODO iterate over proofs to find the ones for the query
	paths, err := common.KeysToPaths(query.Keys(), 0)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(query.StateCommitment()), Paths: paths}
	batchProof, err := l.forest.Proofs(trieRead)
	if err != nil {
		return nil, fmt.Errorf("could not get proofs: %w", err)
	}

	proofToGo := common.EncodeTrieBatchProof(batchProof)

	if len(paths) > 0 {
		l.metrics.ProofSize(uint32(len(proofToGo) / len(paths)))
	}

	return ledger.Proof(proofToGo), err
}

// TODO implement an approximate MemSize method
func (l *Ledger) MemSize() (int64, error) {
	return 0, nil
}

// DiskSize returns the amount of disk space used by the storage (in bytes)
func (l *Ledger) DiskSize() (int64, error) {
	return 0, nil
}

// // chunk view construction
// // unknown register tracks access to parts of the partial trie which
// // are not expanded and values are unknown.
// unknownRegTouch := make(map[string]bool)
// regMap := vc.ChunkDataPack.GetRegisterValues()
// getRegister := func(key flow.RegisterID) (flow.RegisterValue, error) {
// 	// check if register has been provided in the chunk data pack
// 	val, ok := regMap[string(key)]
// 	if !ok {
// 		unknownRegTouch[string(key)] = true
// 		return nil, fmt.Errorf("missing register")
// 	}
// 	return val, nil
// }

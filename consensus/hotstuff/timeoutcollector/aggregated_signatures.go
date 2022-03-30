package timeoutcollector

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// signerInfo holds information about a signer, its weight and index
type signerInfo struct {
	weight uint64
	index  int
}

// sigInfo holds signature and message submitted by some signer
type sigInfo struct {
	sig crypto.Signature
	msg []byte
}

type MultiMessageSignatureAggregator struct {
	hasher           hash.Hasher
	ids              flow.IdentityList
	idToInfo         map[flow.Identifier]signerInfo // auxiliary map to lookup signer weight and index by ID (only gets updated by constructor)
	indexToSignature map[int]sigInfo                // signatures indexed by the signer index
	publicKeys       []crypto.PublicKey
	totalWeight      uint64
	lock             sync.RWMutex
}

func NewMultiMessageSigAggregator(ids flow.IdentityList, // list of all authorized signers
	pks []crypto.PublicKey, // list of corresponding public keys used for signature verifications
	dsTag string, // domain separation tag used by the signature) *MultiMessageSignatureAggregator
) (*MultiMessageSignatureAggregator, error) {

	if len(pks) == 0 {
		return nil, fmt.Errorf("number of participants must be larger than 0, got %d", len(pks))
	}
	// sanity check for BLS keys
	for i, key := range pks {
		if key == nil || key.Algorithm() != crypto.BLSBLS12381 {
			return nil, fmt.Errorf("key at index %d is not a BLS key", i)
		}
	}

	// build the internal map for a faster look-up
	idToInfo := make(map[flow.Identifier]signerInfo)
	for i, id := range ids {
		idToInfo[id.NodeID] = signerInfo{
			weight: id.Weight,
			index:  i,
		}
	}

	return &MultiMessageSignatureAggregator{
		hasher:           crypto.NewBLSKMAC(dsTag),
		ids:              ids,
		idToInfo:         idToInfo,
		indexToSignature: make(map[int]sigInfo),
		publicKeys:       pks,
	}, nil
}

func (a *MultiMessageSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature, msg []byte) error {
	info, ok := a.idToInfo[signerID]
	if !ok {
		return model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}
	valid, err := a.publicKeys[info.index].Verify(sig, msg, a.hasher)
	if err != nil {
		return fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !valid {
		return fmt.Errorf("invalid signature from %s: %w", signerID, model.ErrInvalidSignature)
	}
	return nil
}

func (a *MultiMessageSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature, msg []byte) (uint64, error) {
	info, found := a.idToInfo[signerID]
	if !found {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	if _, duplicate := a.indexToSignature[info.index]; duplicate {
		return a.totalWeight, model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	a.indexToSignature[info.index] = sigInfo{
		sig: sig,
		msg: msg,
	}
	a.totalWeight += info.weight

	return a.totalWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
// The function is thread-safe
func (a *MultiMessageSignatureAggregator) TotalWeight() uint64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.totalWeight
}

// Aggregate aggregates the signatures and returns the aggregated signature.
// The function performs a final verification and errors if the aggregated signature is not valid. This is
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
// The function errors with:
//  - model.InsufficientSignaturesError if no signatures have been added yet
//  - model.InvalidSignatureIncludedError if some signature(s), included via TrustedAdd, are invalid
// The function is thread-safe.
//
// TODO : When compacting the list of signers, update the return from []flow.Identifier
//        to a compact bit vector.
func (a *MultiMessageSignatureAggregator) Aggregate() ([]flow.Identifier, []byte, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	// compute aggregation result and cache it in `a.cachedSignerIndices`, `a.cachedSignature`
	sharesNum := len(a.indexToSignature)
	if sharesNum == 0 {
		return nil, nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures\"")
	}
	pks := make([]crypto.PublicKey, 0, sharesNum)
	messages := make([][]byte, 0, sharesNum)
	signatures := make([]crypto.Signature, 0, sharesNum)
	hashers := make([]hash.Hasher, 0, sharesNum)
	signerIDs := make([]flow.Identifier, 0, sharesNum)
	for i, info := range a.indexToSignature {
		pks = append(pks, a.publicKeys[i])
		messages = append(messages, info.msg)
		signatures = append(signatures, info.sig)
		hashers = append(hashers, a.hasher)
		signerIDs = append(signerIDs, a.ids[i].NodeID)
	}

	aggSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		// invalidInputsError for:
		//  * empty `signatures` slice, i.e. sharesNum == 0, which we exclude by earlier check
		//  * if some signature(s), included via TrustedAdd, could not be decoded
		if crypto.IsInvalidInputsError(err) {
			return nil, nil, model.NewInvalidSignatureIncludedErrorf("signatures with invalid structure were included via TrustedAdd: %w", err)
		}
		return nil, nil, fmt.Errorf("BLS signature aggregation failed: %w", err)
	}

	valid, err := crypto.VerifyBLSSignatureManyMessages(pks, aggSignature, messages, hashers)
	if err != nil {
		return nil, nil, fmt.Errorf("signature verification failed: %w", err)
	}
	if !valid {
		return nil, nil, model.NewInvalidSignatureIncludedErrorf("invalid signature(s) have been included via TrustedAdd")
	}

	return signerIDs, aggSignature, nil
}

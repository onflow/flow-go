package timeoutcollector

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// signerInfo holds information about a signer, its public key and weight
type signerInfo struct {
	pk     crypto.PublicKey
	weight uint64
}

// sigInfo holds signature and message submitted by some signer
type sigInfo struct {
	sig crypto.Signature
	msg []byte
}

// WeightedMultiMessageSignatureAggregator implements consensus/hotstuff.WeightedMultiMessageSignatureAggregator.
// It aggregates BLS signatures for many messages from different signers.
// Only public keys needs to be agreed upon upfront.
// Each signer is allowed to sign at most once.
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
// Implementation is thread-safe.
type WeightedMultiMessageSignatureAggregator struct {
	lock          sync.RWMutex
	hasher        hash.Hasher
	idToInfo      map[flow.Identifier]signerInfo // auxiliary map to lookup signer weight and public key (only gets updated by constructor)
	idToSignature map[flow.Identifier]sigInfo    // signatures indexed by the signer ID
	totalWeight   uint64                         // total accumulated weight
}

var _ hotstuff.WeightedMultiMessageSignatureAggregator = (*WeightedMultiMessageSignatureAggregator)(nil)

// NewWeightedMultiMessageSigAggregator returns a multi message signature aggregator initialized with a list of flow
// identities, their respective public keys and a domain separation tag. The identities
// represent the list of all possible signers.
// The constructor errors if:
// - the list of identities is empty
// - if the length of keys does not match the length of identities
// - if one of the keys is not a valid public key.
//
// A multi message sig aggregator is used for one aggregation only. A new instance should be used for each
// signature aggregation task in the protocol.
func NewWeightedMultiMessageSigAggregator(ids flow.IdentityList, // list of all authorized signers
	pks []crypto.PublicKey, // list of corresponding public keys used for signature verifications
	dsTag string, // domain separation tag used by the signature) *WeightedMultiMessageSignatureAggregator
) (*WeightedMultiMessageSignatureAggregator, error) {
	if len(ids) != len(pks) {
		return nil, fmt.Errorf("keys length %d and identities length %d do not match", len(pks), len(ids))
	}

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
			pk:     pks[i],
			weight: id.Weight,
		}
	}

	return &WeightedMultiMessageSignatureAggregator{
		hasher:        crypto.NewBLSKMAC(dsTag),
		idToInfo:      idToInfo,
		idToSignature: make(map[flow.Identifier]sigInfo),
	}, nil
}

// Verify verifies the signature under the stored public keys.
// Expected errors during normal operations:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
// The function is thread-safe.
func (a *WeightedMultiMessageSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature, msg []byte) error {
	info, ok := a.idToInfo[signerID]
	if !ok {
		return model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}
	valid, err := info.pk.Verify(sig, msg, a.hasher)
	if err != nil {
		return fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !valid {
		return fmt.Errorf("invalid signature from %s: %w", signerID, model.ErrInvalidSignature)
	}
	return nil
}

// TrustedAdd adds signature and message to the internal set of signatures and adds the signer's
// weight to the total collected weight, iff the signature is _not_ a duplicate.
//
// The total weight of all collected signatures (excluding duplicates) is returned regardless
// of any returned error.
// The function errors with:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.DuplicatedSignerError if the signer has been already added
// The function is thread-safe.
func (a *WeightedMultiMessageSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature, msg []byte) (uint64, error) {
	info, found := a.idToInfo[signerID]
	if !found {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	if _, duplicate := a.idToSignature[signerID]; duplicate {
		return a.totalWeight, model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	a.idToSignature[signerID] = sigInfo{
		sig: sig,
		msg: msg,
	}
	a.totalWeight += info.weight

	return a.totalWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
// The function is thread-safe
func (a *WeightedMultiMessageSignatureAggregator) TotalWeight() uint64 {
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
func (a *WeightedMultiMessageSignatureAggregator) Aggregate() ([]flow.Identifier, []byte, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	sharesNum := len(a.idToSignature)
	if sharesNum == 0 {
		return nil, nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures\"")
	}
	pks := make([]crypto.PublicKey, 0, sharesNum)
	messages := make([][]byte, 0, sharesNum)
	signatures := make([]crypto.Signature, 0, sharesNum)
	hashers := make([]hash.Hasher, 0, sharesNum)
	signerIDs := make([]flow.Identifier, 0, sharesNum)
	for id, info := range a.idToSignature {
		pks = append(pks, a.idToInfo[id].pk)
		messages = append(messages, info.msg)
		signatures = append(signatures, info.sig)
		hashers = append(hashers, a.hasher)
		signerIDs = append(signerIDs, id)
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

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

// TimeoutSignatureAggregator implements consensus/hotstuff.TimeoutSignatureAggregator.
// It aggregates BLS signatures for many messages from different signers.
// Only public keys needs to be agreed upon upfront.
// Each signer is allowed to sign at most once.
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
// Implementation is thread-safe.
type TimeoutSignatureAggregator struct {
	lock          sync.RWMutex
	hasher        hash.Hasher
	idToInfo      map[flow.Identifier]signerInfo       // auxiliary map to lookup signer weight and public key (only gets updated by constructor)
	idToSignature map[flow.Identifier]crypto.Signature // signatures indexed by the signer ID
	totalWeight   uint64                               // total accumulated weight
}

var _ hotstuff.TimeoutSignatureAggregator = (*TimeoutSignatureAggregator)(nil)

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
	dsTag string, // domain separation tag used by the signature
) (*TimeoutSignatureAggregator, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("number of participants must be larger than 0, got %d", len(ids))
	}
	// sanity check for BLS keys
	for i, identity := range ids {
		if identity.StakingPubKey.Algorithm() != crypto.BLSBLS12381 {
			return nil, fmt.Errorf("key at index %d is not a BLS key", i)
		}
	}

	// build the internal map for a faster look-up
	idToInfo := make(map[flow.Identifier]signerInfo)
	for _, id := range ids {
		idToInfo[id.NodeID] = signerInfo{
			pk:     id.StakingPubKey,
			weight: id.Weight,
		}
	}

	return &TimeoutSignatureAggregator{
		hasher:        crypto.NewBLSKMAC(dsTag),
		idToInfo:      idToInfo,
		idToSignature: make(map[flow.Identifier]crypto.Signature),
	}, nil
}

// Verify verifies the signature under the stored public keys.
// Expected errors during normal operations:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
// The function is thread-safe.
func (a *TimeoutSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature, msg []byte) error {
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
func (a *TimeoutSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature, msg []byte) (uint64, error) {
	info, found := a.idToInfo[signerID]
	if !found {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	if _, duplicate := a.idToSignature[signerID]; duplicate {
		return a.totalWeight, model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	a.idToSignature[signerID] = sig
	a.totalWeight += info.weight

	return a.totalWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
// The function is thread-safe
func (a *TimeoutSignatureAggregator) TotalWeight() uint64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.totalWeight
}

// UnsafeAggregate aggregates the signatures and returns the aggregated signature.
// The function DOES NOT perform a final verification of aggregated
// signature. This aggregated signature needs to be verified against messages that were submitted
// in `TrustedAdd`.
// Expected errors during normal operations:
//  - model.InsufficientSignaturesError if no signatures have been added yet
// This function is thread-safe
//
func (a *TimeoutSignatureAggregator) UnsafeAggregate() ([]flow.Identifier, []byte, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	sharesNum := len(a.idToSignature)
	if sharesNum == 0 {
		return nil, nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures")
	}
	signatures := make([]crypto.Signature, 0, sharesNum)
	signerIDs := make([]flow.Identifier, 0, sharesNum)
	for id, sig := range a.idToSignature {
		signatures = append(signatures, sig)
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

	return signerIDs, aggSignature, nil
}

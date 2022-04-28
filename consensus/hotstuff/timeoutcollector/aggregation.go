package timeoutcollector

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
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

// sigInfo holds signature and high QC view submitted by some signer
type sigInfo struct {
	sig        crypto.Signature
	highQCView uint64
}

// TimeoutSignatureAggregator implements consensus/hotstuff.TimeoutSignatureAggregator.
// It performs timeout specific BLS aggregation over multiple distinct messages.
// We perform timeout signature aggregation for some concrete view, because of that our messages
// are build in next way: hash(view, highestQCView), where `highestQCView` is a value contributed
// by different replicas.
// View and public keys needs to be agreed upon upfront.
// Each signer is allowed to sign at most once.
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP)
// This module does not verify PoPs of input public keys, it assumes verification was done outside this module.
// Implementation is thread-safe.
type TimeoutSignatureAggregator struct {
	lock          sync.RWMutex
	hasher        hash.Hasher
	idToInfo      map[flow.Identifier]signerInfo // auxiliary map to lookup signer weight and public key (only gets updated by constructor)
	idToSignature map[flow.Identifier]sigInfo    // signatures indexed by the signer ID
	totalWeight   uint64                         // total accumulated weight
	view          uint64                         // view for which we are aggregating signatures
}

var _ hotstuff.TimeoutSignatureAggregator = (*TimeoutSignatureAggregator)(nil)

// NewTimeoutSignatureAggregator returns a multi message signature aggregator initialized with a predefined view
// for which we aggregate signatures, list of flow identities,
// their respective public keys and a domain separation tag. The identities
// represent the list of all possible signers.
// The constructor errors if:
// - the list of identities is empty
// - if one of the keys is not a valid public key.
//
// A multi message sig aggregator is used for one aggregation only. A new instance should be used for each
// signature aggregation task in the protocol.
func NewTimeoutSignatureAggregator(
	view uint64, // view for which we are aggregating signatures
	ids flow.IdentityList, // list of all authorized signers
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
		idToSignature: make(map[flow.Identifier]sigInfo),
		view:          view,
	}, nil
}

// VerifyAndAdd verifies the signature under the stored public keys and adds signature with corresponding
// highest QC view to the internal set. Internal set and collected weight is modified iff the signer ID is not a duplicate and signature _is_ valid.
// The total weight of all collected signatures (excluding duplicates) is returned regardless
// of any returned error.
// Expected errors during normal operations:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.DuplicatedSignerError if the signer has been already added
//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
// The function is thread-safe.
func (a *TimeoutSignatureAggregator) VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, highestQCView uint64) (totalWeight uint64, exception error) {
	info, ok := a.idToInfo[signerID]
	if !ok {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	msg := verification.MakeTimeoutMessage(a.view, highestQCView)
	valid, err := info.pk.Verify(sig, msg, a.hasher)
	if err != nil {
		return a.TotalWeight(), fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !valid {
		return a.TotalWeight(), fmt.Errorf("invalid signature from %s: %w", signerID, model.ErrInvalidSignature)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	if _, duplicate := a.idToSignature[signerID]; duplicate {
		return a.totalWeight, model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	a.idToSignature[signerID] = sigInfo{
		sig:        sig,
		highQCView: highestQCView,
	}
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

// Aggregate aggregates the signatures and returns the aggregated signature.
// The function performs a final verification of aggregated
// signature. Caller can be sure that resulting signature is valid.
// Expected errors during normal operations:
//  - model.InsufficientSignaturesError if no signatures have been added yet
// This function is thread-safe
//
func (a *TimeoutSignatureAggregator) Aggregate() ([]flow.Identifier, []uint64, crypto.Signature, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	sharesNum := len(a.idToSignature)
	if sharesNum == 0 {
		return nil, nil, nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures")
	}
	pks := make([]crypto.PublicKey, 0, sharesNum)
	signatures := make([]crypto.Signature, 0, sharesNum)
	hashers := make([]hash.Hasher, 0, sharesNum)
	signers := make([]flow.Identifier, 0, sharesNum)
	messages := make([][]byte, 0, sharesNum)
	highQCViews := make([]uint64, 0, sharesNum)
	for id, info := range a.idToSignature {
		msg := verification.MakeTimeoutMessage(a.view, info.highQCView)
		signatures = append(signatures, info.sig)
		signers = append(signers, id)
		messages = append(messages, msg)
		pks = append(pks, a.idToInfo[id].pk)
		hashers = append(hashers, a.hasher)
		highQCViews = append(highQCViews, info.highQCView)
	}

	aggSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		// invalidInputsError for:
		//  * empty `signatures` slice, i.e. sharesNum == 0, which we exclude by earlier check
		//  * if some signature(s), included via TrustedAdd, could not be decoded
		if crypto.IsInvalidInputsError(err) {
			return nil, nil, nil, model.NewInvalidSignatureIncludedErrorf("signatures with invalid structure were included via TrustedAdd: %w", err)
		}
		return nil, nil, nil, fmt.Errorf("BLS signature aggregation failed: %w", err)
	}

	valid, err := crypto.VerifyBLSSignatureManyMessages(pks, aggSignature, messages, hashers)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("signature verification failed: %w", err)
	}
	// since we add only verified signatures, then we MUST have a valid signature at this point
	if !valid {
		return nil, nil, nil, fmt.Errorf("constructed invalid BLS signature")
	}

	return signers, highQCViews, aggSignature, nil
}

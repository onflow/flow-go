package timeoutcollector

import (
	"fmt"
	"sync"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

// signerInfo holds information about a signer, its public key and weight
type signerInfo struct {
	pk     crypto.PublicKey
	weight uint64
}

// sigInfo holds signature and high QC view submitted by some signer
type sigInfo struct {
	sig          crypto.Signature
	newestQCView uint64
}

// TimeoutSignatureAggregator implements consensus/hotstuff.TimeoutSignatureAggregator.
// It performs timeout specific BLS aggregation over multiple distinct messages.
// We perform timeout signature aggregation for some concrete view, utilizing the protocol specification
// that timeouts sign the message: hash(view, newestQCView), where newestQCView can have different values
// for different replicas.
// View and the identities of all authorized replicas are
// specified when the TimeoutSignatureAggregator is instantiated.
// Each signer is allowed to sign at most once.
// Aggregation uses BLS scheme. Mitigation against rogue attacks is done using Proof Of Possession (PoP).
// Implementation is only safe under the assumption that all proofs of possession (PoP) of the public keys
// are valid. This module does not perform the PoPs validity checks, it assumes verification was done
// outside the module.
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
// represent the list of all authorized signers.
// The constructor does not verify PoPs of input public keys, it assumes verification was done outside
// this module.
// The constructor errors if:
// - the list of identities is empty
// - if one of the keys is not a valid public key.
//
// A multi message sig aggregator is used for aggregating timeouts for a single view only. A new instance should be used for each
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
		hasher:        msig.NewBLSHasher(dsTag), // concurrency safe
		idToInfo:      idToInfo,
		idToSignature: make(map[flow.Identifier]sigInfo),
		view:          view,
	}, nil
}

// VerifyAndAdd verifies the signature under the stored public keys and adds signature with corresponding
// newest QC view to the internal set. Internal set and collected weight is modified iff the signer ID is not a duplicate and signature _is_ valid.
// The total weight of all collected signatures (excluding duplicates) is returned regardless
// of any returned error.
// Expected errors during normal operations:
//   - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//   - model.DuplicatedSignerError if the signer has been already added
//   - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
//
// The function is thread-safe.
func (a *TimeoutSignatureAggregator) VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, newestQCView uint64) (totalWeight uint64, exception error) {
	info, ok := a.idToInfo[signerID]
	if !ok {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	// to avoid expensive signature verification we will proceed with double lock style check
	if a.hasSignature(signerID) {
		return a.TotalWeight(), model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	msg := verification.MakeTimeoutMessage(a.view, newestQCView)
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
		sig:          sig,
		newestQCView: newestQCView,
	}
	a.totalWeight += info.weight

	return a.totalWeight, nil
}

func (a *TimeoutSignatureAggregator) hasSignature(singerID flow.Identifier) bool {
	a.lock.RLock()
	defer a.lock.RUnlock()
	_, found := a.idToSignature[singerID]
	return found
}

// TotalWeight returns the total weight presented by the collected signatures.
// The function is thread-safe
func (a *TimeoutSignatureAggregator) TotalWeight() uint64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	return a.totalWeight
}

// View returns view for which aggregation happens
// The function is thread-safe
func (a *TimeoutSignatureAggregator) View() uint64 {
	return a.view
}

// Aggregate aggregates the signatures and returns the aggregated signature.
// The resulting aggregated signature is guaranteed to be valid, as all individual
// signatures are pre-validated before their addition.
// Expected errors during normal operations:
//   - model.InsufficientSignaturesError if no signatures have been added yet
//
// This function is thread-safe
func (a *TimeoutSignatureAggregator) Aggregate() ([]hotstuff.TimeoutSignerInfo, crypto.Signature, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	sharesNum := len(a.idToSignature)
	signatures := make([]crypto.Signature, 0, sharesNum)
	signersData := make([]hotstuff.TimeoutSignerInfo, 0, sharesNum)
	for id, info := range a.idToSignature {
		signatures = append(signatures, info.sig)
		signersData = append(signersData, hotstuff.TimeoutSignerInfo{
			NewestQCView: info.newestQCView,
			Signer:       id,
		})
	}

	aggSignature, err := crypto.AggregateBLSSignatures(signatures)
	if err != nil {
		// `AggregateBLSSignatures` returns two possible errors:
		//  - crypto.BLSAggregateEmptyListError if `signatures` slice is empty, i.e no signatures have been added yet:
		//    respond with model.InsufficientSignaturesError
		//  - crypto.invalidSignatureError if some signature(s) could not be decoded, which should be impossible since
		//    we check all signatures before adding them (there is no `TrustedAdd` method in this module)
		if crypto.IsBLSAggregateEmptyListError(err) {
			return nil, nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures: %w", err)
		}
		// any other error here is a symptom of an internal bug
		return nil, nil, fmt.Errorf("unexpected internal error during BLS signature aggregation: %w", err)
	}

	// TODO-1: add logic to check if only one `NewestQCView` is used. In that case
	// check the aggregated signature is not identity (that's enough to ensure
	// aggregated key is not identity, given all signatures are individually valid)
	// This is not implemented for now because `VerifyTC` does not error for an identity public key
	// (that's because the crypto layer currently does not return false when verifying signatures using `VerifyBLSSignatureManyMessages`
	//  and encountering identity public keys)
	//
	// TODO-2: check if the logic should be extended to look at the partial aggregated signatures of all
	// signatures against the same message.
	return signersData, aggSignature, nil
}

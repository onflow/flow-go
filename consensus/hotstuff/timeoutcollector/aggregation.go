package timeoutcollector

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
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
// We perform timeout signature aggregation for some concrete view, utilizing the protocol specification
// that timeouts sign the message: hash(view, highestQC.View), where `highestQC` can have different values
// for different replicas.
// View and the identities of all authorized replicas are 
// specified when the TimeoutSignatureAggregator is instantiated.
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
	highestQC     *flow.QuorumCertificate        // highest QC submitted via VerifyAndAdd
}

var _ hotstuff.TimeoutSignatureAggregator = (*TimeoutSignatureAggregator)(nil)

// NewTimeoutSignatureAggregator returns a multi message signature aggregator initialized with a predefined view
// for which we aggregate signatures, list of flow identities,
// their respective public keys and a domain separation tag. The identities
// represent the list of all authorized signers.
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
		hasher:        crypto.NewBLSKMAC(dsTag), // concurrency safe
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
func (a *TimeoutSignatureAggregator) VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, highestQC *flow.QuorumCertificate) (totalWeight uint64, exception error) {
	info, ok := a.idToInfo[signerID]
	if !ok {
		return a.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	// to avoid expensive signature verification we will proceed with double lock style check
	if a.hasSignature(signerID) {
		return a.TotalWeight(), model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	msg := verification.MakeTimeoutMessage(a.view, highestQC.View)
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
		highQCView: highestQC.View,
	}
	a.totalWeight += info.weight

	if a.highestQC == nil || a.highestQC.View < highestQC.View {
		a.highestQC = highestQC
	}

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

// Aggregate aggregates the signatures and returns the aggregated signature.
// The function performs a final verification of aggregated
// signature. Caller can be sure that resulting signature is valid.
// Expected errors during normal operations:
//  - model.InsufficientSignaturesError if no signatures have been added yet
// This function is thread-safe
//
func (a *TimeoutSignatureAggregator) Aggregate() (*flow.TimeoutCertificate, error) {
	a.lock.RLock()
	defer a.lock.RUnlock()

	sharesNum := len(a.idToSignature)
	if sharesNum == 0 {
		return nil, model.NewInsufficientSignaturesErrorf("cannot aggregate an empty list of signatures")
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
		//  * if some signature(s) could not be decoded, which should be impossible since we check all signatures before adding them
		// Hence, any error here is a symptom of an internal bug
		return nil, fmt.Errorf("unexpected internal error during BLS signature aggregation: %w", err)
	}


	return &flow.TimeoutCertificate{
		View:          a.view,
		TOHighQCViews: highQCViews,
		TOHighestQC:   a.highestQC,
		SignerIDs:     signers,
		SigData:       aggSignature,
	}, nil
}

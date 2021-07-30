package votecollector

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// SigType is the aggregable signature type.
type SigType int

// There are two signatures are aggregable, one is the normal staking signature,
// the other is the threshold sig used as staking sigature.
const (
	SigTypeStaking SigType = iota
	SigTypeThreshold
	SigTypeInvalid
)

// CombinedAggregator aggregates signatures from different signers for the same message.
// The instance must be initialized with the message, the required weight to be
// sufficient for aggregation, as well as the weight for each signer.
type CombinedAggregator interface {
	// Verify a vote's signature
	Verify(signerID flow.Identifier, sig crypto.Signature) (bool, SigType, error)

	// TrustedAdd adds a verified signature, and returns whether if has collected enough signature shares
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature, sigType SigType) (bool, error)

	// VerifyAndAdd verifies the signature, if the signature is valid, then it will be added
	// the first return value is whether the sig is valid
	// the second return value is whether the sig is added
	// (false, false, nil) means the signature is invalid and is not added to the aggregator
	// (true, true, nil) means the signature is valid and added to the aggregator
	// (true, false, nil) means the signature is valid and not added to the aggregator
	VerifyAndAdd(signerID flow.Identifier, sig crypto.Signature, sigType SigType) (bool, bool, error)

	// HasSufficientWeight returns whether the signature aggregator has stored enough signature shares.
	HasSufficientWeight() bool

	// AggregateSignature assumes enough signature shares have been collected, and returns aggregated signatures
	// The first returned aggregated signature is the staking signature;
	// The second returned aggregated signature is the threshold signature;
	// For consensus cluster, the second return aggregated signature is the aggregated random beacon sig shares.
	// For collection cluster, the second returned aggregated signature will always be nil.
	AggregateSignature() (flow.AggregatedSignature, flow.AggregatedSignature, error)
}

type RandomBeaconReconstructor interface {
	// Verify returns true if and only if the signature is valid.
	Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error)

	// TrustedAdd adds the signature share to the reconstructors internal
	// state. Validity of signature is not checked. It is up to the
	// implementation, whether it still adds a signature or not, when the
	// minimal number of required sig shares has already been reached,
	// because the reconstructed group signature is the same.
	// Returns: true if and only if enough signature shares were collected
	TrustedAdd(signerIndex uint, sigShare crypto.Signature) (bool, error)

	// HasSufficientShares returns true if and only if reconstructor
	// has collected a sufficient number of signature shares.
	HasSufficientShares() bool

	// Reconstruct reconstructs the group signature from the provided
	// signature shares. Errors if the the number of shares is insufficient
	// or some of the added signatures shares were invalid.
	Reconstruct() (crypto.Signature, error)
}

// QCBuilder is responsible for creating votes, proposals and QC's for a given block.
type QCBuilder interface {
	// CreateQC creates a QC for the given block.
	CreateQC(stakingSig, thresholdSig flow.AggregatedSignature, beaconSig crypto.Signature) (*flow.QuorumCertificate, error)
}

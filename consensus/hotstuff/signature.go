package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type RandomBeaconReconstructor interface {
	// Add a share
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)
	HasSufficientShares() bool
	// assume it has sufficient shares
	Reconstruct() (crypto.Signature, error)
}

// RandomBeaconAggregtor aggregates the random beacon signatures
type RandomBeaconAggregtor interface {
	// TrustedAdd adds an already verified signature.
	// return (true, nil) means the signature has been added
	// return (false, nil) means the signature is a duplication
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)
	// Aggregate assumes enough shares have been collected, it aggregates the signatures
	// and return the aggregated signature.
	// if called concurrently, only one thread will be running the aggregation.
	Aggregate() ([]byte, error)
}

// StakingSigAggregator aggregates the staking signatures
type StakingSigAggregator interface {
	// TrustedAdd adds an already verified signature.
	// return (true, nil) means the signature has been added
	// return (false, nil) means the signature is a duplication
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)

	// Aggregate assumes enough shares have been collected, it aggregates the signatures
	// and return the aggregated signature.
	// if called concurrently, only one thread will be running the aggregation.
	Aggregate() ([]byte, error)
}

type AggregatedSignatureData struct {
	StakingSigners               []flow.Identifier
	ThresholdSigners             []flow.Identifier
	AggregatedStakingSig         crypto.Signature
	AggregatedRandomBeaconSig    crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

type Packer interface {
	Combine(sig *AggregatedSignatureData) ([]flow.Identifier, []byte, error)

	Split(signerIDs []flow.Identifier, sigData []byte) (*AggregatedSignatureData, error)
}

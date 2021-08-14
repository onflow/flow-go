package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type RandomBeaconReconstructor interface {
	// Add a beacon share without verifying it. 
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)
	HasSufficientShares() bool
	// Reconstruct the beacon signature if there are enough shares. It errors if not enough shares were collected.
	Reconstruct() (crypto.Signature, error)
}

// RandomBeaconAggregator aggregates the random beacon signatures
type RandomBeaconAggregator interface {
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
	RandomBeaconSigners             []flow.Identifier
	AggregatedStakingSig         crypto.Signature
	AggregatedRandomBeaconSig    crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

type Packer interface {
	Combine(sig *AggregatedSignatureData) ([]flow.Identifier, []byte, error)

	Split(signerIDs []flow.Identifier, sigData []byte) (*AggregatedSignatureData, error)
}

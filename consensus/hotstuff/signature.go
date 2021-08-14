package hotstuff

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// RandomBeaconReconstructor collects verified signature shares, and reconstructs the
// group signature with enough shares.
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

// SignatureAggregator aggregates the verified signatures.
// It can either be used to aggregate staking signatures or aggregate random beacon signatures,
// but not a mix of staking signature and random beacon signature.
type SignatureAggregator interface {
	// TrustedAdd adds an already verified signature.
	// return (true, nil) means the signature has been added
	// return (false, nil) means the signature is a duplication
	TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (bool, error)

	// Aggregate assumes enough shares have been collected, it aggregates the signatures
	// and return the aggregated signature.
	// if called concurrently, only one thread will be running the aggregation.
	Aggregate() ([]byte, error)
}

// AggregatedSignatureData is an intermediate struct for Packer to pack the
// aggregated signature data into raw bytes or unpack from raw bytes.
type AggregatedSignatureData struct {
	StakingSigners               []flow.Identifier
	RandomBeaconSigners          []flow.Identifier
	AggregatedStakingSig         crypto.Signature
	AggregatedRandomBeaconSig    crypto.Signature
	ReconstructedRandomBeaconSig crypto.Signature
}

// Packer packs aggregated signature data into raw bytes to be used in block header.
type Packer interface {
	Pack(sig *AggregatedSignatureData) ([]flow.Identifier, []byte, error)

	Unpack(signerIDs []flow.Identifier, sigData []byte) (*AggregatedSignatureData, error)
}

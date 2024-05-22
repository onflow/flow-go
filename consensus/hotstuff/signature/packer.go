package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// ConsensusSigDataPacker implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigDataPacker struct {
	model.SigDataPacker
	committees hotstuff.Replicas
}

var _ hotstuff.Packer = &ConsensusSigDataPacker{}

// NewConsensusSigDataPacker creates a new ConsensusSigDataPacker instance
func NewConsensusSigDataPacker(committees hotstuff.Replicas) *ConsensusSigDataPacker {
	return &ConsensusSigDataPacker{
		committees: committees,
	}
}

// Pack serializes the block signature data into raw bytes, suitable to create a QC.
// To pack the block signature data, we first build a compact data type, and then encode it into bytes.
// Expected error returns during normal operations:
//   - none; all errors are symptoms of inconsistent input data or corrupted internal state.
func (p *ConsensusSigDataPacker) Pack(view uint64, sig *hotstuff.BlockSignatureData) ([]byte, []byte, error) {
	// retrieve all authorized consensus participants at the given block
	fullMembers, err := p.committees.IdentitiesByEpoch(view)
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committee for view %d: %w", view, err)
	}

	// breaking staking and random beacon signers into signerIDs and sig type for compaction
	// each signer must have its signerID and sig type stored at the same index in the two slices
	// For v2, RandomBeaconSigners is nil, as we don't track individually which nodes contributed to the random beacon
	// For v3, RandomBeaconSigners is not nil, each RandomBeaconSigner also signed staking sig, so the returned signerIDs, should
	// include both StakingSigners and RandomBeaconSigners
	signerIndices, sigType, err := signature.EncodeSignerToIndicesAndSigType(fullMembers.NodeIDs(), sig.StakingSigners, sig.RandomBeaconSigners)
	if err != nil {
		return nil, nil, fmt.Errorf("unexpected internal error while encoding signer indices and sig types: %w", err)
	}

	data := model.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    sig.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: sig.ReconstructedRandomBeaconSig,
	}

	// encode the structured data into raw bytes
	encoded, err := p.Encode(&data)
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode data %v, %w", data, err)
	}

	return signerIndices, encoded, nil
}

// Unpack de-serializes the provided signature data.
// view is the view of the block that the aggregated sig is signed for
// sig is the aggregated signature data
// It returns:
//   - (sigData, nil) if successfully unpacked the signature data
//   - (nil, model.InvalidFormatError) if failed to unpack the signature data
func (p *ConsensusSigDataPacker) Unpack(signerIdentities flow.IdentitySkeletonList, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	// decode into typed data
	data, err := p.Decode(sigData) // all potential error are of type `model.InvalidFormatError`
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %w", err)
	}

	stakingSigners, randomBeaconSigners, err := signature.DecodeSigTypeToStakingAndBeaconSigners(signerIdentities, data.SigType)
	if err != nil {
		if errors.Is(err, signature.ErrIllegallyPaddedBitVector) || errors.Is(err, signature.ErrIncompatibleBitVectorLength) {
			return nil, model.NewInvalidFormatErrorf("invalid SigType vector: %w", err)
		}
		return nil, fmt.Errorf("could not decode signer indices and sig type: %w", err)
	}

	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners.NodeIDs(),
		RandomBeaconSigners:          randomBeaconSigners.NodeIDs(),
		AggregatedStakingSig:         data.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    data.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: data.ReconstructedRandomBeaconSig,
	}, nil
}

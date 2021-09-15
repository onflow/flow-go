package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/signature"
)

// ConsensusSigPackerImpl implements the hotstuff.Packer interface.
// The encoding method is RLP.
type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
	encoder    *rlp.Encoder // rlp encoder is used in order to ensure deterministic encoding
}

var _ hotstuff.Packer = &ConsensusSigPackerImpl{}

func NewConsensusSigPackerImpl(committees hotstuff.Committee) *ConsensusSigPackerImpl {
	return &ConsensusSigPackerImpl{
		committees: committees,
		encoder:    rlp.NewEncoder(),
	}
}

// signatureData is a compact data type for encoding the block signature data
type signatureData struct {
	// bit-vector indicating type of signature for each signer.
	// the order of each sig type matches the order of corresponding signer IDs
	SigType []hotstuff.SigType

	AggregatedStakingSig      []byte
	AggregatedRandomBeaconSig []byte
	RandomBeacon              crypto.Signature
}

// Pack serializes the block signature data into raw bytes, suitable to creat a QC.
// To pack the block signature data, we first build a compact data type, and then encode it into bytes.
// Expected error returns during normal operations:
//  * none; all errors are symptoms of inconsistent input data or corrupted internal state.
func (p *ConsensusSigPackerImpl) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	consensus, err := p.committees.Identities(blockID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	// lookup is a map from node identifier to node identity
	lookup := consensus.Lookup()
	count := len(sig.StakingSigners) + len(sig.RandomBeaconSigners)

	signerIDs := make([]flow.Identifier, 0, count)
	// the order of sig types matches the signer IDs so that it indicates
	// which type of signature each signer produces.
	sigTypes := make([]hotstuff.SigType, 0, count)

	for _, stakingSigner := range sig.StakingSigners {
		_, ok := lookup[stakingSigner]
		if ok {
			signerIDs = append(signerIDs, stakingSigner)
			sigTypes = append(sigTypes, hotstuff.SigTypeStaking)
		} else {
			return nil, nil, fmt.Errorf("staking signer ID (%v) not found in the committees at block: %v", stakingSigner, blockID)
		}
	}

	for _, beaconSigner := range sig.RandomBeaconSigners {
		_, ok := lookup[beaconSigner]
		if ok {
			signerIDs = append(signerIDs, beaconSigner)
			sigTypes = append(sigTypes, hotstuff.SigTypeRandomBeacon)
		} else {
			return nil, nil, fmt.Errorf("random beacon signer ID (%v) not found in the committees at block: %v", beaconSigner, blockID)
		}
	}

	data := signatureData{
		SigType:                   sigTypes,
		AggregatedStakingSig:      sig.AggregatedStakingSig,
		AggregatedRandomBeaconSig: sig.AggregatedRandomBeaconSig,
		RandomBeacon:              sig.ReconstructedRandomBeaconSig,
	}

	encoded, err := p.encoder.Encode(data)
	if err != nil {
		return nil, nil, fmt.Errorf("could not encode data %v, %w", data, err)
	}

	return signerIDs, encoded, nil
}

// Unpack de-serializes the provided signature data.
// blockID is the block that the aggregated sig is signed for
// sig is the aggregated signature data
// It returns:
//  - (sigData, nil) if successfully unpacked the signature data
//  - (nil, signature.ErrInvalidFormat) if failed to unpack the signature data
func (p *ConsensusSigPackerImpl) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	var data signatureData
	err := p.encoder.Decode(sigData, &data)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data %s: %w", err, signature.ErrInvalidFormat)
	}

	stakingSigners := make([]flow.Identifier, 0)
	randomBeaconSigners := make([]flow.Identifier, 0)

	if len(data.SigType) != len(signerIDs) {
		return nil, fmt.Errorf("unmatched number of signerIDs, %v != %v: %w", len(data.SigType), len(signerIDs), signature.ErrInvalidFormat)
	}

	for i, sigType := range data.SigType {
		if sigType == hotstuff.SigTypeStaking {
			stakingSigners = append(stakingSigners, signerIDs[i])
		} else if sigType == hotstuff.SigTypeRandomBeacon {
			randomBeaconSigners = append(randomBeaconSigners, signerIDs[i])
		} else {
			return nil, fmt.Errorf("unknown sigType %v, %w", sigType, signature.ErrInvalidFormat)
		}
	}

	return &hotstuff.BlockSignatureData{
		StakingSigners:               stakingSigners,
		RandomBeaconSigners:          randomBeaconSigners,
		AggregatedStakingSig:         data.AggregatedStakingSig,
		AggregatedRandomBeaconSig:    data.AggregatedRandomBeaconSig,
		ReconstructedRandomBeaconSig: data.RandomBeacon,
	}, nil
}

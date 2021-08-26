package signature

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encoding/rlp"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
	encoder    *rlp.Encoder
}

var _ hotstuff.Packer = &ConsensusSigPackerImpl{}

func NewConsensusSigPackerImpl(committees hotstuff.Committee) *ConsensusSigPackerImpl {
	return &ConsensusSigPackerImpl{
		committees: committees,
		encoder:    rlp.NewEncoder(),
	}
}

type signatureData struct {
	SigType                   []byte // bit-vector indicating type of signature
	AggregatedStakingSig      crypto.Signature
	AggregatedRandomBeaconSig crypto.Signature
	RandomBeacon              crypto.Signature
}

func (p *ConsensusSigPackerImpl) Pack(blockID flow.Identifier, sig *hotstuff.BlockSignatureData) ([]flow.Identifier, []byte, error) {
	consensus, err := p.committees.Identities(blockID, filter.HasRole(flow.RoleConsensus))
	if err != nil {
		return nil, nil, fmt.Errorf("could not find consensus committees by block id(%v): %w", blockID, err)
	}

	lookup := consensus.Lookup()
	count := len(sig.StakingSigners) + len(sig.RandomBeaconSigners)

	sigTypes := make([]byte, 0, count)
	signerIDs := make([]flow.Identifier, 0, count)

	for _, stakingSigner := range sig.StakingSigners {
		_, ok := lookup[stakingSigner]
		if ok {
			sigTypes = append(sigTypes, byte(hotstuff.SigTypeStaking))
			signerIDs = append(signerIDs, stakingSigner)
		} else {
			return nil, nil, fmt.Errorf("staking signer ID (%v) not found in the committees at block: %v", stakingSigner, blockID)
		}
	}

	for _, beaconSigner := range sig.RandomBeaconSigners {
		_, ok := lookup[beaconSigner]
		if ok {
			sigTypes = append(sigTypes, byte(hotstuff.SigTypeRandomBeacon))
			signerIDs = append(signerIDs, beaconSigner)
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

func (p *ConsensusSigPackerImpl) Unpack(blockID flow.Identifier, signerIDs []flow.Identifier, sigData []byte) (*hotstuff.BlockSignatureData, error) {
	var data signatureData
	err := p.encoder.Decode(sigData, &sigData)
	if err != nil {
		return nil, fmt.Errorf("could not decode sig data: %w", err)
	}

	stakingSigners := make([]flow.Identifier, 0)
	randomBeaconSigners := make([]flow.Identifier, 0)

	if len(data.SigType) != len(signerIDs) {
		return nil, fmt.Errorf("unmatched number of signerIDs, %v != %v", len(data.SigType), len(signerIDs))
	}

	for i, sigType := range data.SigType {
		if sigType == byte(hotstuff.SigTypeStaking) {
			stakingSigners = append(stakingSigners, signerIDs[i])
		} else if sigType == byte(hotstuff.SigTypeRandomBeacon) {
			randomBeaconSigners = append(randomBeaconSigners, signerIDs[i])
		} else {
			return nil, fmt.Errorf("unknown sigType: %v", sigType)
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

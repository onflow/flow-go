package signature

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	hotmodel "github.com/dapperlabs/flow-go/model/hotstuff"
)

// RandomBeaconAwareSigProvider is an implementation of the SigVerifier interface which implements
// proper verification of random-beacon signatures.
// This implementation of SigVerifier is intended for use full consensus nodes.
type RandomBeaconAwareSigProvider struct {
	StakingSigVerifier
	RandomBeaconSigVerifier
}

func NewRandomBeaconAwareSigProvider(stakingSigTag string) hotstuff.SigVerifier {
	return &RandomBeaconAwareSigProvider{
		StakingSigVerifier:      NewStakingSigVerifier(stakingSigTag),
		RandomBeaconSigVerifier: NewRandomBeaconSigVerifier(),
	}
}

// StakingSigProvider is an implementation of the SigVerifier interface which is OBLIVIOUS to RANDOM BEACON.
// Verification of any Random-Beacon related signatures will _always_ succeed.
// This implementation of SigVerifier is intended for use in the collector's cluster-internal consensus.
type StakingSigProvider struct {
	StakingSigVerifier
}

func NewStakingSigProvider(stakingSigTag string) hotstuff.SigVerifier {
	return &StakingSigProvider{
		StakingSigVerifier: NewStakingSigVerifier(stakingSigTag),
	}
}

// VerifyRandomBeaconSig returns always true
func (s *StakingSigProvider) VerifyRandomBeaconSig(sigShare crypto.Signature, block *hotmodel.Block, signerPubKey crypto.PublicKey) (bool, error) {
	return true, nil
}

// VerifyAggregatedRandomBeaconSignature returns always true
func (s *StakingSigProvider) VerifyAggregatedRandomBeaconSignature(sig crypto.Signature, block *hotmodel.Block, groupPubKey crypto.PublicKey) (bool, error) {
	return true, nil
}

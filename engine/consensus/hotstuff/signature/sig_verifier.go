package signature

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/model/encoding"
)

// RandomBeaconAwareSigVerifier is an implementation of the SigVerifier interface with
// proper verification of random-beacon signatures. This implementation of SigVerifier
// is intended for use byf hotstuff followers to validate the consensus signatures
type RandomBeaconAwareSigVerifier struct {
	StakingSigVerifier
	RandomBeaconSigVerifier
}

func NewRandomBeaconAwareSigVerifier() hotstuff.SigVerifier {
	return &RandomBeaconAwareSigVerifier{
		StakingSigVerifier:      NewStakingSigVerifier(encoding.ConsensusVoteTag),
		RandomBeaconSigVerifier: NewRandomBeaconSigVerifier(),
	}
}

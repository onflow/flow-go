package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusSigPackerImpl struct {
	committees hotstuff.Committee
}

func (p *ConsensusSigPackerImpl) Combine(
	stakingSigners []flow.Identifier,
	thresholdSigners []flow.Identifier,
	aggregatedStakingSig crypto.Signature,
	aggregatedThresholdSig crypto.Signature,
	reconstructedThresholdSig crypto.Signature,
) ([]flow.Identifier, []byte, error) {
	panic("to be implemented")
}

func (p *ConsensusSigPackerImpl) Split(signerIDs []flow.Identifier, sigData []byte) (
	[]flow.Identifier, // staking signers
	[]flow.Identifier, // threshold signers
	crypto.Signature, // aggregated staking sig
	crypto.Signature, // aggregated threshold sig
	crypto.Signature, // reconstructed threshold sig
	error) {
	panic("to be implemented")
}

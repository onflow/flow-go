// +build relic

package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	model "github.com/dapperlabs/flow-go/model/hotstuff"
	"github.com/dapperlabs/flow-go/protocol"
)

// StakingSigner provides symmetry functions to generate and verify signatures
type StakingSigner struct {
	StakingSigVerifier
	myID              flow.Identifier
	protocolState     protocol.State
	stakingPrivateKey crypto.PrivateKey // private staking key
}

// NewStakingSigner creates an instance of StakingSigner
func NewStakingSigner(
	myID flow.Identifier,
	protocolState protocol.State,
	stakingSigTag string,
	stakingPrivateKey crypto.PrivateKey,
) *StakingSigner {
	return &StakingSigner{
		StakingSigVerifier: NewStakingSigVerifier(stakingSigTag),
		myID:               myID,
		protocolState:      protocolState,
		stakingPrivateKey:  stakingPrivateKey,
	}
}

// Aggregate aggregates the given signature that signed on the given block
// block - it is needed in order to double check the reconstruct signature is valid
// And verifying the sig requires the signed message, which is the block
// sigs - the signatures to be aggregated. Assuming each signature has been verified already.
func (s *StakingSigner) Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error) {
	if len(sigs) == 0 { // ensure that sigs is not empty
		return nil, fmt.Errorf("cannot aggregate an empty slice of signatures")
	}

	// aggregate staking sigs
	aggStakingSigs, signerIDs := aggregateStakingSignature(sigs)
	ok, err := s.VerifyAggregatedStakingSignature(aggStakingSigs, block, signerIDs) // sanity check
	aggsig := model.AggregatedSignature{
		StakingSignatures:     aggStakingSigs,
		RandomBeaconSignature: nil,
		SignerIDs:             signerIDs,
	}

	return &aggsig, nil
}

func aggregateStakingSignature(sigs []*model.SingleSignature) ([]crypto.Signature, []flow.Identifier) {
	// This implementation is a naive way of aggregation the signatures. It will work, with
	// the downside of costing more bandwidth.
	// The more optimal way, which is the real aggregation, will be implemented when the crypto
	// API is available.
	aggsig := make([]crypto.Signature, len(sigs))
	for i, sig := range sigs {
		aggsig[i] = sig.StakingSignature
	}

	// pick signer IDs from signatures
	signerIDs := make([]flow.Identifier, len(sigs))
	for i, sig := range sigs {
		signerIDs[i] = sig.SignerID
	}

	return aggsig, signerIDs
}

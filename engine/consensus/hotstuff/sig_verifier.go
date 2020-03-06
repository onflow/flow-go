package hotstuff

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigVerifier verifies signatures
type SigVerifier interface {

	// VerifyStakingSig verifies a single BLS staking signature for a block using the signer's public key.
	// Inputs:
	//    stakingSig - the signature from node
	//    block - the signed block
	//    signerKey - the signer's public staking key
	VerifyStakingSig(stakingSig crypto.Signature, block *hotstuff.Block, signerKey crypto.PublicKey) (bool, error)

	// VerifyRandomBeaconSig verifies a random beacon signature share for a block using the signer's public key.
	// Inputs:
	//    sigShare - the signature share (from individual random beacon member) to be verified
	//    block - the signed block
	//    signerPubKey - the signer's public key share
	VerifyRandomBeaconSig(sigShare crypto.Signature, block *hotstuff.Block, signerPubKey crypto.PublicKey) (bool, error)

	// VerifyStakingAggregatedSig verifies an aggregated BLS signature.
	// Inputs:
	//    aggStakingSig - the aggregated staking signature to be verified
	//    block - the block that the signature was signed for.
	//    signerKeys - the signer's public staking key
	// Note: the aggregated BLS staking signature is currently implemented as a slice of individual signatures.
	// The implementation (and method signature) will later be updated, once full BLS sigmnature aggregation is implemented.
	VerifyStakingAggregatedSig(aggStakingSig []crypto.Signature, block *hotstuff.Block, signerKeys []crypto.PublicKey) (bool, error)

	// VerifyRandomBeaconThresholdSig verifies a (reconstructed) random beacon threshold signature for a block.
	// Inputs:
	//    sig - random beacon threshold signature
	//    block - the signed block
	//    groupPubKey - the DKG group public key
	VerifyRandomBeaconThresholdSig(sig crypto.Signature, block *hotstuff.Block, groupPubKey crypto.PublicKey) (bool, error)
}

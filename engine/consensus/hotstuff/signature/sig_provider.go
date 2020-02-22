package signature

import (
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// SigProvider provides symmetry functions to generate and verify signatures
type SigProvider struct {
	sk     crypto.PrivateKey
	hasher crypto.Hasher
	myID   flow.Identifier
}

func NewSigProvider(sk crypto.PrivateKey, hasher crypto.Hasher, myID flow.Identifier) *SigProvider {
	return &SigProvider{
		sk:     sk,
		hasher: hasher,
		myID:   myID,
	}
}

// VerifySig verifies a single signature for a block using the given public key
// sig - the signature to be verified
// block - the block that the signature was signed for.
// signerKey - the public key of the signer who signed the block.
func (s *SigProvider) VerifySig(sig crypto.Signature, block *hotstuff.Block, signerKey crypto.PublicKey) (bool, error) {
	msg := BlockToBytesForSign(block)
	return signerKey.Verify(sig, msg[:], s.hasher)
}

// VerifyAggregatedSignature verifies an aggregated signature.
// aggsig - the aggregated signature to be verified
// block - the block that the signature was signed for.
// signerKeys - the public keys of all the signers who signed the block.
func (s *SigProvider) VerifyAggregatedSignature(aggsig *hotstuff.AggregatedSignature, block *hotstuff.Block, signerKeys []crypto.PublicKey) (bool, error) {

	// for now the aggregated signature for BLS signatures is implemented as a slice of all the signatures.
	// to verifiy it, we basically verify every single signature

	// check that the number of keys and signatures should match
	sigs := aggsig.Raw
	if len(sigs) != len(signerKeys) {
		return false, nil
	}

	// check each signature
	for i, sig := range sigs {
		signerKey := signerKeys[i]
		valid, err := s.VerifySig(sig, block, signerKey)
		if err != nil {
			return false, err
		}
		if !valid {
			return false, nil
		}
	}
	return true, nil
}

// Aggregate aggregates signatures into an aggregated signature
func (s *SigProvider) Aggregate(sigs []*hotstuff.SingleSignature) (*hotstuff.AggregatedSignature, error) {
	// check if sigs is empty
	if len(sigs) == 0 {
		return nil, fmt.Errorf("no signature to be aggregated")
	}

	// This implementation is a naive way of aggregation the signatures. It will work, with
	// the downside of costing more bandwidth.
	// The more optimal way, which is the real aggregation, will be implemented when the crypto
	// API is available.
	aggsig := make([]crypto.Signature, len(sigs))
	signerIDs := make([]flow.Identifier, len(sigs))
	for i, sig := range sigs {
		aggsig[i] = sig.Raw
		signerIDs[i] = sig.SignerID
	}

	return &hotstuff.AggregatedSignature{
		Raw:       aggsig,
		SignerIDs: signerIDs,
	}, nil
}

// VoteFor signs a Block and returns the Vote for that Block
func (s *SigProvider) VoteFor(block *hotstuff.Block) (*hotstuff.Vote, error) {
	// convert to bytes to be signed
	msg := BlockToBytesForSign(block)

	// generate signature
	signature, err := s.sk.Sign(msg[:], s.hasher)

	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x) to vote: %w", block.BlockID, err)
	}

	return &hotstuff.Vote{
		BlockID: block.BlockID,
		View:    block.View,
		Signature: &hotstuff.SingleSignature{
			Raw:      signature,
			SignerID: s.myID,
		},
	}, nil
}

// Propose signs a Block and returns the Proposal
func (s *SigProvider) Propose(block *hotstuff.Block) (*hotstuff.Proposal, error) {
	// convert to bytes to be signed
	msg := BlockToBytesForSign(block)

	// generate signature
	signature, err := s.sk.Sign(msg[:], s.hasher)

	if err != nil {
		return nil, fmt.Errorf("fail to sign block (%x): %w", block.BlockID, err)
	}

	return &hotstuff.Proposal{
		Block:     block,
		Signature: signature,
	}, nil
}

// BlockToBytesForSign generates the bytes that was signed for a block
// Note: this function should be reused when signing a block or a vote
func BlockToBytesForSign(block *hotstuff.Block) flow.Identifier {
	return flow.MakeID(struct {
		BlockID flow.Identifier
		View    uint64
	}{
		BlockID: block.BlockID,
		View:    block.View,
	})
}

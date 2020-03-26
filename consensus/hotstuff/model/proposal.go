package model

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Proposal represent a new proposed block within HotStuff (and thus a
// a header in the bigger picture), signed by the proposer.
type Proposal struct {
	Block                 *Block
	StakingSignature      crypto.Signature
	RandomBeaconSignature crypto.Signature
}

// ProposerVote extracts the proposer vote from the proposal
func (p *Proposal) ProposerVote() *Vote {
	sig := SingleSignature{
		SignerID:              p.Block.ProposerID,
		StakingSignature:      p.StakingSignature,
		RandomBeaconSignature: p.RandomBeaconSignature,
	}
	vote := Vote{
		BlockID:   p.Block.BlockID,
		View:      p.Block.View,
		Signature: &sig,
	}
	return &vote
}

// ProposalFromFlow turns a flow header into a hotstuff block type.
func ProposalFromFlow(header *flow.Header, parentView uint64) *Proposal {
	sig := AggregatedSignature{
		StakingSignatures:     header.ParentStakingSigs,
		RandomBeaconSignature: header.ParentRandomBeaconSig,
		SignerIDs:             header.ParentSigners,
	}
	qc := QuorumCertificate{
		BlockID:             header.ParentID,
		View:                parentView,
		AggregatedSignature: &sig,
	}
	block := Block{
		BlockID:     header.ID(),
		View:        header.View,
		QC:          &qc,
		ProposerID:  header.ProposerID,
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	proposal := Proposal{
		Block:                 &block,
		StakingSignature:      header.ProposerStakingSig,
		RandomBeaconSignature: header.ProposerRandomBeaconSig,
	}
	return &proposal
}

// ProposalToFlow turns a block proposal into a flow header.
func ProposalToFlow(proposal *Proposal) *flow.Header {
	block := proposal.Block
	header := flow.Header{
		ParentID:                block.QC.BlockID,
		PayloadHash:             block.PayloadHash,
		Timestamp:               block.Timestamp,
		View:                    block.View,
		ParentSigners:           block.QC.AggregatedSignature.SignerIDs,
		ParentStakingSigs:       block.QC.AggregatedSignature.StakingSignatures,
		ParentRandomBeaconSig:   block.QC.AggregatedSignature.RandomBeaconSignature,
		ProposerID:              block.ProposerID,
		ProposerStakingSig:      proposal.StakingSignature,
		ProposerRandomBeaconSig: proposal.RandomBeaconSignature,
	}
	return &header
}

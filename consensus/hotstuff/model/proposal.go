package model

import (
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
)

// Proposal represents a block proposal under construction.
// In order to decide whether a proposal is safe to sign, HotStuff's Safety Rules require
// proof that the leader entered the respective view in a protocol-compliant manner. Specifically,
// we require a TimeoutCertificate [TC] if and only if the QC in the block is _not_ for the
// immediately preceding view. Thereby we protect the consensus process from malicious leaders
// attempting to skip views that haven't concluded yet (a form of front-running attack).
// However, LastViewTC is only relevant until a QC is known that certifies the correctness of
// the block. Thereafter, the QC attests that honest consensus participants have confirmed the
// validity of the fork up to the latest certified block (including protocol-compliant view transitions).
//
// By explicitly differentiating the Proposal from the SignedProposal (extending Proposal by
// adding the proposer's signature), we can unify the algorithmic path of signing block proposals.
// This codifies the important aspect that a proposer's signature for their own block
// is conceptually also just a vote (we explicitly use that for aggregating votes, including the
// proposer's own vote to a QC). In order to express this conceptual equivalence in code, the
// voting logic in Safety Rules must also operate on an unsigned Proposal.
//
// TODO: atm, the flow.Header embeds the LastViewTC. However, for HotStuff we have `model.Block`
// and `model.Proposal`, where the latter was introduced when we added the PaceMaker to
// vanilla HotStuff. It would be more consistent, if we added `LastViewTC` to `model.Block`,
// or even better, introduce an interface for HotStuff's notion of a block (exposing
// the fields in `model.Block` plus LastViewTC)
type Proposal struct {
	Block      *Block
	LastViewTC *flow.TimeoutCertificate
}

// SignedProposal represent a new proposed block within HotStuff (and thus
// a header in the bigger picture), signed by the proposer.
//
// CAUTION: the signature only covers the pair (Block.View, Block.BlockID). Therefore, only
// the data that is hashed into the BlockID is cryptographically secured by the proposer's
// signature.
// Specifically, the proposer's signature cannot be covered by the Block.BlockID, as the
// proposer _signs_ the Block.BlockID (otherwise we have a cyclic dependency).
type SignedProposal struct {
	Proposal
	SigData []byte
}

// ProposerVote extracts the proposer vote from the proposal
func (p *SignedProposal) ProposerVote() *Vote {
	vote := Vote{
		View:     p.Block.View,
		BlockID:  p.Block.BlockID,
		SignerID: p.Block.ProposerID,
		SigData:  p.SigData,
	}
	return &vote
}

// SignedProposalFromFlow turns a flow header proposal into a hotstuff block type.
// Since not all header fields are exposed to HotStuff, this conversion is not reversible.
func SignedProposalFromFlow(p *flow.Proposal) *SignedProposal {
	proposal := SignedProposal{
		Proposal: Proposal{
			Block:      BlockFromFlow(p.Header),
			LastViewTC: p.Header.LastViewTC,
		},
		SigData: p.ProposerSigData,
	}
	return &proposal
}

// TODO(malleability, #7100) clean up conversion functions and/or proposal types here
func SignedProposalFromBlock(p *flow.BlockProposal) *SignedProposal {
	proposal := SignedProposal{
		Proposal: Proposal{
			Block:      BlockFromFlow(p.Block.Header),
			LastViewTC: p.Block.Header.LastViewTC,
		},
		SigData: p.ProposerSigData,
	}
	return &proposal
}

func SignedProposalFromClusterBlock(p *cluster.BlockProposal) *SignedProposal {
	proposal := SignedProposal{
		Proposal: Proposal{
			Block:      BlockFromFlow(p.Block.ToHeader()),
			LastViewTC: p.Block.Header.LastViewTC,
		},
		SigData: p.ProposerSigData,
	}
	return &proposal
}

// ProposalFromFlow turns an unsigned flow header into a unsigned hotstuff block type.
func ProposalFromFlow(header *flow.Header) *Proposal {
	proposal := Proposal{
		Block:      BlockFromFlow(header),
		LastViewTC: header.LastViewTC,
	}
	return &proposal
}

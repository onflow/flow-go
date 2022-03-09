package integration_test

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type Signer struct {
	localID flow.Identifier
}

func (*Signer) CreateProposal(block *model.Block) (*model.Proposal, error) {
	proposal := &model.Proposal{
		Block:   block,
		SigData: nil,
	}
	return proposal, nil
}
func (s *Signer) CreateVote(block *model.Block) (*model.Vote, error) {
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: s.localID,
		SigData:  nil,
	}
	return vote, nil
}
func (*Signer) CreateQC(votes []*model.Vote) (*flow.QuorumCertificate, error) {
	qc := &flow.QuorumCertificate{
		View:    votes[0].View,
		BlockID: votes[0].BlockID,
		// TODO: fix
		SignerIndices: nil,
		SigData:       nil,
	}
	return qc, nil
}

func (*Signer) VerifyVote(voterID *flow.Identity, sigData []byte, block *model.Block) error {
	return nil
}

func (*Signer) VerifyQC(voters flow.IdentityList, sigData []byte, block *model.Block) error {
	return nil
}

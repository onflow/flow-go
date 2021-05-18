package verification

import (
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

type MockSigner struct {
	LocalID flow.Identifier
}

func (*MockSigner) CreateProposal(block *model.Block) (*model.Proposal, error) {
	proposal := &model.Proposal{
		Block:   block,
		SigData: nil,
	}
	return proposal, nil
}
func (s *MockSigner) CreateVote(block *model.Block) (*model.Vote, error) {
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: s.LocalID,
		SigData:  nil,
	}
	return vote, nil
}
func (*MockSigner) CreateQC(votes []*model.Vote) (*flow.QuorumCertificate, error) {
	voterIDs := make([]flow.Identifier, 0, len(votes))
	for _, vote := range votes {
		voterIDs = append(voterIDs, vote.SignerID)
	}
	qc := &flow.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: voterIDs,
		SigData:   nil,
	}
	return qc, nil
}

func (*MockSigner) VerifyVote(voterID *flow.Identity, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

func (*MockSigner) VerifyQC(voters flow.IdentityList, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

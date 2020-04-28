package integration_test

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
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
func (*Signer) CreateQC(votes []*model.Vote) (*model.QuorumCertificate, error) {
	voterIDs := make([]flow.Identifier, 0, len(votes))
	for _, vote := range votes {
		voterIDs = append(voterIDs, vote.SignerID)
	}
	qc := &model.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: voterIDs,
		SigData:   nil,
	}
	return qc, nil
}

func (*Signer) VerifyVote(voterID flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

func (*Signer) VerifyQC(voterIDs []flow.Identifier, sigData []byte, block *model.Block) (bool, error) {
	return true, nil
}

// move to in memory network
type SubmitFunc func(uint8, interface{}, ...flow.Identifier) error

type Conduit struct {
	channelID uint8
	submit    SubmitFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}

type Network struct {
	engines  map[uint8]network.Engine
	conduits []*Conduit
}

func NewNetwork() *Network {
	return &Network{
		engines:  make(map[uint8]network.Engine),
		conduits: make([]*Conduit, 0),
	}
}

func (n *Network) Register(code uint8, engine network.Engine) (network.Conduit, error) {
	n.engines[code] = engine
	// the submit function needs the access to all the nodes,
	// so will be added later
	c := &Conduit{
		channelID: code,
	}
	n.conduits = append(n.conduits, c)
	return c, nil
}

func (n *Network) WithSubmit(submit SubmitFunc) *Network {
	for _, conduit := range n.conduits {
		conduit.submit = submit
	}
	return n
}

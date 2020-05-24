package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// TODO consolidate this with model/flow/Block.Genesis
func GenerateRootBlock(identityList flow.IdentityList) *flow.Block {
	payload := flow.Payload{
		Identities: identityList,
		Guarantees: nil,
		Seals:      nil,
	}
	header := flow.Header{
		ChainID:        flow.GetChainID(),
		ParentID:       flow.ZeroID,
		Height:         0,
		PayloadHash:    payload.Hash(),
		Timestamp:      flow.GenesisTime(),
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	return &flow.Block{
		Header:  &header,
		Payload: &payload,
	}
}

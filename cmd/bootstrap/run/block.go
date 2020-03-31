package run

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootBlock(identityList flow.IdentityList, seal flow.Seal) flow.Block {
	payload := flow.Payload{
		Identities: identityList,
		Guarantees: nil,
		Seals:      []*flow.Seal{&seal},
	}
	header := flow.Header{
		ChainID:        flow.DefaultChainID,
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

	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

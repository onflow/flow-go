package run

import (
	"time"

	"github.com/dapperlabs/flow-go/model/flow"
)

func GenerateRootBlock(chainID flow.ChainID, parentID flow.Identifier, height uint64, timestamp time.Time, participants flow.IdentityList) *flow.Block {
	// TODO: generate root seal with root system events for epoch setup & commit (including identities)
	payload := flow.Payload{
		Guarantees: nil,
		Seals:      nil,
	}
	header := flow.Header{
		ChainID:        chainID,
		ParentID:       parentID,
		Height:         height,
		PayloadHash:    payload.Hash(),
		Timestamp:      timestamp,
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

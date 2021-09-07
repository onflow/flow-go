package run

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootBlock(chainID flow.ChainID, parentID flow.Identifier, height uint64, timestamp time.Time) *flow.Block {

	payload := flow.Payload{
		Guarantees: nil,
		Seals:      nil,
	}
	header := flow.Header{
		ChainID:            chainID,
		ParentID:           parentID,
		Height:             height,
		PayloadHash:        payload.Hash(),
		Timestamp:          timestamp,
		View:               0,
		ParentVoterIDs:     nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
		ProposerSigData:    nil,
	}

	return &flow.Block{
		Header:  &header,
		Payload: &payload,
	}
}

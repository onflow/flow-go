package run

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootHeader(chainID flow.ChainID, parentID flow.Identifier, height uint64, timestamp time.Time) *flow.Header {
	return flow.NewRootHeader(flow.UntrustedHeader{
		HeaderBody: flow.HeaderBody{
			ChainID:            chainID,
			ParentID:           parentID,
			Height:             height,
			Timestamp:          timestamp,
			View:               0,
			ParentVoterIndices: nil,
			ParentVoterSigData: nil,
			ProposerID:         flow.ZeroID,
		},
		PayloadHash: flow.ZeroID,
	})
}

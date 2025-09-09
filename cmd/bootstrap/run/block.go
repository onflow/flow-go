package run

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootHeaderBody(chainID flow.ChainID, parentID flow.Identifier, height uint64, view uint64, timestamp time.Time) (*flow.HeaderBody, error) {
	rootHeaderBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            chainID,
		ParentID:           parentID,
		Height:             height,
		Timestamp:          uint64(timestamp.UnixMilli()),
		View:               view,
		ParentVoterIndices: nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate root header body: %w", err)
	}

	return rootHeaderBody, nil
}

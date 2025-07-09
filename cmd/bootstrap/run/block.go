package run

import (
	"fmt"
	"time"

	"github.com/onflow/flow-go/model/flow"
)

func GenerateRootHeader(chainID flow.ChainID, parentID flow.Identifier, height uint64, timestamp time.Time) (*flow.Header, error) {
	rootHeaderBody, err := flow.NewRootHeaderBody(flow.UntrustedHeaderBody{
		ChainID:            chainID,
		ParentID:           parentID,
		Height:             height,
		Timestamp:          timestamp,
		View:               0,
		ParentVoterIndices: nil,
		ParentVoterSigData: nil,
		ProposerID:         flow.ZeroID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate root header body: %w", err)
	}

	rootHeader, err := flow.NewGenesisHeader(flow.UntrustedHeader{
		HeaderBody:  *rootHeaderBody,
		PayloadHash: flow.ZeroID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate root header: %w", err)
	}

	return rootHeader, nil
}

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
	header := flow.NewHeader(
		chainID,
		parentID,
		height,
		payload.Hash(),
		timestamp,
		0,
		nil,
		nil,
		flow.ZeroID,
		nil)

	return &flow.Block{
		Header:  header,
		Payload: &payload,
	}
}

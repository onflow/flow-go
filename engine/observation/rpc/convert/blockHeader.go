package convert

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
)

func BlockHeaderToMessage(h *flow.Header) (entities.BlockHeader, error) {
	bh := entities.BlockHeader{
		Hash:              h.PayloadHash[:],
		PreviousBlockHash: h.ParentID[:],
		Number:            h.Height,
	}
	return bh, nil
}

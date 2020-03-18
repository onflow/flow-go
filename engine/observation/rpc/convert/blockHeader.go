package convert

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
)

func BlockHeaderToMessage(h *flow.Header) (entities.BlockHeader, error) {
	id := h.ID()
	bh := entities.BlockHeader{
		Hash:              id[:],
		PreviousBlockHash: h.ParentID[:],
		Number:            h.Height,
	}
	return bh, nil
}

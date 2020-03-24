package convert

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
)

func BlockHeaderToMessage(h *flow.Header) (entities.BlockHeader, error) {
	id := h.ID()
	bh := entities.BlockHeader{
		Id:       id[:],
		ParentId: h.ParentID[:],
		Height:   h.Height,
	}
	return bh, nil
}

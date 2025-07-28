package integration

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func DefaultRoot() *flow.UnsignedHeader {
	header := &flow.UnsignedHeader{
		HeaderBody: flow.HeaderBody{
			ChainID:   "chain",
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: uint64(time.Now().UnixMilli()),
		},
		PayloadHash: unittest.IdentifierFixture(),
	}
	return header
}

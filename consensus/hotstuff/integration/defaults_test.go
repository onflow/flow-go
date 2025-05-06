package integration

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func DefaultRoot() *flow.Header {
	header := &flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:   "chain",
			ParentID:  flow.ZeroID,
			Height:    0,
			Timestamp: time.Now().UTC(),
		},
		PayloadHash: unittest.IdentifierFixture(),
	}
	return header
}

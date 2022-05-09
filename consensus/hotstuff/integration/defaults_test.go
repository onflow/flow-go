package integration

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func DefaultRoot() *flow.Header {
	header := &flow.Header{
		ChainID:     "chain",
		ParentID:    flow.ZeroID,
		Height:      0,
		PayloadHash: unittest.IdentifierFixture(),
		Timestamp:   time.Now().UTC(),
	}
	return header
}

func DefaultPruned() uint64 {
	return 0
}

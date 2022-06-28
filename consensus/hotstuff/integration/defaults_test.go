package integration

import (
	"time"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func DefaultRoot() *flow.Header {
	return flow.NewHeader(
		"chain",
		flow.ZeroID,
		0,
		unittest.IdentifierFixture(),
		time.Now().UTC(),
		0,
		nil,
		nil,
		flow.ZeroID,
		nil)
}

func DefaultStart() uint64 {
	return 1
}

func DefaultPruned() uint64 {
	return 0
}

func DefaultVoted() uint64 {
	return 0
}

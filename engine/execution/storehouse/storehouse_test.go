package storehouse_test

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

var unknownBlock = unittest.IdentifierFixture()
var unknownReg = makeReg("unknown", "unknown")

func makeReg(key string, value string) flow.RegisterEntry {
	return flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: "owner",
			Key:   key,
		},
		Value: []byte(value),
	}
}

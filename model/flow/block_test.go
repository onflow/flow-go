package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func Test_NilProducesSameHashAsEmptySlice(t *testing.T) {

	nilPayload := flow.Payload{
		Identities: nil,
		Guarantees: nil,
		Seals:      nil,
	}

	slicePayload := flow.Payload{
		Identities: make([]*flow.Identity, 0),
		Guarantees: make([]*flow.CollectionGuarantee, 0),
		Seals:      make([]*flow.Seal, 0),
	}

	assert.Equal(t, nilPayload.Hash(), slicePayload.Hash())
}

func Test_OrderingChangesHash(t *testing.T) {

	identities := unittest.IdentityListFixture(5)

	payload1 := flow.Payload{
		Identities: identities,
	}

	payload2 := flow.Payload{
		Identities: []*flow.Identity{identities[3], identities[2], identities[4], identities[1], identities[0]},
	}

	assert.NotEqual(t, payload1.Hash(), payload2.Hash())
}

package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestProtocolStateEntry_Copy tests if the copy method returns a deep copy of the entry. All changes to cpy shouldn't affect the original entry.
func TestProtocolStateEntry_Copy(t *testing.T) {
	entry := &unittest.ProtocolStateFixture(unittest.WithNextEpochProtocolState()).ProtocolStateEntry
	cpy := entry.Copy()
	assert.Equal(t, entry, cpy)
	assert.NotSame(t, entry.NextEpochProtocolState, cpy.NextEpochProtocolState)
	assert.NotSame(t, entry.PreviousEpochEventIDs, cpy.PreviousEpochEventIDs)
	assert.NotSame(t, entry.CurrentEpochEventIDs, cpy.CurrentEpochEventIDs)

	cpy.InvalidStateTransitionAttempted = !entry.InvalidStateTransitionAttempted
	assert.NotEqual(t, entry, cpy)

	assert.Equal(t, entry.Identities[0], cpy.Identities[0])
	cpy.Identities[0].Dynamic.Weight = 123
	assert.NotEqual(t, entry.Identities[0], cpy.Identities[0])

	cpy.Identities = append(cpy.Identities, &flow.DynamicIdentityEntry{
		NodeID: unittest.IdentifierFixture(),
		Dynamic: flow.DynamicIdentity{
			Weight:  100,
			Ejected: false,
		},
	})
	assert.NotEqual(t, entry.Identities, cpy.Identities)
}

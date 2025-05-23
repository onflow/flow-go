package flow_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack/v4"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIdentityEncodingJSON(t *testing.T) {

	t.Run("normal identity", func(t *testing.T) {
		identity := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
		enc, err := json.Marshal(identity)
		require.NoError(t, err)
		var dec flow.Identity
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		require.True(t, identity.EqualTo(&dec))
	})

	t.Run("empty address should be omitted", func(t *testing.T) {
		identity := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
		identity.Address = ""
		enc, err := json.Marshal(identity)
		require.NoError(t, err)
		// should have no address field in output
		assert.False(t, strings.Contains(string(enc), "Address"))
		var dec flow.Identity
		err = json.Unmarshal(enc, &dec)
		require.NoError(t, err)
		require.True(t, identity.EqualTo(&dec))
	})
}

func TestIdentityEncodingMsgpack(t *testing.T) {
	identity := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
	enc, err := msgpack.Marshal(identity)
	require.NoError(t, err)
	var dec flow.Identity
	err = msgpack.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.True(t, identity.EqualTo(&dec))
}

func TestIdentityList_Exists(t *testing.T) {
	t.Run("should find a given element", func(t *testing.T) {
		il1 := unittest.IdentityListFixture(10)
		il2 := unittest.IdentityListFixture(1)

		// sort the first list
		il1 = il1.Sort(flow.Canonical[flow.Identity])

		for i := 0; i < 10; i++ {
			assert.True(t, il1.Exists(il1[i]))
		}
		assert.False(t, il1.Exists(il2[0]))
	})
}

func TestIdentityList_IdentifierExists(t *testing.T) {
	t.Run("should find a given identifier", func(t *testing.T) {
		il1 := unittest.IdentityListFixture(10)
		il2 := unittest.IdentityListFixture(1)

		// sort the first list
		il1 = il1.Sort(flow.Canonical[flow.Identity])

		for i := 0; i < 10; i++ {
			assert.True(t, il1.IdentifierExists(il1[i].NodeID))
		}
		assert.False(t, il1.IdentifierExists(il2[0].NodeID))
	})
}

func TestIdentityList_Union(t *testing.T) {
	t.Run("retains the original identity list", func(t *testing.T) {
		// An identity list is a slice, i.e. it is vulnerable to in-place modifications via append.
		// Per convention, all IdentityList operations should leave the original lists invariant.
		// Here, we purposefully create a slice, whose backing array has enough space to
		// include the elements we are going to add via Union. Furthermore, we force element duplication
		// by taking the union of the IdentityList with itself. If the implementation is not careful
		// about creating copies and works with the slices itself, it will modify the input and fail the test.

		il := unittest.IdentityListFixture(20)
		il = il[:10]
		ilBackup := il.Copy()

		_ = il.Union(il)
		assert.Equal(t, ilBackup, il)
	})
	t.Run("should contain all identities", func(t *testing.T) {
		il1 := unittest.IdentityListFixture(10)
		il2 := unittest.IdentityListFixture(10)

		union := il1.Union(il2)

		uniques := make(map[flow.Identifier]struct{})

		// should contain all items form 1 and 2, since there are no duplicates
		assert.Len(t, union, len(il1)+len(il2))
		for _, identity := range union {
			_, in1 := il1.ByNodeID(identity.NodeID)
			_, in2 := il2.ByNodeID(identity.NodeID)
			// each item should be in one of the input lists
			assert.True(t, in1 || in2)

			// there should be no duplicates
			_, dupe := uniques[identity.NodeID]
			assert.False(t, dupe)
			uniques[identity.NodeID] = struct{}{}
		}
	})

	t.Run("should omit duplicates", func(t *testing.T) {
		il1 := unittest.IdentityListFixture(10)
		il2 := unittest.IdentityListFixture(10)
		// add one duplicate between the two lists, which should be included only once
		dup := il1[0]
		il2[0] = dup

		union := il1.Union(il2)

		uniques := make(map[flow.Identifier]struct{})

		// should contain one less than the sum of the two input list lengths since there is a dupe
		assert.Len(t, union, len(il1)+len(il2)-1)
		for _, identity := range union {
			_, in1 := il1.ByNodeID(identity.NodeID)
			_, in2 := il2.ByNodeID(identity.NodeID)
			// each item should be in one of the input lists
			assert.True(t, in1 || in2)

			// there should be no duplicates
			_, dupe := uniques[identity.NodeID]
			assert.False(t, dupe)
			uniques[identity.NodeID] = struct{}{}
		}
	})
}

func TestSample(t *testing.T) {
	t.Run("Sample max", func(t *testing.T) {
		il := unittest.IdentityListFixture(10)
		sam, err := il.Sample(10)
		require.NoError(t, err)
		require.Equal(t, uint(10), sam.Count())
	})

	t.Run("Sample oversized", func(t *testing.T) {
		il := unittest.IdentityListFixture(10)
		sam, err := il.Sample(11)
		require.NoError(t, err)
		require.Equal(t, uint(10), sam.Count())
	})
}

func TestShuffle(t *testing.T) {
	t.Run("should be shuffled", func(t *testing.T) {
		il := unittest.IdentityListFixture(15) // ~1/billion chance of shuffling to input state
		shuffled, err := il.Shuffle()
		require.NoError(t, err)
		assert.Equal(t, len(il), len(shuffled))
		assert.ElementsMatch(t, il, shuffled)
	})
	t.Run("should not be deterministic", func(t *testing.T) {
		il := unittest.IdentityListFixture(10)
		shuffled1, err := il.Shuffle()
		require.NoError(t, err)
		shuffled2, err := il.Shuffle()
		require.NoError(t, err)
		assert.NotEqual(t, shuffled1, shuffled2)
		assert.ElementsMatch(t, shuffled1, shuffled2)
	})
}

// check that identities consistently hash to the same ID, even with different
// public key implementations
func TestIdentity_ID(t *testing.T) {
	identity1 := unittest.IdentityFixture(unittest.WithKeys)
	var identity2 = new(flow.Identity)
	*identity2 = *identity1
	identity2.StakingPubKey = encodable.StakingPubKey{PublicKey: identity1.StakingPubKey}

	id1 := flow.MakeID(identity1)
	id2 := flow.MakeID(identity2)
	assert.Equal(t, id1, id2)
}

func TestIdentity_Sort(t *testing.T) {
	il := unittest.IdentityListFixture(20)
	// make sure the list is not sorted
	il[0].NodeID[0], il[1].NodeID[0] = 2, 1
	require.False(t, flow.IsCanonical(il[0], il[1]))
	assert.False(t, flow.IsIdentityListCanonical(il))

	canonical := il.Sort(flow.Canonical[flow.Identity])
	assert.True(t, flow.IsIdentityListCanonical(canonical))

	// check `IsIdentityListCanonical` detects order equality in a sorted list
	il[1] = il[10] // add a duplication
	canonical = il.Sort(flow.Canonical[flow.Identity])
	assert.False(t, flow.IsIdentityListCanonical(canonical))
}

func TestIdentity_EqualTo(t *testing.T) {

	pks := unittest.PublicKeysFixture(2, crypto.ECDSASecp256k1)

	t.Run("empty are equal", func(t *testing.T) {
		a := &flow.Identity{}
		b := &flow.Identity{}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("NodeID diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{NodeID: [32]byte{1, 2, 3}}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{NodeID: [32]byte{2, 2, 2}}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Address diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{Address: "b"}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{Address: "c"}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Role diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{Role: flow.RoleCollection}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{Role: flow.RoleExecution}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Initial weight diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{InitialWeight: 1}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{InitialWeight: 2}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("status diff", func(t *testing.T) {
		a := &flow.Identity{DynamicIdentity: flow.DynamicIdentity{EpochParticipationStatus: flow.EpochParticipationStatusActive}}
		b := &flow.Identity{DynamicIdentity: flow.DynamicIdentity{EpochParticipationStatus: flow.EpochParticipationStatusLeaving}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("StakingPubKey diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{StakingPubKey: pks[0]}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{StakingPubKey: pks[1]}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("NetworkPubKey diff", func(t *testing.T) {
		a := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{NetworkPubKey: pks[0]}}
		b := &flow.Identity{IdentitySkeleton: flow.IdentitySkeleton{NetworkPubKey: pks[1]}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Same data equals", func(t *testing.T) {
		a := &flow.Identity{
			IdentitySkeleton: flow.IdentitySkeleton{
				NodeID:        flow.Identifier{1, 2, 3},
				Address:       "address",
				Role:          flow.RoleCollection,
				InitialWeight: 23,
				StakingPubKey: pks[0],
				NetworkPubKey: pks[1],
			},
			DynamicIdentity: flow.DynamicIdentity{
				EpochParticipationStatus: flow.EpochParticipationStatusActive,
			},
		}
		b := &flow.Identity{
			IdentitySkeleton: flow.IdentitySkeleton{
				NodeID:        flow.Identifier{1, 2, 3},
				Address:       "address",
				Role:          flow.RoleCollection,
				InitialWeight: 23,
				StakingPubKey: pks[0],
				NetworkPubKey: pks[1],
			},
			DynamicIdentity: flow.DynamicIdentity{
				EpochParticipationStatus: flow.EpochParticipationStatusActive,
			},
		}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

func TestIdentityList_EqualTo(t *testing.T) {

	t.Run("empty are equal", func(t *testing.T) {
		a := flow.IdentityList{}
		b := flow.IdentityList{}

		require.True(t, flow.IdentityListEqualTo(a, b))
		require.True(t, flow.IdentityListEqualTo(b, a))
	})

	t.Run("different len arent equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()

		a := flow.IdentityList{identityA}
		b := flow.IdentityList{}

		require.False(t, flow.IdentityListEqualTo(a, b))
		require.False(t, flow.IdentityListEqualTo(b, a))
	})

	t.Run("different data means not equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()
		identityB := unittest.IdentityFixture()

		a := flow.IdentityList{identityA}
		b := flow.IdentityList{identityB}

		require.False(t, flow.IdentityListEqualTo(a, b))
		require.False(t, flow.IdentityListEqualTo(b, a))
	})

	t.Run("same data means equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()

		a := flow.IdentityList{identityA, identityA}
		b := flow.IdentityList{identityA, identityA}

		require.True(t, flow.IdentityListEqualTo(a, b))
		require.True(t, flow.IdentityListEqualTo(b, a))
	})
}

func TestIdentityList_GetIndex(t *testing.T) {
	t.Run("should return expected index of identifier in identity list and true", func(t *testing.T) {
		participants := unittest.IdentityListFixture(3)
		index, ok := participants.GetIndex(participants[1].NodeID)
		require.True(t, ok)
		require.Equal(t, uint(1), index)
	})

	t.Run("should return 0 and false for identifier not found in identity list", func(t *testing.T) {
		participants := unittest.IdentityListFixture(3)
		index, ok := participants.GetIndex(unittest.IdentifierFixture())
		require.False(t, ok)
		require.Equal(t, uint(0), index)
	})
}

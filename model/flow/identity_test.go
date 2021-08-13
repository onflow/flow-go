package flow_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestHexStringToIdentifier(t *testing.T) {
	type testcase struct {
		hex         string
		expectError bool
	}

	cases := []testcase{{
		// non-hex characters
		hex:         "123456789012345678901234567890123456789012345678901234567890123z",
		expectError: true,
	}, {
		// too short
		hex:         "1234",
		expectError: true,
	}, {
		// just right
		hex:         "1234567890123456789012345678901234567890123456789012345678901234",
		expectError: false,
	}}

	for _, tcase := range cases {
		id, err := flow.HexStringToIdentifier(tcase.hex)
		if tcase.expectError {
			assert.Error(t, err)
			continue
		} else {
			assert.NoError(t, err)
		}

		assert.Equal(t, tcase.hex, id.String())
	}
}

/*
func TestIdentityEncodingJSON(t *testing.T) {
	identity := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
	enc, err := json.Marshal(identity)
	require.NoError(t, err)
	var dec flow.Identity
	err = json.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, identity, &dec)
}

func TestIdentityEncodingMsgpack(t *testing.T) {
	identity := unittest.IdentityFixture(unittest.WithRandomPublicKeys())
	enc, err := msgpack.Marshal(identity)
	require.NoError(t, err)
	var dec flow.Identity
	err = msgpack.Unmarshal(enc, &dec)
	require.NoError(t, err)
	require.Equal(t, identity, &dec)
}
*/

func TestIdentityList_Union(t *testing.T) {

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
		require.Equal(t, uint(10), il.Sample(10).Count())
	})

	t.Run("Sample oversized", func(t *testing.T) {
		il := unittest.IdentityListFixture(10)
		require.Equal(t, uint(10), il.Sample(11).Count())
	})
}

func TestShuffle(t *testing.T) {
	t.Run("should be shuffled", func(t *testing.T) {
		il := unittest.IdentityListFixture(15) // ~1/billion chance of shuffling to input state
		shuffled := il.DeterministicShuffle(rand.Int63())
		assert.Equal(t, len(il), len(shuffled))
		assert.ElementsMatch(t, il, shuffled)
	})
	t.Run("should be deterministic", func(t *testing.T) {
		il := unittest.IdentityListFixture(10)
		seed := rand.Int63()
		shuffled1 := il.DeterministicShuffle(seed)
		shuffled2 := il.DeterministicShuffle(seed)
		assert.Equal(t, shuffled1, shuffled2)
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
	random := il.DeterministicShuffle(time.Now().UnixNano())
	assert.False(t, random.Sorted(order.Canonical))

	canonical := il.Sort(order.Canonical)
	assert.True(t, canonical.Sorted(order.Canonical))
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
		a := &flow.Identity{NodeID: [32]byte{1, 2, 3}}
		b := &flow.Identity{NodeID: [32]byte{2, 2, 2}}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Address diff", func(t *testing.T) {
		a := &flow.Identity{Address: "b"}
		b := &flow.Identity{Address: "c"}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Role diff", func(t *testing.T) {
		a := &flow.Identity{Role: flow.RoleCollection}
		b := &flow.Identity{Role: flow.RoleExecution}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Stake diff", func(t *testing.T) {
		a := &flow.Identity{Stake: 1}
		b := &flow.Identity{Stake: 2}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Ejected diff", func(t *testing.T) {
		a := &flow.Identity{Ejected: true}
		b := &flow.Identity{Ejected: false}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("StakingPubKey diff", func(t *testing.T) {
		a := &flow.Identity{StakingPubKey: pks[0]}
		b := &flow.Identity{StakingPubKey: pks[1]}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("NetworkPubKey diff", func(t *testing.T) {
		a := &flow.Identity{NetworkPubKey: pks[0]}
		b := &flow.Identity{NetworkPubKey: pks[1]}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("Same data equals", func(t *testing.T) {
		a := &flow.Identity{
			NodeID:        flow.Identifier{1, 2, 3},
			Address:       "address",
			Role:          flow.RoleCollection,
			Stake:         23,
			Ejected:       false,
			StakingPubKey: pks[0],
			NetworkPubKey: pks[1],
		}
		b := &flow.Identity{
			NodeID:        flow.Identifier{1, 2, 3},
			Address:       "address",
			Role:          flow.RoleCollection,
			Stake:         23,
			Ejected:       false,
			StakingPubKey: pks[0],
			NetworkPubKey: pks[1],
		}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

func TestIdentityList_EqualTo(t *testing.T) {

	t.Run("empty are equal", func(t *testing.T) {
		a := flow.IdentityList{}
		b := flow.IdentityList{}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})

	t.Run("different len arent equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()

		a := flow.IdentityList{identityA}
		b := flow.IdentityList{}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("different data means not equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()
		identityB := unittest.IdentityFixture()

		a := flow.IdentityList{identityA}
		b := flow.IdentityList{identityB}

		require.False(t, a.EqualTo(b))
		require.False(t, b.EqualTo(a))
	})

	t.Run("same data means equal", func(t *testing.T) {
		identityA := unittest.IdentityFixture()

		a := flow.IdentityList{identityA, identityA}
		b := flow.IdentityList{identityA, identityA}

		require.True(t, a.EqualTo(b))
		require.True(t, b.EqualTo(a))
	})
}

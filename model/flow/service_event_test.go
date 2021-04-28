package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	testassert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmihailenco/msgpack"
	"gotest.tools/assert"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEventConversion(t *testing.T) {

	t.Run("epoch setup", func(t *testing.T) {
		fixture := unittest.EventFixture(flow.EventEpochSetup, uint32(1), uint32(1), unittest.IdentifierFixture())
		fixture.Payload = []byte(`{"type":"Event","value":{"id":"A.01cf0e2f2f715450.FlowEpoch.EpochSetup","fields":[{"name":"counter","value":{"type":"UInt64","value":"1"}},{"name":"nodeInfo","value":{"type":"Array","value":[{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"0000000000000000000000000000000000000000000000000000000000000000"}},{"name":"role","value":{"type":"UInt8","value":"1"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}},{"name":"networkingKey","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}},{"name":"stakingKey","value":{"type":"String","value":"000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"0000000000000000000000000000000000000000000000000000000000000001"}},{"name":"role","value":{"type":"UInt8","value":"2"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"}},{"name":"networkingKey","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"}},{"name":"stakingKey","value":{"type":"String","value":"000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"0000000000000000000000000000000000000000000000000000000000000002"}},{"name":"role","value":{"type":"UInt8","value":"3"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"}},{"name":"networkingKey","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"}},{"name":"stakingKey","value":{"type":"String","value":"000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"0000000000000000000000000000000000000000000000000000000000000003"}},{"name":"role","value":{"type":"UInt8","value":"4"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"}},{"name":"networkingKey","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"}},{"name":"stakingKey","value":{"type":"String","value":"000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}}]}},{"name":"firstView","value":{"type":"UInt64","value":"70"}},{"name":"finalView","value":{"type":"UInt64","value":"139"}},{"name":"collectorClusters","value":{"type":"Array","value":[{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"0"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[{"key":{"type":"String","value":"0000000000000000000000000000000000000000000000000000000000000000"},"value":{"type":"UInt64","value":"100"}}]}},{"name":"totalWeight","value":{"type":"UInt64","value":"100"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"1"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"2"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"3"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}}]}},{"name":"randomSource","value":{"type":"String","value":"15800823147682539441"}},{"name":"DKGPhase1FinalView","value":{"type":"UInt64","value":"121"}},{"name":"DKGPhase2FinalView","value":{"type":"UInt64","value":"123"}},{"name":"DKGPhase3FinalView","value":{"type":"UInt64","value":"125"}}]}}`)

		// decode JSON into cadence.Value
		decoded, err := jsoncdc.Decode(fixture.Payload)
		require.NoError(t, err)

		// cast to cadence.Event for parsing
		expected := decoded.(cadence.Event)

		// convert Cadence types to Go types
		event, err := flow.ConvertServiceEvent(fixture)
		require.NoError(t, err)

		// cast event type to epoch setup
		actual := event.Event.(*flow.EpochSetup)

		// check if converted values same as expected
		testassert.Equal(t, actual.Counter, uint64(expected.Fields[0].(cadence.UInt64)))
		testassert.Equal(t, actual.FirstView, uint64(expected.Fields[2].(cadence.UInt64)))
		testassert.Equal(t, actual.FinalView, uint64(expected.Fields[3].(cadence.UInt64)))
		testassert.Equal(t, actual.RandomSource, []byte(expected.Fields[5].(cadence.String)))

		// make sure assignment lengths are equal
		testassert.Len(t, actual.Assignments, len(expected.Fields[4].(cadence.Array).Values))

		// check if length of nodes per cluster is the same expected
		for index, assignment := range actual.Assignments {

			cluster := expected.Fields[4].(cadence.Array).Values[index].(cadence.Struct)

			// read node weights by ID map to match length of nodes in this cluster
			testassert.Len(t, assignment, len(cluster.Fields[1].(cadence.Dictionary).Pairs))
		}
	})
}

func TestEncodeDecode(t *testing.T) {

	setup := unittest.EpochSetupFixture()
	commit := unittest.EpochCommitFixture()

	t.Run("json", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			b, err := json.Marshal(setup)
			require.Nil(t, err)

			gotSetup := new(flow.EpochSetup)
			err = json.Unmarshal(b, gotSetup)
			require.Nil(t, err)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = json.Marshal(commit)
			require.Nil(t, err)

			gotCommit := new(flow.EpochCommit)
			err = json.Unmarshal(b, gotCommit)
			require.Nil(t, err)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})

		t.Run("generic type", func(t *testing.T) {
			b, err := json.Marshal(setup.ServiceEvent())
			require.Nil(t, err)

			outer := new(flow.ServiceEvent)
			err = json.Unmarshal(b, outer)
			require.Nil(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = json.Marshal(commit.ServiceEvent())
			require.Nil(t, err)

			outer = new(flow.ServiceEvent)
			err = json.Unmarshal(b, outer)
			require.Nil(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})
	})

	t.Run("msgpack", func(t *testing.T) {
		t.Run("specific event types", func(t *testing.T) {
			b, err := msgpack.Marshal(setup)
			require.Nil(t, err)

			gotSetup := new(flow.EpochSetup)
			err = msgpack.Unmarshal(b, gotSetup)
			require.Nil(t, err)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = msgpack.Marshal(commit)
			require.Nil(t, err)

			gotCommit := new(flow.EpochCommit)
			err = msgpack.Unmarshal(b, gotCommit)
			require.Nil(t, err)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})

		t.Run("generic type", func(t *testing.T) {
			b, err := msgpack.Marshal(setup.ServiceEvent())
			require.Nil(t, err)

			outer := new(flow.ServiceEvent)
			err = msgpack.Unmarshal(b, outer)
			require.Nil(t, err)
			gotSetup, ok := outer.Event.(*flow.EpochSetup)
			require.True(t, ok)
			assert.DeepEqual(t, setup, gotSetup)

			b, err = msgpack.Marshal(commit.ServiceEvent())
			require.Nil(t, err)

			outer = new(flow.ServiceEvent)
			err = msgpack.Unmarshal(b, outer)
			require.Nil(t, err)
			gotCommit, ok := outer.Event.(*flow.EpochCommit)
			require.True(t, ok)
			assert.DeepEqual(t, commit, gotCommit, cmp.FilterValues(func(a, b crypto.PublicKey) bool {
				return true
			}, cmp.Comparer(func(a, b crypto.PublicKey) bool {
				return a.Equals(b)
			})))
		})
	})
}

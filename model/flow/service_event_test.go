package flow_test

import (
	"encoding/hex"
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

		// Note: this JSON was taken from https://github.com/onflow/flow-core-contracts/ tests
		// with 4 clusters who have 1 node each
		fixture.Payload = []byte(`{"type":"Event","value":{"id":"A.01cf0e2f2f715450.FlowEpoch.EpochSetup","fields":[{"name":"counter","value":{"type":"UInt64","value":"1"}},{"name":"nodeInfo","value":{"type":"Array","value":[{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"e9ffa1712474abc92c48e209051f335b1d9b29226f8a9471dabcaa53f62d3746"}},{"name":"role","value":{"type":"UInt8","value":"1"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"}},{"name":"networkingKey","value":{"type":"String","value":"378dbf45d85c614feb10d8bd4f78f4b6ef8eec7d987b937e123255444657fb3da031f232a507e323df3a6f6b8f50339c51d188e80c0e7a92420945cc6ca893fc"}},{"name":"stakingKey","value":{"type":"String","value":"af4aade26d76bb2ab15dcc89adcef82a51f6f04b3cb5f4555214b40ec89813c7a5f95776ea4fe449de48166d0bbc59b919b7eabebaac9614cf6f9461fac257765415f4d8ef1376a2365ec9960121888ea5383d88a140c24c29962b0a14e4e4e7"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"bef02e1a8ca72dd27e78650bcb793e6cbc54549f785232c0bb45099f25ee50db"}},{"name":"role","value":{"type":"UInt8","value":"2"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001"}},{"name":"networkingKey","value":{"type":"String","value":"cfdfe8e4362c8f79d11772cb7277ab16e5033a63e8dd5d34caf1b041b77e5b2d63c2072260949ccf8907486e4cfc733c8c42ca0e4e208f30470b0d950856cd47"}},{"name":"stakingKey","value":{"type":"String","value":"8207559cd7136af378bba53a8f0196dee3849a3ab02897c1995c3e3f6ca0c4a776c3ae869d1ddbb473090054be2400ad06d7910aa2c5d1780220fdf3765a3c1764bce10c6fe66a5a2be51a422e878518bd750424bb56b8a0ecf0f8ad2057e83f"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"76a1a261f463d1530f1db90d412c0e68a1516aa740cb39b977c4bbe8aafc9afe"}},{"name":"role","value":{"type":"UInt8","value":"3"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000002"}},{"name":"networkingKey","value":{"type":"String","value":"d64318ba0dbf68f3788fc81c41d507c5822bf53154530673127c66f50fe4469ccf1a054a868a9f88506a8999f2386d86fcd2b901779718cba4fb53c2da258f9e"}},{"name":"stakingKey","value":{"type":"String","value":"880b162b7ec138b36af401d07868cb08d25746d905395edbb4625bdf105d4bb2b2f4b0f4ae273a296a6efefa7ce9ccb914e39947ce0e83745125cab05d62516076ff0173ed472d3791ccef937597c9ea12381d76f547a092a4981d77ff3fba83"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowIDTableStaking.NodeInfo","fields":[{"name":"id","value":{"type":"String","value":"e81b23c32d05b038f67eb90a0538b45242e43bcc56530554b7b3f3d354b08f2e"}},{"name":"role","value":{"type":"UInt8","value":"4"}},{"name":"networkingAddress","value":{"type":"String","value":"00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003"}},{"name":"networkingKey","value":{"type":"String","value":"697241208dcc9142b6f53064adc8ff1c95760c68beb2ba083c1d005d40181fd7a1b113274e0163c053a3addd47cd528ec6a1f190cf465aac87c415feaae011ae"}},{"name":"stakingKey","value":{"type":"String","value":"b1f97d0a06020eca97352e1adde72270ee713c7daf58da7e74bf72235321048b4841bdfc28227964bf18e371e266e32107d238358848bcc5d0977a0db4bda0b4c33d3874ff991e595e0f537c7b87b4ddce92038ebc7b295c9ea20a1492302aa7"}},{"name":"tokensStaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensCommitted","value":{"type":"UFix64","value":"1350000.00000000"}},{"name":"tokensUnstaking","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensUnstaked","value":{"type":"UFix64","value":"0.00000000"}},{"name":"tokensRewarded","value":{"type":"UFix64","value":"0.00000000"}},{"name":"delegators","value":{"type":"Array","value":[]}},{"name":"delegatorIDCounter","value":{"type":"UInt32","value":"0"}},{"name":"tokensRequestedToUnstake","value":{"type":"UFix64","value":"0.00000000"}},{"name":"initialWeight","value":{"type":"UInt64","value":"100"}}]}}]}},{"name":"firstView","value":{"type":"UInt64","value":"70"}},{"name":"finalView","value":{"type":"UInt64","value":"139"}},{"name":"collectorClusters","value":{"type":"Array","value":[{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"0"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[{"key":{"type":"String","value":"e9ffa1712474abc92c48e209051f335b1d9b29226f8a9471dabcaa53f62d3746"},"value":{"type":"UInt64","value":"100"}}]}},{"name":"totalWeight","value":{"type":"UInt64","value":"100"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"1"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"2"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.Cluster","fields":[{"name":"index","value":{"type":"UInt16","value":"3"}},{"name":"nodeWeights","value":{"type":"Dictionary","value":[]}},{"name":"totalWeight","value":{"type":"UInt64","value":"0"}},{"name":"votes","value":{"type":"Array","value":[]}}]}}]}},{"name":"randomSource","value":{"type":"String","value":"15800823147682539441"}},{"name":"DKGPhase1FinalView","value":{"type":"UInt64","value":"121"}},{"name":"DKGPhase2FinalView","value":{"type":"UInt64","value":"123"}},{"name":"DKGPhase3FinalView","value":{"type":"UInt64","value":"125"}}]}}`)

		// decode JSON into cadence.Value
		decoded, err := jsoncdc.Decode(fixture.Payload)
		require.NoError(t, err)

		// cast to cadence.Event for parsing
		expected := decoded.(cadence.Event)

		// convert Cadence types to Go types
		event, err := flow.ConvertServiceEvent(fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

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

		// TODO: check that participants are matching
	})

	t.Run("epoch commit", func(t *testing.T) {

		fixture := unittest.EventFixture(flow.EventEpochCommit, uint32(1), uint32(1), unittest.IdentifierFixture())

		// Note: this JSON was taken from https://github.com/onflow/flow-core-contracts/ tests
		// with 4 clusters who have 1 node each
		fixture.Payload = []byte(`{"type":"Event","value":{"id":"A.01cf0e2f2f715450.FlowEpoch.EpochCommitted","fields":[{"name":"counter","value":{"type":"UInt64","value":"1"}},{"name":"clusterQCs","value":{"type":"Array","value":[{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.ClusterQC","fields":[{"name":"index","value":{"type":"UInt16","value":"0"}},{"name":"votes","value":{"type":"Array","value":[{"type":"String","value":"e9ffa1712474abc92c48e209051f335b1d9b29226f8a9471dabcaa53f62d3746"}]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.ClusterQC","fields":[{"name":"index","value":{"type":"UInt16","value":"1"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.ClusterQC","fields":[{"name":"index","value":{"type":"UInt16","value":"2"}},{"name":"votes","value":{"type":"Array","value":[]}}]}},{"type":"Struct","value":{"id":"A.01cf0e2f2f715450.FlowEpochClusterQC.ClusterQC","fields":[{"name":"index","value":{"type":"UInt16","value":"3"}},{"name":"votes","value":{"type":"Array","value":[]}}]}}]}},{"name":"dkgPubKeys","value":{"type":"Array","value":[{"type":"String","value":"8c588266db5f5cda629e83f8aa04ae9413593fac19e4865d06d291c9d14fbdd9bdb86a7a12f9ef8590c79cb635e3163315d193087e9336092987150d0cd2b14ac6365f7dc93eec573752108b8c12368abb65f0652d9f644e5aed611c37926950"},{"type":"String","value":"87a339e4e5c74f089da20a33f515d8c8f4464ab53ede5a74aa2432cd1ae66d522da0c122249ee176cd747ddc83ca81090498389384201614caf51eac392c1c0a916dfdcfbbdf7363f9552b6468434add3d3f6dc91a92bbe3ee368b59b7828488"}]}}]}}`)

		// decode JSON into cadence.Value
		decoded, err := jsoncdc.Decode(fixture.Payload)
		require.NoError(t, err)

		// cast to cadence.Event for parsing
		expected := decoded.(cadence.Event)

		// convert Cadence types to Go types
		event, err := flow.ConvertServiceEvent(fixture)
		require.NoError(t, err)
		require.NotNil(t, event)

		// cast event type to epoch setup
		actual := event.Event.(*flow.EpochCommit)

		// check if converted values same as expected
		testassert.Equal(t, actual.Counter, uint64(expected.Fields[0].(cadence.UInt64)))

		// valide the group public key maches
		groupPubKeyString := string(expected.Fields[2].(cadence.Array).Values[0].(cadence.String))
		groupKeybytes, err := hex.DecodeString(groupPubKeyString)
		require.NoError(t, err)

		groupPubKey, err := crypto.DecodePublicKey(crypto.BLSBLS12381, groupKeybytes)
		require.NoError(t, err)

		testassert.Equal(t, actual.DKGGroupKey, groupPubKey)

		// TODO: Validate individual keys match
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

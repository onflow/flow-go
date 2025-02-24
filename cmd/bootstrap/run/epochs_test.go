package run

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGenerateRecoverTxArgsWithDKG_ExcludeIncludeParticipants tests that GenerateRecoverTxArgsWithDKG produces expected arguments
// for the recover epoch transaction, when excluding and including participants recovery epoch participants.
// This test uses fuzzy testing to generate random combinations of participants to exclude and include and checks that the
// generated arguments match the expected output.
// This test assumes that we include nodes that are not part of the protocol state and exclude nodes that are part of the protocol state.
// This test also verifies that the DKG index map contains all consensus nodes despite the exclusion and inclusion filters.
func TestGenerateRecoverTxArgsWithDKG_ExcludeIncludeParticipants(testifyT *testing.T) {
	utils.RunWithSporkBootstrapDir(testifyT, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {
		log := unittest.Logger()
		internalNodes, err := common.ReadFullInternalNodeInfos(log, internalPrivDir, configPath)
		require.NoError(testifyT, err)
		partnerNodes, err := common.ReadFullPartnerNodeInfos(log, partnerWeights, partnerDir)
		require.NoError(testifyT, err)

		allNodeIds := make(flow.IdentityList, 0)
		for _, node := range append(internalNodes, partnerNodes...) {
			allNodeIds = append(allNodeIds, node.Identity())
		}

		rootSnapshot := unittest.RootSnapshotFixture(allNodeIds)
		allIdentities, err := rootSnapshot.Identities(filter.Any)
		require.NoError(testifyT, err)

		rapid.Check(testifyT, func(t *rapid.T) {
			numberOfNodesToInclude := rapid.IntRange(0, 3).Draw(t, "nodes-to-include")
			numberOfNodesToExclude := rapid.UintRange(0, 3).Draw(t, "nodes-to-exclude")

			// we specifically omit collection nodes from the exclusion list since we have a specific
			// check that there must be a valid cluster of collection nodes.
			excludedNodes, err := allIdentities.Filter(
				filter.Not(filter.HasRole[flow.Identity](flow.RoleCollection))).Sample(numberOfNodesToExclude)
			require.NoError(t, err)
			excludeNodeIds := excludedNodes.NodeIDs()
			// an eligible participant is a current epoch participant with a weight greater than zero that has not been explicitly excluded
			eligibleEpochIdentities := allIdentities.Filter(filter.And(
				filter.IsValidCurrentEpochParticipant,
				filter.HasWeightGreaterThanZero[flow.Identity],
				filter.Not(filter.HasNodeID[flow.Identity](excludeNodeIds...))))

			expectedNodeIds := make(map[cadence.String]struct{})
			includeNodeIds := unittest.IdentifierListFixture(numberOfNodesToInclude)
			for _, nodeID := range eligibleEpochIdentities.NodeIDs().Union(includeNodeIds) {
				expectedNodeIds[cadence.String(nodeID.String())] = struct{}{}
			}

			epochProtocolState, err := rootSnapshot.EpochProtocolState()
			require.NoError(t, err)
			currentEpochCommit := epochProtocolState.EpochCommit()
			expectedDKGIndexMap := make(map[cadence.String]cadence.Int)
			for nodeID, index := range currentEpochCommit.DKGIndexMap {
				expectedDKGIndexMap[cadence.String(nodeID.String())] = cadence.NewInt(index)
			}

			args, err := GenerateRecoverTxArgsWithDKG(
				log,
				internalNodes,
				2, // number of collection clusters
				currentEpochCommit.Counter+1,
				flow.Localnet,
				100,   // staking auction length, in views
				4000,  // recovery epoch length, in views
				60*60, // recovery epoch duration, in seconds
				false, // unsafe overwrite
				currentEpochCommit.DKGIndexMap,
				currentEpochCommit.DKGParticipantKeys,
				currentEpochCommit.DKGGroupKey,
				excludeNodeIds,
				includeNodeIds,
				rootSnapshot,
			)
			require.NoError(t, err)

			// dkg index map
			dkgIndexMapArgPairs := args[10].(cadence.Dictionary).Pairs
			assert.Equal(t, len(dkgIndexMapArgPairs), len(expectedDKGIndexMap))
			for _, pair := range dkgIndexMapArgPairs {
				expectedIndex, ok := expectedDKGIndexMap[pair.Key.(cadence.String)]
				require.True(t, ok)
				require.Equal(t, expectedIndex, pair.Value.(cadence.Int))
			}
			// node ids
			nodeIDsArgValues := args[11].(cadence.Array).Values
			assert.Equal(t, len(nodeIDsArgValues), len(expectedNodeIds))
			for _, nodeId := range nodeIDsArgValues {
				_, ok := expectedNodeIds[nodeId.(cadence.String)]
				require.True(t, ok)
			}
		})
	})
}

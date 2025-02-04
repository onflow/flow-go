package run

import (
	"github.com/onflow/cadence"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
	"testing"
)

func TestGenerateRecoverEpochTxArgs_ExcludeIncludeParticipants(testifyT *testing.T) {
	// tests that given the root snapshot, the command
	// writes the expected arguments to stdout.
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
			includeNodeIds := unittest.IdentifierListFixture(numberOfNodesToInclude)
			expectedNodeIds := make(map[cadence.String]struct{})
			expectedDKGIndexMap := make(map[cadence.String]cadence.Int)

			excludedNodes, err := allIdentities.Filter(
				filter.Not(filter.HasRole[flow.Identity](flow.RoleCollection))).Sample(numberOfNodesToExclude)
			require.NoError(t, err)
			excludeNodeIds := excludedNodes.NodeIDs()

			eligibleEpochIdentities := allIdentities.Filter(filter.And(
				filter.IsValidCurrentEpochParticipant,
				filter.HasWeightGreaterThanZero[flow.Identity],
				filter.Not(filter.HasNodeID[flow.Identity](excludeNodeIds...))))

			for _, nodeID := range eligibleEpochIdentities.NodeIDs().Union(includeNodeIds) {
				expectedNodeIds[cadence.String(nodeID.String())] = struct{}{}
			}

			for index, consensusNode := range allIdentities.Filter(filter.HasRole[flow.Identity](flow.RoleConsensus)) {
				expectedDKGIndexMap[cadence.String(consensusNode.NodeID.String())] = cadence.NewInt(index)
			}

			args, err := GenerateRecoverEpochTxArgs(
				log,
				internalPrivDir,
				configPath,
				2,
				2,
				flow.Localnet,
				100,
				4000,
				60*60,
				false,
				excludeNodeIds,
				includeNodeIds,
				rootSnapshot,
			)
			require.NoError(t, err)

			// dkg index map
			for _, pair := range args[10].(cadence.Dictionary).Pairs {
				expectedIndex, ok := expectedDKGIndexMap[pair.Key.(cadence.String)]
				require.True(t, ok)
				require.Equal(t, expectedIndex, pair.Value.(cadence.Int))
			}
			// node ids
			for _, nodeId := range args[11].(cadence.Array).Values {
				_, ok := expectedNodeIds[nodeId.(cadence.String)]
				require.True(t, ok)
			}
		})
	})
}

package cmd

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestRecoverEpochHappyPath ensures recover epoch transaction arguments are generated as expected.
func TestRecoverEpochHappyPath(t *testing.T) {
	// tests that given the root snapshot, the command
	// writes the expected arguments to stdout.
	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {
		internalNodes, err := common.ReadFullInternalNodeInfos(log, internalPrivDir, configPath)
		require.NoError(t, err)
		partnerNodes, err := common.ReadFullPartnerNodeInfos(log, partnerWeights, partnerDir)
		require.NoError(t, err)

		allNodeIds := make(flow.IdentityList, 0)
		allNodeIdsCdc := make(map[cadence.String]*flow.Identity)
		for _, node := range append(internalNodes, partnerNodes...) {
			allNodeIds = append(allNodeIds, node.Identity())
			allNodeIdsCdc[cadence.String(node.Identity().NodeID.String())] = node.Identity()
		}

		// create a root snapshot
		rootSnapshot := unittest.RootSnapshotFixture(allNodeIds)
		snapshotFn := func() *inmem.Snapshot { return rootSnapshot }

		// get expected dkg information
		currentEpochDKG, err := rootSnapshot.Epochs().Current().DKG()
		require.NoError(t, err)
		expectedDKGPubKeys := make(map[cadence.String]struct{})
		expectedDKGGroupKey := cadence.String(hex.EncodeToString(currentEpochDKG.GroupKey().Encode()))
		for _, id := range allNodeIds {
			if id.GetRole() == flow.RoleConsensus {
				dkgPubKey, keyShareErr := currentEpochDKG.KeyShare(id.GetNodeID())
				require.NoError(t, keyShareErr)
				expectedDKGPubKeys[cadence.String(hex.EncodeToString(dkgPubKey.Encode()))] = struct{}{}
			}
		}

		// run command with overwritten stdout
		stdout := bytes.NewBuffer(nil)
		generateRecoverEpochTxArgsCmd.SetOut(stdout)

		flagInternalNodePrivInfoDir = internalPrivDir
		flagNodeConfigJson = configPath
		flagCollectionClusters = 2
		flagEpochCounter = 2
		flagRootChainID = flow.Localnet.String()
		flagNumViewsInStakingAuction = 100
		flagNumViewsInEpoch = 4000

		generateRecoverEpochTxArgs(snapshotFn)(generateRecoverEpochTxArgsCmd, nil)

		// read output from stdout
		var outputTxArgs []interface{}
		err = json.NewDecoder(stdout).Decode(&outputTxArgs)
		require.NoError(t, err)

		// verify each argument
		decodedValues := unittest.InterfafceToCdcValues(t, outputTxArgs)
		currEpoch := rootSnapshot.Epochs().Current()
		finalView, err := currEpoch.FinalView()
		require.NoError(t, err)
		currEpochTargetEndTime, err := currEpoch.TargetEndTime()
		require.NoError(t, err)

		// epoch counter
		require.Equal(t, cadence.NewUInt64(flagEpochCounter), decodedValues[0])
		// epoch start view
		require.Equal(t, cadence.NewUInt64(finalView+1), decodedValues[1])
		// staking phase end view
		require.Equal(t, cadence.NewUInt64(finalView+flagNumViewsInStakingAuction), decodedValues[2])
		// epoch end view
		require.Equal(t, cadence.NewUInt64(finalView+flagNumViewsInEpoch), decodedValues[3])
		// target duration
		require.Equal(t, cadence.NewUInt64(flagRecoveryEpochTargetDuration), decodedValues[4])
		// target end time
		require.Equal(t, cadence.NewUInt64(currEpochTargetEndTime+flagRecoveryEpochTargetDuration), decodedValues[5])
		// clusters: we cannot guarantee order of the cluster when we generate the test fixtures
		// so, we ensure each cluster member is part of the full set of node ids
		for _, cluster := range decodedValues[6].(cadence.Array).Values {
			for _, nodeId := range cluster.(cadence.Array).Values {
				_, ok := allNodeIdsCdc[nodeId.(cadence.String)]
				require.True(t, ok)
			}
		}
		// qcVoteData: we cannot guarantee order of the cluster when we generate the test fixtures
		// so, we ensure each voter id that participated in a qc vote exists and is a collection node
		for _, voteData := range decodedValues[7].(cadence.Array).Values {
			fields := cadence.FieldsMappedByName(voteData.(cadence.Struct))
			for _, voterId := range fields["voterIDs"].(cadence.Array).Values {
				id, ok := allNodeIdsCdc[voterId.(cadence.String)]
				require.True(t, ok)
				require.Equal(t, flow.RoleCollection, id.Role)
			}
		}
		// dkg pub keys
		for _, dkgPubKey := range decodedValues[8].(cadence.Array).Values {
			_, ok := expectedDKGPubKeys[dkgPubKey.(cadence.String)]
			require.True(t, ok)
		}
		// dkg group key
		require.Equal(t, expectedDKGGroupKey, decodedValues[9].(cadence.String))
		// dkg index map
		for _, pair := range decodedValues[10].(cadence.Dictionary).Pairs {
			_, ok := allNodeIdsCdc[pair.Key.(cadence.String)]
			require.True(t, ok)
		}
		// node ids
		for _, nodeId := range decodedValues[11].(cadence.Array).Values {
			_, ok := allNodeIdsCdc[nodeId.(cadence.String)]
			require.True(t, ok)
		}
		// unsafeAllowOverWrite
		require.Equal(t, cadence.NewBool(false), decodedValues[12])
	})
}

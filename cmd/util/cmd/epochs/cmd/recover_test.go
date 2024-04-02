package cmd

import (
	"bytes"
	"encoding/json"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
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
		for _, node := range internalNodes {
			allNodeIds = append(allNodeIds, node.Identity())
		}
		for _, node := range partnerNodes {
			allNodeIds = append(allNodeIds, node.Identity())
		}

		// create a root snapshot
		rootSnapshot := unittest.RootSnapshotFixture(allNodeIds)

		snapshotFn := func() *inmem.Snapshot { return rootSnapshot }

		// run command with overwritten stdout
		stdout := bytes.NewBuffer(nil)
		generateRecoverEpochTxArgsCmd.SetOut(stdout)

		flagInternalNodePrivInfoDir = internalPrivDir
		flagNodeConfigJson = configPath
		flagCollectionClusters = 2
		flagNumViewsInEpoch = 4000
		flagNumViewsInStakingAuction = 100
		flagEpochCounter = 2

		generateRecoverEpochTxArgs(snapshotFn)(generateRecoverEpochTxArgsCmd, nil)

		// read output from stdout
		var outputTxArgs []interface{}
		err = json.NewDecoder(stdout).Decode(&outputTxArgs)
		require.NoError(t, err)
		// compare to expected values
		expectedArgs := extractRecoverEpochArgs(rootSnapshot)
		unittest.VerifyCdcArguments(t, expectedArgs[:len(expectedArgs)-1], outputTxArgs[:len(expectedArgs)-1])
		// @TODO validate cadence values for generated cluster assignments and clusters
	})
}

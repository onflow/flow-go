package cmd

import (
	"bytes"
	"encoding/json"
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
		// create a root snapshot
		rootSnapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))

		snapshotFn := func() *inmem.Snapshot { return rootSnapshot }

		// run command with overwritten stdout
		stdout := bytes.NewBuffer(nil)
		generateRecoverEpochTxArgsCmd.SetOut(stdout)

		flagPartnerWeights = partnerWeights
		flagPartnerNodeInfoDir = partnerDir
		flagInternalNodePrivInfoDir = internalPrivDir
		flagNodeConfigJson = configPath
		flagCollectionClusters = 2
		flagStartView = 1000
		flagStakingEndView = 2000
		flagEndView = 4000

		generateRecoverEpochTxArgs(snapshotFn)(generateRecoverEpochTxArgsCmd, nil)

		// read output from stdout
		var outputTxArgs []interface{}
		err := json.NewDecoder(stdout).Decode(&outputTxArgs)
		require.NoError(t, err)
		// compare to expected values
		expectedArgs := extractRecoverEpochArgs(rootSnapshot)

		unittest.VerifyCdcArguments(t, expectedArgs, outputTxArgs)
	})
}

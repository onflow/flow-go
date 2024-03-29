package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestReset_LocalSnapshot tests the command with a local snapshot file.
func TestReset_LocalSnapshot(t *testing.T) {

	// tests that given the root snapshot file and no payout, the command
	// writes the expected arguments to stdout.
	t.Run("happy path", func(t *testing.T) {

		unittest.RunWithTempDir(t, func(bootDir string) {

			// create a root snapshot
			rootSnapshot := unittest.RootSnapshotFixture(unittest.IdentityListFixture(10, unittest.WithAllRoles()))

			// write snapshot to correct path in bootDir
			err := writeRootSnapshot(bootDir, rootSnapshot)
			require.NoError(t, err)

			// set initial flag values
			flagBootDir = bootDir

			// run command with overwritten stdout
			stdout := bytes.NewBuffer(nil)
			resetCmd.SetOut(stdout)
			resetRun(resetCmd, nil)

			// read output from stdout
			var outputTxArgs []interface{}
			err = json.NewDecoder(stdout).Decode(&outputTxArgs)
			require.NoError(t, err)

			// compare to expected values
			expectedArgs := extractResetEpochArgs(rootSnapshot)
			unittest.VerifyCdcArguments(t, expectedArgs, outputTxArgs)
		})
	})

	// tests that without the root snapshot file in the bootstrap
	// dir, the command exits and does not output a args json file
	t.Run("missing snapshot file", func(t *testing.T) {
		unittest.RunWithTempDir(t, func(bootDir string) {

			var hook unittest.LoggerHook
			log, hook = unittest.HookedLogger()

			// set initial flag values
			flagBootDir = bootDir

			// run command
			resetRun(resetCmd, nil)

			assert.Regexp(t, "failed to retrieve root snapshot from local bootstrap directory", hook.Logs())
		})
	})
}

// TestReset_BucketSnapshot tests generating resetEpoch arguments using a
// root snapshot downloaded from GCP.
func TestReset_BucketSnapshot(t *testing.T) {
	// this test is skipped, as it requires an internet connection
	unittest.SkipUnless(t, unittest.TEST_REQUIRES_GCP_ACCESS, "requires an internet connection")

	// should output tx arguments to stdout
	t.Run("happy path", func(t *testing.T) {
		// set initial flag values
		flagBucketNetworkName = "mainnet-13"

		// run command with overwritten stdout
		stdout := bytes.NewBuffer(nil)
		resetCmd.SetOut(stdout)
		resetRun(resetCmd, nil)

		// read output from stdout
		var outputTxArgs []interface{}
		err := json.NewDecoder(stdout).Decode(&outputTxArgs)
		require.NoError(t, err)

		// compare to expected values
		rootSnapshot, err := getSnapshotFromBucket(fmt.Sprintf(rootSnapshotBucketURL, flagBucketNetworkName))
		require.NoError(t, err)
		expectedArgs := extractResetEpochArgs(rootSnapshot)
		unittest.VerifyCdcArguments(t, expectedArgs, outputTxArgs)
	})

	// should output arguments to stdout, including specified payout
	t.Run("happy path - with payout", func(t *testing.T) {
		// set initial flag values
		flagBucketNetworkName = "mainnet-13"

		// run command with overwritten stdout
		stdout := bytes.NewBuffer(nil)
		resetCmd.SetOut(stdout)
		resetRun(resetCmd, nil)

		// read output from stdout
		var outputTxArgs []interface{}
		err := json.NewDecoder(stdout).Decode(&outputTxArgs)
		require.NoError(t, err)

		// compare to expected values
		rootSnapshot, err := getSnapshotFromBucket(fmt.Sprintf(rootSnapshotBucketURL, flagBucketNetworkName))
		require.NoError(t, err)
		expectedArgs := extractResetEpochArgs(rootSnapshot)
		unittest.VerifyCdcArguments(t, expectedArgs, outputTxArgs)
	})

	// with a missing snapshot, should log an error
	t.Run("missing snapshot", func(t *testing.T) {

		var hook unittest.LoggerHook
		log, hook = unittest.HookedLogger()

		// set initial flag values
		flagBucketNetworkName = "not-a-real-network-name"

		// run command
		resetRun(resetCmd, nil)

		assert.Regexp(t, "failed to retrieve root snapshot from bucket", hook.Logs())
	})
}

func writeRootSnapshot(bootDir string, snapshot *inmem.Snapshot) error {
	rootSnapshotPath := filepath.Join(bootDir, bootstrap.PathRootProtocolStateSnapshot)
	return writeJSON(rootSnapshotPath, snapshot.Encodable())
}

// TODO: unify methods from all commands
// TODO: move this to common module
func writeJSON(path string, data interface{}) error {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, bz, 0644)
	if err != nil {
		return err
	}

	return nil
}

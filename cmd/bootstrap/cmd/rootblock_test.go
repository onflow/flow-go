package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/unittest"
)

const rootBlockHappyPathLogs = "collecting partner network and staking keys" +
	`read \d+ partner node configuration files` +
	`read \d+ weights for partner nodes` +
	"generating internal private networking and staking keys" +
	`read \d+ internal private node-info files` +
	`read internal node configurations` +
	`read \d+ weights for internal nodes` +
	`remove internal partner nodes` +
	`removed 0 internal partner nodes` +
	`checking constraints on consensus nodes` +
	`assembling network and staking keys` +
	`running DKG for consensus nodes` +
	`read \d+ node infos for DKG` +
	`will run DKG` +
	`finished running DKG` +
	`.+/random-beacon.priv.json` +
	`wrote file \S+/root-dkg-data.priv.json` +
	`reading votes for collection node cluster root blocks` +
	`read vote .+` +
	`constructing root blocks for collection node clusters` +
	`constructing root QCs for collection node clusters` +
	`producing QC for cluster .*` +
	`producing QC for cluster .*` +
	`constructing root header` +
	`constructing intermediary bootstrapping data` +
	`wrote file \S+/intermediary-bootstrapping-data.json` +
	`constructing root block` +
	`wrote file \S+/root-block.json` +
	`constructing and writing votes` +
	`wrote file \S+/root-block-vote.\S+.json`

var rootBlockHappyPathRegex = regexp.MustCompile(rootBlockHappyPathLogs)

// setupHappyPathFlags sets up all required flags for the root block happy path test.
func setupHappyPathFlags(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {
	rootParent := unittest.StateCommitmentFixture()
	flagOutdir = bootDir
	flagConfig = configPath
	flagPartnerNodeInfoDir = partnerDir
	flagPartnerWeights = partnerWeights
	flagInternalNodePrivInfoDir = internalPrivDir

	flagIntermediaryClusteringDataPath = filepath.Join(bootDir, model.PathClusteringData)
	flagRootClusterBlockVotesDir = filepath.Join(bootDir, model.DirnameRootBlockVotes)
	flagEpochCounter = 0

	flagRootParent = hex.EncodeToString(rootParent[:])
	flagRootChain = "main"
	flagRootHeight = 12332
	flagRootView = 1000
	flagNumViewsInEpoch = 100_000
	flagNumViewsInStakingAuction = 50_000
	flagNumViewsInDKGPhase = 2_000
	flagFinalizationSafetyThreshold = 1_000
	flagUseDefaultEpochTargetEndTime = true
	flagEpochTimingRefCounter = 0
	flagEpochTimingRefTimestamp = 0
	flagEpochTimingDuration = 0
}

// TestRootBlock_HappyPath verifies that the rootBlock function
// completes successfully with valid arguments and outputs
// logs matching the expected pattern.
func TestRootBlock_HappyPath(t *testing.T) {
	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {
		setupHappyPathFlags(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath)

		// clusterAssignment will generate the collector clusters
		// In addition, it also generates votes from internal collector nodes
		clusterAssignment(clusterAssignmentCmd, nil)

		// KV store values (epoch extension view count and finalization safety threshold) must be explicitly set for mainnet
		require.NoError(t, rootBlockCmd.Flags().Set("kvstore-finalization-safety-threshold", "1000"))
		require.NoError(t, rootBlockCmd.Flags().Set("kvstore-epoch-extension-view-count", "100000"))

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		rootBlock(rootBlockCmd, nil)
		require.Regexp(t, rootBlockHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		rootBlockDataPath := filepath.Join(bootDir, model.PathRootBlockData)
		require.FileExists(t, rootBlockDataPath)
	})
}

// TestInvalidRootBlockView verifies that running
// rootBlock with an invalid root view (0) on "main" or "testnet" chains.
// The test runs in subprocesses because the tested code calls os.Exit.
func TestInvalidRootBlockView(t *testing.T) {
	for _, chain := range []string{"main", "test"} {
		t.Run("invalid root block view for "+chain, func(t *testing.T) {
			expectedError := fmt.Sprintf("--root-view must be non-zero on %q chain", chain)
			extraEnv := []string{"CHAIN=" + chain}

			runTestInSubprocessWithError(
				t,
				"TestInvalidRootBlockViewSubprocess",
				expectedError,
				extraEnv,
			)
		})
	}
}

// TestInvalidRootBlockViewSubprocess runs in subprocess for invalid root view test on various chains
// This test only runs when invoked by TestInvalidRootBlockView as a sub-process.
func TestInvalidRootBlockViewSubprocess(t *testing.T) {
	invalidRootBlockSubprocess(t, func() {
		flagRootView = 0
		flagRootChain = os.Getenv("CHAIN")
	})
}

// TestInvalidKVStoreValues verifies that running
// rootBlock with an invalid kvstore values (default) on "main" or "testnet" chains.
// The test runs in subprocesses because the tested code calls os.Exit.
func TestInvalidKVStoreValues(t *testing.T) {
	for _, chain := range []string{"main", "test"} {
		t.Run("invalid kv store values for "+chain, func(t *testing.T) {
			expectedError := fmt.Sprintf("KV store values (epoch extension view count and finalization safety threshold) must be explicitly set on the %q chain", chain)
			extraEnv := []string{"CHAIN=" + chain}

			runTestInSubprocessWithError(
				t,
				"TestInvalidKVStoreValuesSubprocess",
				expectedError,
				extraEnv,
			)
		})
	}
}

// TestInvalidKVStoreValuesSubprocess runs in subprocess for invalid kvstore values test on various chains
func TestInvalidKVStoreValuesSubprocess(t *testing.T) {
	invalidRootBlockSubprocess(t, func() {
		flagRootChain = os.Getenv("CHAIN")
	})
}

// invalidRootBlockSubprocess is a reusable helper that runs rootBlock() with setup and logger hook,
// allowing the caller to override flags via the flagsModifier.
func invalidRootBlockSubprocess(t *testing.T, flagsModifier func()) {
	if os.Getenv("FLAG_RUN_IN_SUBPROCESS_ONLY") != "1" {
		return
	}

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {
		setupHappyPathFlags(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath)

		// Allow customization of flags before running rootBlock
		if flagsModifier != nil {
			flagsModifier()
		}

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		rootBlock(rootBlockCmd, nil)
	})
}

// runTestInSubprocessWithError executes a test function in a subprocess,
// expecting it to fail with an error. It is used for testing code paths
// that call os.Exit.
func runTestInSubprocessWithError(t *testing.T, testName, expectedOutput string, extraEnv []string) {
	cmd := exec.Command(os.Args[0], "-test.run="+testName)

	env := append(os.Environ(), "FLAG_RUN_IN_SUBPROCESS_ONLY=1")
	env = append(env, extraEnv...)
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(output), expectedOutput)
}

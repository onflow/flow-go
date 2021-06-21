package cmd

import (
	"encoding/hex"
	"os"
	"regexp"

	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utils "github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

const finalizeHappyPathLogs = "^deterministic bootstrapping random seed" +
	"collecting partner network and staking keys" +
	`read \d+ partner node configuration files` +
	`read \d+ stakes for partner nodes` +
	"generating internal private networking and staking keys" +
	`read \d+ internal private node-info files` +
	`read internal node configurations` +
	`read \d+ stakes for internal nodes` +
	`checking constraints on consensus/cluster nodes` +
	`assembling network and staking keys` +
	`wrote file \S+/node-infos.pub.json` +
	`running DKG for consensus nodes` +
	`read \d+ node infos for DKG` +
	`will run DKG` +
	`finished running DKG` +
	`.+/random-beacon.priv.json` +
	`constructing root block` +
	`constructing root QC` +
	`computing collection node clusters` +
	`constructing root blocks for collection node clusters` +
	`constructing root QCs for collection node clusters` +
	`constructing root execution result and block seal` +
	`constructing root procotol snapshot` +
	`wrote file \S+/root-protocol-state-snapshot.json` +
	`saved result and seal are matching` +
	`attempting to copy private key files` +
	`skipping copy of private keys to output dir` +
	`created keys for \d+ consensus nodes` +
	`created keys for \d+ collection nodes` +
	`created keys for \d+ verification nodes` +
	`created keys for \d+ execution nodes` +
	`created keys for \d+ access nodes` +
	"🌊 🏄 🤙 Done – ready to flow!"

var finalizeHappyPathRegex = regexp.MustCompile(finalizeHappyPathLogs)

func TestFinalize_HappyPath(t *testing.T) {
	deterministicSeed := generateRandomSeed()
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(1000)
	epochCounter := uint64(0)

	RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerStakes, internalPrivDir, configPath string) {

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerStakes = partnerStakes
		flagInternalNodePrivInfoDir = internalPrivDir

		flagFastKG = true

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootChain = chainName
		flagRootHeight = rootHeight
		flagEpochCounter = epochCounter

		// set deterministic bootstrapping seed
		flagBootstrapRandomSeed = deterministicSeed

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)
	})
}

func TestFinalize_Deterministic(t *testing.T) {
	deterministicSeed := generateRandomSeed()
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(1000)
	epochCounter := uint64(0)

	RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerStakes, internalPrivDir, configPath string) {

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerStakes = partnerStakes
		flagInternalNodePrivInfoDir = internalPrivDir

		flagFastKG = true

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootChain = chainName
		flagRootHeight = rootHeight
		flagEpochCounter = epochCounter

		// set deterministic bootstrapping seed
		flagBootstrapRandomSeed = deterministicSeed

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)

		// read snapshot
		firstSnapshot := readRootProtocolSnapshot(t, bootDir)

		// delete snapshot file
		err := os.Remove(snapshotPath)
		require.NoError(t, err)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		assert.FileExists(t, snapshotPath)

		// read snapshot
		secondSnapshot := readRootProtocolSnapshot(t, bootDir)

		assert.Equal(t, firstSnapshot, secondSnapshot)
	})
}

func RunWithSporkBootstrapDir(t testing.TB, f func(bootDir, partnerDir, partnerStakes, internalPrivDir, configPath string)) {
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)

	flagOutdir = dir

	// make sure contraints are satisfied, 2/3's of con and col nodes are internal
	internalNodes := utils.GenerateNodeInfos(3, 6, 2, 1, 1)
	partnerNodes := utils.GenerateNodeInfos(1, 1, 1, 1, 1)

	partnerDir, partnerStakesPath, err := utils.WritePartnerFiles(partnerNodes, dir)
	require.NoError(t, err)

	internalPrivDir, configPath, err := utils.WriteInternalFiles(internalNodes, dir)
	require.NoError(t, err)

	f(dir, partnerDir, partnerStakesPath, internalPrivDir, configPath)
}

func readRootProtocolSnapshot(t *testing.T, bootDir string) *inmem.Snapshot {
	snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
	bz, err := io.ReadFile(snapshotPath)
	require.NoError(t, err)
	snapshot, err := convert.BytesToInmemSnapshot(bz)
	require.NoError(t, err)
	return snapshot
}

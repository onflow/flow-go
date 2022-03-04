package cmd

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utils "github.com/onflow/flow-go/cmd/bootstrap/utils"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const finalizeHappyPathLogs = "^deterministic bootstrapping random seed" +
	"collecting partner network and staking keys" +
	`read \d+ partner node configuration files` +
	`read \d+ weights for partner nodes` +
	"generating internal private networking and staking keys" +
	`read \d+ internal private node-info files` +
	`read internal node configurations` +
	`read \d+ weights for internal nodes` +
	`checking constraints on consensus/cluster nodes` +
	`assembling network and staking keys` +
	`reading root block data` +
	`reading root block votes` +
	`read vote .*` +
	`reading dkg data` +
	`constructing root QC` +
	`computing collection node clusters` +
	`constructing root blocks for collection node clusters` +
	`constructing root QCs for collection node clusters` +
	`constructing root execution result and block seal` +
	`constructing root protocol snapshot` +
	`wrote file \S+/root-protocol-state-snapshot.json` +
	`saved result and seal are matching` +
	`saved root snapshot is valid` +
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
	deterministicSeed := GenerateRandomSeed(flow.EpochSetupRandomSourceLength)
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(12332)
	epochCounter := uint64(2)

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {

		flagOutdir = bootDir

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerWeights = partnerWeights
		flagInternalNodePrivInfoDir = internalPrivDir

		flagFastKG = true
		flagRootChain = chainName
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootHeight = rootHeight

		// set deterministic bootstrapping seed
		flagBootstrapRandomSeed = deterministicSeed

		// rootBlock will generate DKG and place it into bootDir/public-root-information
		rootBlock(nil, nil)

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagEpochCounter = epochCounter
		flagRootBlock = filepath.Join(bootDir, model.PathRootBlockData)
		flagDKGDataPath = filepath.Join(bootDir, model.PathRootDKGData)
		flagRootBlockVotesDir = filepath.Join(bootDir, model.DirnameRootBlockVotes)

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		finalize(nil, nil)
		assert.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)
	})
}

func TestFinalize_Deterministic(t *testing.T) {
	deterministicSeed := GenerateRandomSeed(flow.EpochSetupRandomSourceLength)
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(1000)
	epochCounter := uint64(0)

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {

		flagOutdir = bootDir

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerWeights = partnerWeights
		flagInternalNodePrivInfoDir = internalPrivDir

		flagFastKG = true

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootChain = chainName
		flagRootHeight = rootHeight
		flagEpochCounter = epochCounter

		// set deterministic bootstrapping seed
		flagBootstrapRandomSeed = deterministicSeed

		// rootBlock will generate DKG and place it into model.PathRootDKGData
		rootBlock(nil, nil)

		flagRootBlock = filepath.Join(bootDir, model.PathRootBlockData)
		flagDKGDataPath = filepath.Join(bootDir, model.PathRootDKGData)
		flagRootBlockVotesDir = filepath.Join(bootDir, model.DirnameRootBlockVotes)

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)

		// read snapshot
		_, err := utils.ReadRootProtocolSnapshot(bootDir)
		require.NoError(t, err)

		// delete snapshot file
		err = os.Remove(snapshotPath)
		require.NoError(t, err)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		assert.FileExists(t, snapshotPath)

		// read snapshot
		_, err = utils.ReadRootProtocolSnapshot(bootDir)
		require.NoError(t, err)

		// ATTENTION: we can't use next statement because QC generation is not deterministic
		// assert.Equal(t, firstSnapshot, secondSnapshot)
		// Meaning we don't have a guarantee that with same input arguments we will get same QC.
		// This doesn't mean that QC is invalid, but it will result in different structures,
		// different QC => different service events => different result => different seal
		// We need to use a different mechanism for comparing.
		// ToDo: Revisit if this test case is valid at all.
	})
}

func TestFinalize_SameSeedDifferentStateCommits(t *testing.T) {
	deterministicSeed := GenerateRandomSeed(flow.EpochSetupRandomSourceLength)
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(1000)
	epochCounter := uint64(0)

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {

		flagOutdir = bootDir

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerWeights = partnerWeights
		flagInternalNodePrivInfoDir = internalPrivDir

		flagFastKG = true

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootChain = chainName
		flagRootHeight = rootHeight
		flagEpochCounter = epochCounter

		// set deterministic bootstrapping seed
		flagBootstrapRandomSeed = deterministicSeed

		// rootBlock will generate DKG and place it into bootDir/public-root-information
		rootBlock(nil, nil)

		flagRootBlock = filepath.Join(bootDir, model.PathRootBlockData)
		flagDKGDataPath = filepath.Join(bootDir, model.PathRootDKGData)
		flagRootBlockVotesDir = filepath.Join(bootDir, model.DirnameRootBlockVotes)

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)

		// read snapshot
		snapshot1, err := utils.ReadRootProtocolSnapshot(bootDir)
		require.NoError(t, err)

		// delete snapshot file
		err = os.Remove(snapshotPath)
		require.NoError(t, err)

		// change input state commitments
		rootCommit2 := unittest.StateCommitmentFixture()
		rootParent2 := unittest.StateCommitmentFixture()
		flagRootCommit = hex.EncodeToString(rootCommit2[:])
		flagRootParent = hex.EncodeToString(rootParent2[:])

		finalize(nil, nil)
		require.Regexp(t, finalizeHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		assert.FileExists(t, snapshotPath)

		// read snapshot
		snapshot2, err := utils.ReadRootProtocolSnapshot(bootDir)
		require.NoError(t, err)

		// current epochs
		currentEpoch1 := snapshot1.Epochs().Current()
		currentEpoch2 := snapshot2.Epochs().Current()

		// check dkg
		dkg1, err := currentEpoch1.DKG()
		require.NoError(t, err)
		dkg2, err := currentEpoch2.DKG()
		require.NoError(t, err)
		assert.Equal(t, dkg1, dkg2)

		// check clustering
		clustering1, err := currentEpoch1.Clustering()
		require.NoError(t, err)
		clustering2, err := currentEpoch2.Clustering()
		require.NoError(t, err)
		assert.Equal(t, clustering1, clustering2)

		// verify random sources are same
		randomSource1, err := currentEpoch1.RandomSource()
		require.NoError(t, err)
		randomSource2, err := currentEpoch2.RandomSource()
		require.NoError(t, err)
		assert.Equal(t, randomSource1, randomSource2)
		assert.Equal(t, randomSource1, deterministicSeed)
		assert.Equal(t, flow.EpochSetupRandomSourceLength, len(randomSource1))
	})
}

func TestFinalize_InvalidRandomSeedLength(t *testing.T) {
	rootCommit := unittest.StateCommitmentFixture()
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(12332)
	epochCounter := uint64(2)

	// set random seed with smaller length
	deterministicSeed, err := hex.DecodeString("a12354a343234aa44bbb43")
	require.NoError(t, err)

	// invalid length execution logs
	expectedLogs := regexp.MustCompile("random seed provided length is not valid")

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {

		flagOutdir = bootDir

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerWeights = partnerWeights
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
		assert.Regexp(t, expectedLogs, hook.logs.String())
		hook.logs.Reset()
	})
}

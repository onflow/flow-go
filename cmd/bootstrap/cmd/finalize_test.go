package cmd

import (
	cryptoRand "crypto/rand"
	"encoding/hex"
	"math/rand"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/cmd/util/cmd/common"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/prg"
	"github.com/onflow/flow-go/utils/unittest"
)

const finalizeHappyPathLogs = "collecting partner network and staking keys" +
	`read \d+ partner node configuration files` +
	`read \d+ weights for partner nodes` +
	"generating internal private networking and staking keys" +
	`read \d+ internal private node-info files` +
	`read internal node configurations` +
	`read \d+ weights for internal nodes` +
	`checking constraints on consensus nodes` +
	`assembling network and staking keys` +
	`reading root block data` +
	`reading root block votes` +
	`read vote .*` +
	`reading random beacon keys` +
	`reading intermediary bootstrapping data` +
	`constructing root QC` +
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

		flagIntermediaryClusteringDataPath = filepath.Join(bootDir, model.PathClusteringData)
		flagRootClusterBlockVotesDir = filepath.Join(bootDir, model.DirnameRootBlockVotes)
		flagEpochCounter = epochCounter

		// clusterAssignment will generate the collector clusters
		// In addition, it also generates votes from internal collector nodes
		clusterAssignment(clusterAssignmentCmd, nil)

		flagRootChain = chainName
		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootHeight = rootHeight
		flagRootView = 1_000
		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagNumViewsInEpoch = 100_000
		flagNumViewsInStakingAuction = 50_000
		flagNumViewsInDKGPhase = 2_000
		flagFinalizationSafetyThreshold = 1_000
		flagEpochExtensionViewCount = 100_000
		flagUseDefaultEpochTargetEndTime = true
		flagEpochTimingRefCounter = 0
		flagEpochTimingRefTimestamp = 0
		flagEpochTimingDuration = 0

		// KV store values (epoch extension view count and finalization safety threshold) must be explicitly set for mainnet
		require.NoError(t, rootBlockCmd.Flags().Set("kvstore-finalization-safety-threshold", "1000"))
		require.NoError(t, rootBlockCmd.Flags().Set("kvstore-epoch-extension-view-count", "100000"))

		// rootBlock will generate DKG and place it into bootDir/public-root-information
		rootBlock(rootBlockCmd, nil)

		flagRootBlockPath = filepath.Join(bootDir, model.PathRootBlockData)
		flagIntermediaryBootstrappingDataPath = filepath.Join(bootDir, model.PathIntermediaryBootstrappingData)
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

func TestClusterAssignment(t *testing.T) {
	tmp := flagCollectionClusters
	flagCollectionClusters = 5
	// Happy path (limit set-up, can't have one less internal node)
	partnersLen := 12
	internalLen := 10
	// clusters are assigned (partner:internal) as [3:2, 3:2, 2:2, 2:2, 2:2],
	// with internal proportion being [40%, 40%, 50%, 50%, 50%]
	// removing an internal node would lead to a 2:1 ratio (33% internal) in the last cluster
	partners := unittest.NodeInfosFixture(partnersLen, unittest.WithRole(flow.RoleCollection))
	internals := unittest.NodeInfosFixture(internalLen, unittest.WithRole(flow.RoleCollection))

	// use a random seed
	seed := make([]byte, 32)
	_, err := cryptoRand.Read(seed)
	require.NoError(t, err)
	prng, err := prg.New(seed, prg.BootstrapClusterAssignment, nil)
	require.NoError(t, err)

	log := zerolog.Nop()
	// should not error
	_, _, canConstructQCs, err := common.ConstructClusterAssignment(log, model.ToIdentityList(partners), model.ToIdentityList(internals), int(flagCollectionClusters), prng)
	require.NoError(t, err)
	require.True(t, canConstructQCs)

	// unhappy Path
	internals = internals[:len(internals)-1] // reduce one internal node
	// should no longer be able to construct QCs using only votes from internal nodes
	_, _, canConstructQCs, err = common.ConstructClusterAssignment(log, model.ToIdentityList(partners), model.ToIdentityList(internals), int(flagCollectionClusters), prng)
	require.NoError(t, err)
	require.False(t, canConstructQCs)
	// revert the flag value
	flagCollectionClusters = tmp
}

func TestEpochTimingConfig(t *testing.T) {
	// Reset flags after test is completed
	defer func(_flagDefault bool, _flagRefCounter, _flagRefTs, _flagDur uint64) {
		flagUseDefaultEpochTargetEndTime = _flagDefault
		flagEpochTimingRefCounter = _flagRefCounter
		flagEpochTimingRefTimestamp = _flagRefTs
		flagEpochTimingDuration = _flagDur
	}(flagUseDefaultEpochTargetEndTime, flagEpochTimingRefCounter, flagEpochTimingRefTimestamp, flagEpochTimingDuration)

	flags := []*uint64{&flagEpochTimingRefCounter, &flagEpochTimingRefTimestamp, &flagEpochTimingDuration}
	t.Run("if default is set, no other flag may be set", func(t *testing.T) {
		flagUseDefaultEpochTargetEndTime = true
		for _, flag := range flags {
			*flag = rand.Uint64()%100 + 1
			err := validateOrPopulateEpochTimingConfig()
			assert.Error(t, err)
			*flag = 0 // set the flag back to 0
		}
		err := validateOrPopulateEpochTimingConfig()
		assert.NoError(t, err)
	})

	t.Run("if default is not set, all other flags must be set", func(t *testing.T) {
		flagUseDefaultEpochTargetEndTime = false
		// First set all required flags and ensure validation passes
		flagEpochTimingRefCounter = rand.Uint64() % flagEpochCounter
		flagEpochTimingDuration = rand.Uint64()%100_000 + 1
		flagEpochTimingRefTimestamp = rand.Uint64()

		err := validateOrPopulateEpochTimingConfig()
		assert.NoError(t, err)

		// Next, check that validation fails if any one flag is not set
		// NOTE: we do not include refCounter here, because it is allowed to be zero.
		for _, flag := range []*uint64{&flagEpochTimingRefTimestamp, &flagEpochTimingDuration} {
			*flag = 0
			err := validateOrPopulateEpochTimingConfig()
			assert.Error(t, err)
			*flag = rand.Uint64()%100 + 1 // set the flag back to a non-zero value
		}
	})
}

func TestMergeNodeInfos(t *testing.T) {
	partnersLen := 7
	internalLen := 22
	partners := unittest.NodeInfosFixture(partnersLen, unittest.WithRole(flow.RoleCollection))
	internals := unittest.NodeInfosFixture(internalLen, unittest.WithRole(flow.RoleCollection))

	// Check if there is no overlap, then should pass
	merged, err := mergeNodeInfos(partners, internals)
	require.NoError(t, err)
	require.Len(t, merged, partnersLen+internalLen)

	// Check if internals and partners have overlap, then should fail
	internalAndPartnersHaveOverlap := append(partners, internals[0])
	_, err = mergeNodeInfos(internalAndPartnersHaveOverlap, internals)
	require.Error(t, err)

	// Check if partners have overlap, then should fail
	partnersHaveOverlap := append(partners, partners[0])
	_, err = mergeNodeInfos(partnersHaveOverlap, internals)
	require.Error(t, err)

	// Check if internals have overlap, then should fail
	internalsHaveOverlap := append(internals, internals[0])
	_, err = mergeNodeInfos(partners, internalsHaveOverlap)
	require.Error(t, err)
}

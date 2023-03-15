package cmd

import (
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	utils "github.com/onflow/flow-go/cmd/bootstrap/utils"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/utils/unittest"
)

const finalizeHappyPathLogs = "collecting partner network and staking keys" +
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

		// rootBlock will generate DKG and place it into bootDir/public-root-information
		rootBlock(nil, nil)

		flagRootCommit = hex.EncodeToString(rootCommit[:])
		flagEpochCounter = epochCounter
		flagNumViewsInEpoch = 100_000
		flagNumViewsInStakingAuction = 50_000
		flagNumViewsInDKGPhase = 2_000
		flagEpochCommitSafetyThreshold = 1_000
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

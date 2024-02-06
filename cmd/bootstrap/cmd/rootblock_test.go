package cmd

import (
	"encoding/hex"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

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
	`checking constraints on consensus nodes` +
	`assembling network and staking keys` +
	`wrote file \S+/node-infos.pub.json` +
	`running DKG for consensus nodes` +
	`read \d+ node infos for DKG` +
	`will run DKG` +
	`finished running DKG` +
	`.+/random-beacon.priv.json` +
	`wrote file \S+/root-dkg-data.priv.json` +
	`computing collection node clusters` +
	`constructing root blocks for collection node clusters` +
	`constructing root QCs for collection node clusters` +
	`constructing root header` +
	`constructing intermediary bootstrapping data` +
	`wrote file \S+/intermediary-bootstrapping-data.json` +
	`constructing root block` +
	`wrote file \S+/root-block.json` +
	`constructing and writing votes` +
	`wrote file \S+/root-block-vote.\S+.json`

var rootBlockHappyPathRegex = regexp.MustCompile(rootBlockHappyPathLogs)

func TestRootBlock_HappyPath(t *testing.T) {
	rootParent := unittest.StateCommitmentFixture()
	chainName := "main"
	rootHeight := uint64(12332)

	utils.RunWithSporkBootstrapDir(t, func(bootDir, partnerDir, partnerWeights, internalPrivDir, configPath string) {

		flagOutdir = bootDir

		flagConfig = configPath
		flagPartnerNodeInfoDir = partnerDir
		flagPartnerWeights = partnerWeights
		flagInternalNodePrivInfoDir = internalPrivDir

		flagRootParent = hex.EncodeToString(rootParent[:])
		flagRootChain = chainName
		flagRootHeight = rootHeight
		flagEpochCounter = 0
		flagNumViewsInEpoch = 100_000
		flagNumViewsInStakingAuction = 50_000
		flagNumViewsInDKGPhase = 2_000
		flagEpochCommitSafetyThreshold = 1_000
		flagProtocolVersion = 42

		hook := zeroLoggerHook{logs: &strings.Builder{}}
		log = log.Hook(hook)

		rootBlock(nil, nil)
		assert.Regexp(t, rootBlockHappyPathRegex, hook.logs.String())
		hook.logs.Reset()

		// check if root protocol snapshot exists
		rootBlockDataPath := filepath.Join(bootDir, model.PathRootBlockData)
		assert.FileExists(t, rootBlockDataPath)
	})
}

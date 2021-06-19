package cmd

import (
	"encoding/hex"
	"fmt"
	"os"

	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

// TODO: defina happy path logs
const finalizeHappyPathLogs = "^"

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
		// TODO: verify happy path logs
		hook.logs.Reset()
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
		// TODO: verify happy path logs
		hook.logs.Reset()

		// check if root protocol snapshot exists
		snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)
		assert.FileExists(t, snapshotPath)

		// read snapshot
		firstSnapshot := readRootProtocolSnapshot(t, bootDir)

		// delete snapshot file and generate again
		err := os.Remove(snapshotPath)
		require.NoError(t, err)

		// run finalize again
		finalize(nil, nil)
		// TODO: verify happy path logs
		hook.logs.Reset()

		// read second snapshot
		secondSnapshot := readRootProtocolSnapshot(t, bootDir)

		assert.Equal(t, firstSnapshot, secondSnapshot)
	})

}

func readRootProtocolSnapshot(t *testing.T, bootDir string) *inmem.Snapshot {

	snapshotPath := filepath.Join(bootDir, model.PathRootProtocolStateSnapshot)

	bz, err := io.ReadFile(snapshotPath)
	require.NoError(t, err)

	snapshot, err := convert.BytesToInmemSnapshot(bz)
	require.NoError(t, err)

	return snapshot
}

func RunWithSporkBootstrapDir(t testing.TB, f func(bootDir, partnerDir, partnerStakes, internalPrivDir, configPath string)) {
	dir := unittest.TempDir(t)
	defer os.RemoveAll(dir)

	flagOutdir = dir

	partnerDir, partnerStakesPath := writePartnerNodesAndStakes(t, dir)
	internalPrivDir, configPath := writeInternalNodesAndConfig(t, dir)

	f(dir, partnerDir, partnerStakesPath, internalPrivDir, configPath)
}

func writePartnerNodesAndStakes(t testing.TB, bootDir string) (string, string) {
	nodeInfos := generatePartnerNodeInfos()

	// convert to public nodeInfos
	nodePubInfos := make([]model.NodeInfoPub, len(nodeInfos))
	for i, info := range nodeInfos {
		nodePubInfos[i] = info.Public()
	}

	// create a staking map
	stakes := make(map[flow.Identifier]uint64)
	for _, node := range nodeInfos {
		stakes[node.NodeID] = node.Stake
	}

	// write node public infos to partner dir
	partnersDir := "partners"
	err := os.MkdirAll(filepath.Join(bootDir, partnersDir), os.ModePerm)
	require.NoError(t, err)

	for _, node := range nodePubInfos {
		fileName := fmt.Sprintf(model.PathNodeInfoPub, node.NodeID.String())
		nodePubInfosPath := filepath.Join(partnersDir, fileName)
		writeJSON(nodePubInfosPath, node)
	}

	// write partner stakes
	stakesPath := "partner-stakes.json"
	writeJSON(stakesPath, stakes)

	return filepath.Join(bootDir, partnersDir, model.DirnamePublicBootstrap), filepath.Join(bootDir, stakesPath)
}

func writeInternalNodesAndConfig(t testing.TB, bootDir string) (string, string) {
	nodeInfos := generateInternalNodeInfos()

	// convert to private nodeInfos
	nodePrivInfos := make([]model.NodeInfoPriv, len(nodeInfos))
	for i, node := range nodeInfos {

		netPriv, err := unittest.NetworkingKey()
		require.NoError(t, err)

		stakePriv, err := unittest.StakingKey()
		require.NoError(t, err)

		nodePrivInfos[i] = model.NodeInfoPriv{
			Role:    node.Role,
			Address: node.Address,
			NodeID:  node.NodeID,
			NetworkPrivKey: encodable.NetworkPrivKey{
				PrivateKey: netPriv,
			},
			StakingPrivKey: encodable.StakingPrivKey{
				PrivateKey: stakePriv,
			},
		}
	}

	// create internal node config
	configs := make([]model.NodeConfig, len(nodeInfos))
	for index, node := range nodeInfos {
		configs[index] = model.NodeConfig{
			Role:    node.Role,
			Address: node.Address,
			Stake:   node.Stake,
		}
	}

	// write config
	configPath := "node-internal-infos.pub.json"
	writeJSON(configPath, configs)

	// write node private infos to internal priv dir
	for _, node := range nodePrivInfos {
		internalPrivPath := fmt.Sprintf(model.PathNodeInfoPriv, node.NodeID)
		writeJSON(internalPrivPath, node)
	}

	return filepath.Join(bootDir, model.DirPrivateRoot), filepath.Join(bootDir, configPath)
}

func generateInternalNodeInfos() []model.NodeInfo {

	internalNodes := make([]model.NodeInfo, 0)

	// CONSENSUS = 3
	consensusNodes := unittest.NodeInfosFixture(3,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithStake(1000),
	)
	internalNodes = append(internalNodes, consensusNodes...)

	// COLLECTION = 6
	collectionNodes := unittest.NodeInfosFixture(6,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithStake(1000),
	)
	internalNodes = append(internalNodes, collectionNodes...)

	// EXECUTION = 2
	executionNodes := unittest.NodeInfosFixture(2,
		unittest.WithRole(flow.RoleExecution),
		unittest.WithStake(1000),
	)
	internalNodes = append(internalNodes, executionNodes...)

	// VERIFICATION = 1
	verificationNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleVerification),
		unittest.WithStake(1000),
	)
	internalNodes = append(internalNodes, verificationNodes...)

	// ACCESS = 1
	accessNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleAccess),
		unittest.WithStake(1000),
	)
	internalNodes = append(internalNodes, accessNodes...)

	return internalNodes
}

func generatePartnerNodeInfos() []model.NodeInfo {

	partnerNodes := make([]model.NodeInfo, 0)

	// CONSENSUS = 3
	consensusNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithStake(1000),
	)
	partnerNodes = append(partnerNodes, consensusNodes...)

	// COLLECTION = 6
	collectionNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleCollection),
		unittest.WithStake(1000),
	)
	partnerNodes = append(partnerNodes, collectionNodes...)

	// EXECUTION = 2
	executionNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleExecution),
		unittest.WithStake(1000),
	)
	partnerNodes = append(partnerNodes, executionNodes...)

	// VERIFICATION = 1
	verificationNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleVerification),
		unittest.WithStake(1000),
	)
	partnerNodes = append(partnerNodes, verificationNodes...)

	// ACCESS = 1
	accessNodes := unittest.NodeInfosFixture(1,
		unittest.WithRole(flow.RoleAccess),
		unittest.WithStake(1000),
	)
	partnerNodes = append(partnerNodes, accessNodes...)

	return partnerNodes
}

package testnet

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
	"github.com/onflow/flow-go/utils/unittest"
)

func makeDir(t *testing.T, base string, subdir string) string {
	dir := filepath.Join(base, subdir)
	err := os.MkdirAll(dir, 0700)
	require.NoError(t, err)
	return dir
}

// makeTempDir creates a temporary directory in TmpRoot, and deletes it after the test completes.
func makeTempDir(t *testing.T, pattern string) string {
	dir := makeTempSubDir(t, TmpRoot, pattern)
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	})
	return dir
}

// makeTempSubDir creates a randomly named subdirectory in the given directory.
func makeTempSubDir(t *testing.T, dir, pattern string) string {
	dir, err := os.MkdirTemp(dir, pattern)
	require.NoError(t, err)
	return dir
}

// currentUser returns a uid:gid Unix user identifier string for the current
// user. This is used to run node containers under the same user to avoid
// permission conflicts on files mounted from the host.
func currentUser() string {
	cur, _ := user.Current()
	return fmt.Sprintf("%s:%s", cur.Uid, cur.Gid)
}

func toParticipants(confs []ContainerConfig) flow.IdentityList {
	il := make(flow.IdentityList, 0, len(confs))
	for _, conf := range confs {
		il = append(il, conf.Identity())
	}
	return il
}

func toNodeInfos(confs []ContainerConfig) []bootstrap.NodeInfo {
	infos := make([]bootstrap.NodeInfo, 0, len(confs))
	for _, conf := range confs {
		infos = append(infos, conf.NodeInfo)
	}
	return infos
}

func getSeed() ([]byte, error) {
	seedLen := int(math.Max(crypto.KeyGenSeedMinLen, crypto.KeyGenSeedMinLen))
	seed := make([]byte, seedLen)
	n, err := rand.Read(seed)
	if err != nil || n != seedLen {
		return nil, err
	}
	return seed, nil
}

func WriteJSON(path string, data interface{}) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return err
	}

	marshaled, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return WriteFile(path, marshaled)
}

func WriteFile(path string, data []byte) error {
	err := os.WriteFile(path, data, 0644)
	return err
}

// rootProtocolJsonWithoutAddresses strips out all node addresses from the root protocol json file specified as srcFile
// and creates the dstFile with the modified contents
func rootProtocolJsonWithoutAddresses(srcfile string, dstFile string) error {

	data, err := io.ReadFile(filepath.Join(srcfile))
	if err != nil {
		return err
	}

	var rootSnapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &rootSnapshot)
	if err != nil {
		return err
	}

	strippedSnapshot := inmem.StrippedInmemSnapshot(rootSnapshot)

	return WriteJSON(dstFile, strippedSnapshot)
}

func WriteObserverPrivateKey(observerName, bootstrapDir string) (crypto.PrivateKey, error) {
	// make the observer private key for named observer
	// only used for localnet, not for use with production
	networkSeed := cmd.GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	networkKey, err := utils.GeneratePublicNetworkingKey(networkSeed)
	if err != nil {
		return nil, fmt.Errorf("could not generate networking key: %w", err)
	}

	// hex encode
	keyBytes := networkKey.Encode()
	output := make([]byte, hex.EncodedLen(len(keyBytes)))
	hex.Encode(output, keyBytes)

	// write to file
	outputFile := fmt.Sprintf("%s/private-root-information/%s_key", bootstrapDir, observerName)
	err = os.WriteFile(outputFile, output, 0600)
	if err != nil {
		return nil, fmt.Errorf("could not write private key to file: %w", err)
	}

	return networkKey, nil
}

func WriteTestExecutionService(_ flow.Identifier, address, observerName, bootstrapDir string) (bootstrap.NodeInfo, error) {
	// make the observer private key for named observer
	// only used for localnet, not for use with production
	networkSeed := cmd.GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	networkKey, err := utils.GeneratePublicNetworkingKey(networkSeed)
	if err != nil {
		return bootstrap.NodeInfo{}, fmt.Errorf("could not generate networking key: %w", err)
	}

	// hex encode
	keyBytes := networkKey.Encode()
	output := make([]byte, hex.EncodedLen(len(keyBytes)))
	hex.Encode(output, keyBytes)

	encryptionKey, err := utils.GenerateSecretsDBEncryptionKey()
	if err != nil {
		return bootstrap.NodeInfo{}, err
	}

	pubKey, err := keyutils.LibP2PPublicKeyFromFlow(networkKey.PublicKey())
	if err != nil {
		return bootstrap.NodeInfo{}, fmt.Errorf("could not get libp2p public key from flow public key: %w", err)
	}

	peerID, err := peer.IDFromPublicKey(pubKey)
	if err != nil {
		return bootstrap.NodeInfo{}, fmt.Errorf("could not get peer ID from public key: %w", err)
	}

	nodeID, err := translator.NewPublicNetworkIDTranslator().GetFlowID(peerID)
	if err != nil {
		return bootstrap.NodeInfo{}, fmt.Errorf("could not get flow node ID: %w", err)
	}

	k, err := pubKey.Raw()
	if err != nil {
		return bootstrap.NodeInfo{}, err
	}

	ks := unittest.StakingKeys(1)
	stakingKey := ks[0]

	log.Info().Msgf("test execution node private key: %v, public key: %x, peerID: %v, nodeID: %v", networkKey, k, peerID, nodeID)

	nodeInfo := bootstrap.NewPrivateNodeInfo(
		nodeID,
		flow.RoleExecution,
		address,
		0,
		networkKey,
		stakingKey,
	)

	path := fmt.Sprintf("%s/private-root-information/private-node-info_%v/%vjson",
		bootstrapDir, nodeID, bootstrap.PathPrivNodeInfoPrefix)

	private, err := nodeInfo.Private()
	if err != nil {
		return bootstrap.NodeInfo{}, err
	}

	err = io.WriteJSON(path, private)
	if err != nil {
		return bootstrap.NodeInfo{}, err
	}

	path = fmt.Sprintf("%s/private-root-information/private-node-info_%v/%v",
		bootstrapDir, nodeID, bootstrap.FilenameSecretsEncryptionKey)
	err = os.WriteFile(path, encryptionKey, 0644)
	if err != nil {
		return bootstrap.NodeInfo{}, err
	}

	// write network private key
	outputFile := fmt.Sprintf("%s/private-root-information/private-node-info_%v/network_private_key", bootstrapDir, nodeID)
	err = os.WriteFile(outputFile, output, 0600)
	if err != nil {
		return bootstrap.NodeInfo{}, fmt.Errorf("could not write private key to file: %w", err)
	}

	return nodeInfo, nil
}

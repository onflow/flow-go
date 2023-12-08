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

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/cmd/bootstrap/cmd"
	"github.com/onflow/flow-go/cmd/bootstrap/utils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
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

func WriteObserverPrivateKey(observerName, bootstrapDir string) error {
	// make the observer private key for named observer
	// only used for localnet, not for use with production
	networkSeed := cmd.GenerateRandomSeed(crypto.KeyGenSeedMinLen)
	networkKey, err := utils.GeneratePublicNetworkingKey(networkSeed)
	if err != nil {
		return fmt.Errorf("could not generate networking key: %w", err)
	}

	// hex encode
	keyBytes := networkKey.Encode()
	output := make([]byte, hex.EncodedLen(len(keyBytes)))
	hex.Encode(output, keyBytes)

	// write to file
	outputFile := fmt.Sprintf("%s/private-root-information/%s_key", bootstrapDir, observerName)
	err = os.WriteFile(outputFile, output, 0600)
	if err != nil {
		return fmt.Errorf("could not write private key to file: %w", err)
	}

	return nil
}

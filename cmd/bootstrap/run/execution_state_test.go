package run

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This tests generates a checkpoint file to be used by the execution node when booting.
func TestGenerateExecutionState(t *testing.T) {
	seed := make([]byte, 48)
	seed[0] = 1
	sk, err := GenerateServiceAccountPrivateKey(seed)
	require.NoError(t, err)

	pk := sk.PublicKey(42)
	bootstrapDir, err := ioutil.TempDir("/tmp", "flow-integration-bootstrap")
	require.NoError(t, err)
	trieDir := filepath.Join(bootstrapDir, bootstrap.DirnameExecutionState)
	commit, err := GenerateExecutionState(
		trieDir,
		pk,
		flow.Testnet.Chain(),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply))
	require.NoError(t, err)
	fmt.Printf("sk: %v\n", sk)
	fmt.Printf("pk: %v\n", pk)
	fmt.Printf("commit: %x\n", commit)
	fmt.Printf("a checkpoint file is generated at: %v\n", trieDir)
}

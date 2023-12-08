package run

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// This tests generates a checkpoint file to be used by the execution node when booting.
func TestGenerateExecutionState(t *testing.T) {
	seed := make([]byte, 48)
	seed[0] = 1
	sk, err := generateServiceAccountPrivateKey(seed)
	require.NoError(t, err)

	pk := sk.PublicKey(42)
	bootstrapDir := t.TempDir()
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

func generateServiceAccountPrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSASecp256k1,
		HashAlgo:   hash.SHA2_256,
	}, nil
}

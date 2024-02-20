package run

import (
	"encoding/hex"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
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

func TestPrintServiceKey(t *testing.T) {
	hexString := "" // Replace this with your actual hex-encoded string

	// Decode the hex string to bytes
	decodedBytes, err := hex.DecodeString(hexString)
	require.NoError(t, err)

	priv, err := crypto.DecodePrivateKey(crypto.ECDSASecp256k1, decodedBytes)
	require.NoError(t, err)

	sk := flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSASecp256k1,
		HashAlgo:   hash.SHA2_256,
	}

	pk := sk.PublicKey(1000)
	bytes, err := pk.MarshalJSON()
	require.NoError(t, err)
	fmt.Print(string(bytes))
}

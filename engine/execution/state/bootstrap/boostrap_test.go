package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisStateCommitment(t *testing.T) {
	unittest.RunWithTempDBDir(t, func(dbDir string) {

		ls, err := ledger.NewTrieStorage(dbDir)
		require.NoError(t, err)

		newStateCommitment, err := BootstrapLedger(ls)
		require.NoError(t, err)

		assert.Equal(t, flow.GenesisStateCommitment, newStateCommitment)
	})
}

func TestDecodePrivateKey(t *testing.T) {
	privateKeyBytes, err := hex.DecodeString(flow.RootAccountPrivateKeyHex)
	if err != nil {
		panic("Cannot hex decode hardcoded key!")
	}
	privateKey, err := flow.DecodeAccountPrivateKey(privateKeyBytes)
	if err != nil {
		panic("Cannot decode hardcoded private key!")
	}
	rawEncodedPrivateKey := privateKey.PrivateKey.Encode()
	// fmt.Println(rawEncodedPrivateKey)
	t.Fatalf(hex.EncodeToString(rawEncodedPrivateKey))
}

package blueprints_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
)

// TestSystemChunkTransactionHash tests the hash of the system chunk transaction hash does not change.
// We currently give no guarantees about the system transaction hash not changing block-to-block, but the community
// currently depends on the hash not changing. If the hash changes, the community should be notified in the release notes.
func TestSystemChunkTransactionHash(t *testing.T) {
	t.Parallel()

	// this is formatted in a way that the resulting error message is easy to copy-paste into the test.
	expectedHashes := []chainHash{
		{chainId: "flow-mainnet", expectedHash: "0a7ea89ad32d79a30b91b4c1202230a1e29310e1b92e01c76d036d2e3839159b"},
		{chainId: "flow-testnet", expectedHash: "368434cb7c792c3c35647f30aa90aae5798a45efcf2ff6abb7123b70c1e7850c"},
		{chainId: "flow-previewnet", expectedHash: "e90268cb6e8385d9eb50f2956f47c1c5f77a7b3111de2f66756b2a48855e05ce"},
		{chainId: "flow-emulator", expectedHash: "c6ccd6b805adcfaa6f9719f1dc71c831c40712977f12d82332ba23e2cb499475"},
	}

	var actualHashes []chainHash

	for _, expected := range expectedHashes {
		chain := flow.ChainID(expected.chainId)

		txBody, err := blueprints.SystemChunkTransaction(chain.Chain())
		require.NoError(t, err)

		actualID := txBody.ID().String()
		actualHashes = append(actualHashes, chainHash{chainId: expected.chainId, expectedHash: actualID})
	}

	require.Equal(t, expectedHashes, actualHashes,
		"Hashes of the system transactions have changed.\n"+
			"The community should be notified!\n\n"+
			"Update the expected hashes with the following values:\n%s", formatHashes(actualHashes))

}

type chainHash struct {
	chainId      string
	expectedHash string
}

func formatHashes(hashes []chainHash) string {
	b := strings.Builder{}
	for _, h := range hashes {
		b.WriteString("{chainId: \"")
		b.WriteString(h.chainId)
		b.WriteString("\", expectedHash: \"")
		b.WriteString(h.expectedHash)
		b.WriteString("\"},\n")
	}
	return b.String()
}

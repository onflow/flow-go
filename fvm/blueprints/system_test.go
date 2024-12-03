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
		{chainId: "flow-mainnet", expectedHash: "a1f19de15909715a088bfdf7dd86beea6f31d1c200ce40fb8eaf73e34eafc440"},
		{chainId: "flow-testnet", expectedHash: "a43e32867ddda18f3f4012e450adbc52d4b761e029c3e6ff9f45de2553c54499"},
		{chainId: "flow-previewnet", expectedHash: "46c0987d1e99aa18005a28cf01b97b2816fb0d10f3a2a42dc324bc1dcc5c7450"},
		{chainId: "flow-emulator", expectedHash: "15de5e19fdf2c4b8182d49681a5f4a08e8724c04f822f730ee8b06f242e9f914"},
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

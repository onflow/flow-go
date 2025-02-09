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
		{chainId: "flow-mainnet", expectedHash: "07b5d1ad4b146db87878e99c9792fd1dc730d99685bb7dc6ff1d1c91461098f5"},
		{chainId: "flow-testnet", expectedHash: "f55e6344faf557573c0f712dc93e75f680a212e1d1980fc22c7d98fba1ea8670"},
		{chainId: "flow-previewnet", expectedHash: "f77c9061da6ca078ee1723496d738f71a28abea680a0633b87b5494879150eee"},
		{chainId: "flow-emulator", expectedHash: "9a2633ebbdd13f6da84299407602fb81b1da55586a1b13e54c4b6cb4812d7e15"},
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

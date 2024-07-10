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
		{chainId: "flow-mainnet", expectedHash: "5d576e7fe4ea81d0bd86fb9aa2aaa5c66d5e496d1e6bd239e8c4ee18041b0635"},
		{chainId: "flow-testnet", expectedHash: "ec5a556387a6081f5c6a5d8df22a066455dd205f18a9a2ce3d0d8e71502e63fe"},
		{chainId: "flow-previewnet", expectedHash: "e147c73620afb0398a87cfb6e3f3f992ae3793197c50eba0795c1a73570f28ea"},
		{chainId: "flow-emulator", expectedHash: "1305a3553a44b3fb8d5de68ddb7394f14a0f32621a080b828ea8cc2c64b49b33"},
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

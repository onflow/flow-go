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
		{chainId: "flow-mainnet", expectedHash: "3408f8b1aa1b33cfc3f78c3f15217272807b14cec4ef64168bcf313bc4174621"},
		{chainId: "flow-testnet", expectedHash: "dadf3e1bf916f6cb2510cbea00ed9be78cc1b7d2b9ec29f0ef1d469ead2dda2d"},
		{chainId: "flow-previewnet", expectedHash: "ecee9d431f3ab406c64bd31c2b574035a9971feabd872f5e8f31b55dd08978f3"},
		{chainId: "flow-emulator", expectedHash: "d201f7b80ee8471754e2a1cad30f5ab888d4be3ba2c0a1cac5a3fcc0b34546a4"},
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

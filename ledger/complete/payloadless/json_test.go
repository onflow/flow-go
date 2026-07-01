package payloadless_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/hash"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/ledger/complete/payloadless"
)

func Test_DumpJSONEmpty(t *testing.T) {

	tr := payloadless.NewEmptyMTrie()

	var buffer bytes.Buffer
	err := tr.DumpAsJSON(&buffer)
	require.NoError(t, err)

	js := buffer.String()
	assert.Empty(t, js)
}

func Test_DumpJSONNonEmpty(t *testing.T) {
	path1 := testutils.PathByUint16(1)
	path2 := testutils.PathByUint16(2)
	path3 := testutils.PathByUint16(3)

	value1 := []byte{1}
	value2 := []byte{2}
	value3 := []byte{3}

	paths := []ledger.Path{path1, path2, path3}
	values := [][]byte{value1, value2, value3}

	tr, _, err := payloadless.NewTrieWithUpdatedRegisters(payloadless.NewEmptyMTrie(), paths, values, true)
	require.NoError(t, err)

	var buffer bytes.Buffer
	err = tr.DumpAsJSON(&buffer)
	require.NoError(t, err)

	js := buffer.String()
	split := strings.Split(js, "\n")

	// filter out empty strings
	rows := make([]string, 0)
	for _, s := range split {
		if len(s) > 0 {
			rows = append(rows, s)
		}
	}

	require.Len(t, rows, 3)

	// Each row is a JSON object {"path":"<hex>","leafHash":"<hex>"}. We assert each
	// path is present together with the leaf hash HashLeaf(path, value).
	type entry struct {
		Path     string `json:"path"`
		LeafHash string `json:"leafHash"`
	}
	for i, p := range paths {
		expectedLeafHash := hash.HashLeaf(hash.Hash(p), values[i])
		expectedPathHex := hex.EncodeToString(p[:])
		expectedHashHex := hex.EncodeToString(expectedLeafHash[:])

		found := false
		for _, row := range rows {
			var e entry
			require.NoError(t, json.Unmarshal([]byte(row), &e), "invalid JSON row: %s", row)
			if e.Path == expectedPathHex {
				require.Equal(t, expectedHashHex, e.LeafHash)
				found = true
				break
			}
		}
		require.True(t, found, "row for path %s not found", expectedPathHex)
	}
}

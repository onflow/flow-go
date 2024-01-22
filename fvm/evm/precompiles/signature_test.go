package precompiles_test

import (
	"testing"

	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
)

func TestFunctionSelector(t *testing.T) {
	expected := gethCrypto.Keccak256([]byte("test()"))[:4]
	require.Equal(t, expected, precompiles.ComputeFunctionSelector("test", nil).Bytes())

	expected = gethCrypto.Keccak256([]byte("test(uint32,uint16)"))[:precompiles.FunctionSelectorLength]
	require.Equal(t, expected,
		precompiles.ComputeFunctionSelector("test", []string{"uint32", "uint16"}).Bytes())

	selector := []byte{1, 2, 3, 4}
	data := []byte{5, 6, 7, 8}
	retSelector, retData := precompiles.SplitFunctionSelector(append(selector, data...))
	require.Equal(t, selector, retSelector[:])
	require.Equal(t, data, retData)
}

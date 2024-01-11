package precompiles_test

import (
	"testing"

	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/precompiles"
)

func TestFunctionSignatures(t *testing.T) {

	expected := gethCrypto.Keccak256([]byte("test()"))[:4]
	require.Equal(t, expected, precompiles.ComputeFunctionSignature("test", nil).Bytes())

	expected = gethCrypto.Keccak256([]byte("test(uint32,uint16)"))[:4]
	require.Equal(t, expected,
		precompiles.ComputeFunctionSignature("test", []string{"uint32", "uint16"}).Bytes())
}

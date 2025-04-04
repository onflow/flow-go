package operation

import (
	"testing"

	"github.com/onflow/flow-go/model/flow"
)

func BenchmarkMakePrefixWithID(b *testing.B) {
	var id flow.Identifier

	for range b.N {
		_ = MakePrefix(codeHeader, id)
	}
}

func BenchmarkMakePrefixWithChainIDAndNum(b *testing.B) {
	chainID := flow.ChainID("flow-emulator")
	num := uint64(42)

	for range b.N {
		_ = MakePrefix(codeFinalizedCluster, chainID, num)
	}
}

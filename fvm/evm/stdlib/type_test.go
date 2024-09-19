package stdlib_test

import (
	"testing"

	"github.com/onflow/cadence/runtime/sema"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/stdlib"
	"github.com/onflow/flow-go/model/flow"
)

func TestContractTypeForChain(t *testing.T) {
	t.Parallel()

	comp := stdlib.ContractTypeForChain(flow.Emulator)
	require.NotNil(t, comp)

	nestedTypes := comp.GetNestedTypes()

	blockExecutedType, present := nestedTypes.Get("BlockExecuted")
	require.True(t, present)

	require.IsType(t, &sema.CompositeType{}, blockExecutedType)
	blockExecutedEventType := blockExecutedType.(*sema.CompositeType)

	require.Equal(t,
		"EVM.BlockExecuted",
		blockExecutedEventType.QualifiedIdentifier(),
	)
}

package handler_test

import (
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func TestAddressAllocator(t *testing.T) {

	testutils.RunWithTestBackend(t, func(backend *testutils.TestBackend) {
		testutils.RunWithTestFlowEVMRootAddress(t, backend, func(root flow.Address) {
			aa, err := handler.NewAddressAllocator(backend, root)
			require.NoError(t, err)

			adr := aa.AllocatePrecompileAddress(3)
			expectedAddress := types.NewAddress(gethCommon.HexToAddress("0x0000000000000000000000010000000000000003"))
			require.Equal(t, expectedAddress, adr)
			// check conforming to types
			require.False(t, types.IsACOAAddress(adr))

			// test default value fall back
			adr, err = aa.AllocateCOAAddress()
			require.NoError(t, err)
			expectedAddress = types.NewAddress(gethCommon.HexToAddress("0x000000000000000000000002ffeeddccbbaa9987"))
			require.Equal(t, expectedAddress, adr)
			// check conforming to types
			require.True(t, types.IsACOAAddress(adr))

			// continous allocation logic
			adr, err = aa.AllocateCOAAddress()
			require.NoError(t, err)
			expectedAddress = types.NewAddress(gethCommon.HexToAddress("0x000000000000000000000002ffddbb997755330e"))
			require.Equal(t, expectedAddress, adr)
			// check conforming to types
			require.True(t, types.IsACOAAddress(adr))
		})
	})

}

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

			// test default value fall back
			adr, err := aa.AllocateAddress()
			require.NoError(t, err)
			expectedAddress := types.NewAddress(gethCommon.HexToAddress("0x00000000000000000001"))
			require.Equal(t, expectedAddress, adr)

			// continous allocation logic
			adr, err = aa.AllocateAddress()
			require.NoError(t, err)
			expectedAddress = types.NewAddress(gethCommon.HexToAddress("0x00000000000000000002"))
			require.Equal(t, expectedAddress, adr)
		})

	})

}

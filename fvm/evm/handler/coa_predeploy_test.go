package handler_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/handler"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// Background:
//   - COA addresses are deterministic: prefix 0x000000000000000000000002 +
//     a suffix derived from the future COA resource's UUID (see MakeCOAAddress),
//     so anyone can compute the address of a COA that does not exist yet.
//   - EVM.EVMAddress.deposit() is access(all), so anyone can deposit FLOW
//     into any EVM address, including a pre-deployed COA address.
//
// Root cause (fixed): deployAt used to unconditionally call
// StateDB.CreateAccount on the pre-funded target address.
// DeltaView.CreateAccount flags an existing account via SelfDestruct
// (toBeDestructed) to carry over the balance, but never clears the flag,
// so StateDB.Commit deleted the account - wiping the freshly deployed COA
// code, the nonce AND the carried-over FLOW balance - while DeployCOA still
// returned success. The fix mirrors Geth's create behaviour: only create the
// account if it does not exist yet, so a pre-existing balance is carried
// over to the deployed COA.
func TestHandler_PreDeployedCOADeposit(t *testing.T) {
	t.Parallel()

	t.Run("control: COA deployment without pre-existing balance works", func(t *testing.T) {
		testutils.RunWithTestBackend(t, flow.Testnet, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				h := SetupHandler(t, backend, rootAddr)

				const uuid = uint64(1)
				coaAddr := h.DeployCOA(uuid)
				require.Equal(t, handler.MakeCOAAddress(uuid), coaAddr)

				deployed := h.AccountByAddress(coaAddr, true)
				require.NotEmpty(t, deployed.Code()) // COA contract is deployed
				require.Equal(t, uint64(1), deployed.Nonce())
			})
		})
	})

	t.Run("deposit FLOW to a pre-deployed COA address is carried over on deployment", func(t *testing.T) {
		testutils.RunWithTestBackend(t, flow.Testnet, func(backend *testutils.TestBackend) {
			testutils.RunWithTestFlowEVMRootAddress(t, backend, func(rootAddr flow.Address) {
				h := SetupHandler(t, backend, rootAddr)

				// pick a uuid whose COA has not been deployed yet and derive
				// its deterministic future address
				const uuid = uint64(1)
				preCOAAddress := handler.MakeCOAAddress(uuid)
				require.True(t, types.IsACOAAddress(preCOAAddress))

				// sanity: the address is fresh - no code, no nonce, no balance
				pre := h.AccountByAddress(preCOAAddress, false)
				require.Empty(t, pre.Code())
				require.Zero(t, pre.Nonce())
				require.True(t, types.BalancesAreEqual(types.NewBalanceFromUFix64(0), pre.Balance()))

				// step 1 - deposit FLOW into the pre-deployed COA address
				// (EVM.EVMAddress.deposit(from:) is access(all), anyone can do this)
				deposit := types.MakeABalanceInFlow(100)
				pre.Deposit(types.NewFlowTokenVault(deposit))
				require.True(t, types.BalancesAreEqual(deposit, pre.Balance()))

				// step 2 - the owner creates the COA resource, which triggers
				// DeployCOA(uuid) targeting the same address
				coaAddr := h.DeployCOA(uuid)
				require.Equal(t, preCOAAddress, coaAddr)

				// step 3 - the deployment succeeds and the pre-deposited FLOW
				// is carried over to the deployed COA (Geth create parity)
				deployed := h.AccountByAddress(coaAddr, true)
				require.NotEmpty(t, deployed.Code())
				require.NotEmpty(t, deployed.CodeHash())
				require.Equal(t, uint64(1), deployed.Nonce())
				require.True(t, types.BalancesAreEqual(deposit, deployed.Balance()),
					"pre-deposited FLOW should be carried over to the deployed COA")

				// step 4 - the COA is fully functional: it can withdraw (an
				// authorized call executing EVM code from the COA address),
				// recovering the carried-over FLOW
				withdraw := types.MakeABalanceInFlow(40)
				vault := deployed.Withdraw(withdraw)
				require.True(t, types.BalancesAreEqual(withdraw, vault.Balance()))
				require.True(t, types.BalancesAreEqual(types.MakeABalanceInFlow(60), deployed.Balance()))
				require.Equal(t, uint64(2), deployed.Nonce())
			})
		})
	})
}

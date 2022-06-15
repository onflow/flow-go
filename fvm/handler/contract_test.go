package handler_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/programs"

	"github.com/onflow/flow-go/fvm/handler"
	stateMock "github.com/onflow/flow-go/fvm/mock/state"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestContract_ChildMergeFunctionality(t *testing.T) {
	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView()))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")
	rAdd := runtime.Address(address)
	err := accounts.Create(nil, address)
	require.NoError(t, err)

	contractHandler := handler.NewContractHandler(accounts, func() bool { return false }, nil, nil, nil)

	// no contract initially
	names, err := contractHandler.GetContractNames(rAdd)
	require.NoError(t, err)
	require.Equal(t, len(names), 0)

	// set contract no need for signing accounts
	err = contractHandler.SetContract(rAdd, "testContract", []byte("ABC"), nil)
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())

	// should not be readable from draft
	cont, err := contractHandler.GetContract(rAdd, "testContract")
	require.NoError(t, err)
	require.Equal(t, len(cont), 0)

	// commit
	_, err = contractHandler.Commit()
	require.NoError(t, err)
	cont, err = contractHandler.GetContract(rAdd, "testContract")
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// rollback
	err = contractHandler.SetContract(rAdd, "testContract2", []byte("ABC"), nil)
	require.NoError(t, err)
	err = contractHandler.Rollback()
	require.NoError(t, err)
	require.False(t, contractHandler.HasUpdates())
	_, err = contractHandler.Commit()
	require.NoError(t, err)

	// test contract shouldn't be there
	cont, err = contractHandler.GetContract(rAdd, "testContract2")
	require.NoError(t, err)
	require.Equal(t, len(cont), 0)

	// test contract should be there
	cont, err = contractHandler.GetContract(rAdd, "testContract")
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// remove
	err = contractHandler.RemoveContract(rAdd, "testContract", nil)
	require.NoError(t, err)

	// contract still there because no commit yet
	cont, err = contractHandler.GetContract(rAdd, "testContract")
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// commit removal
	_, err = contractHandler.Commit()
	require.NoError(t, err)

	// contract should no longer be there
	cont, err = contractHandler.GetContract(rAdd, "testContract")
	require.NoError(t, err)
	require.Equal(t, []byte(nil), cont)
}

func TestContract_AuthorizationFunctionality(t *testing.T) {
	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView()))
	accounts := state.NewAccounts(sth)

	authAdd := flow.HexToAddress("01")
	rAdd := runtime.Address(authAdd)
	err := accounts.Create(nil, authAdd)
	require.NoError(t, err)

	authRemove := flow.HexToAddress("02")
	rRemove := runtime.Address(authRemove)
	err = accounts.Create(nil, authRemove)
	require.NoError(t, err)

	authBoth := flow.HexToAddress("03")
	rBoth := runtime.Address(authBoth)
	err = accounts.Create(nil, authBoth)
	require.NoError(t, err)

	unAuth := flow.HexToAddress("04")
	unAuthR := runtime.Address(unAuth)
	err = accounts.Create(nil, unAuth)
	require.NoError(t, err)

	makeHandler := func() *handler.ContractHandler {
		return handler.NewContractHandler(accounts,
			func() bool { return true },
			func() []common.Address { return []common.Address{rAdd, rBoth} },
			func() []common.Address { return []common.Address{rRemove, rBoth} },
			func(address runtime.Address, code []byte) (bool, error) { return false, nil })
	}

	t.Run("try to set contract with unauthorized account", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{unAuthR})
		require.Error(t, err)
		require.False(t, contractHandler.HasUpdates())
	})

	t.Run("try to set contract with account only authorized for removal", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{rRemove})
		require.Error(t, err)
		require.False(t, contractHandler.HasUpdates())
	})

	t.Run("set contract with account authorized for adding", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract2", []byte("ABC"), []common.Address{rAdd})
		require.NoError(t, err)
		require.True(t, contractHandler.HasUpdates())
	})

	t.Run("set contract with account authorized for adding and removing", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract2", []byte("ABC"), []common.Address{rBoth})
		require.NoError(t, err)
		require.True(t, contractHandler.HasUpdates())
	})

	t.Run("try to remove contract with unauthorized account", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{rAdd})
		require.NoError(t, err)
		_, err = contractHandler.Commit()
		require.NoError(t, err)

		err = contractHandler.RemoveContract(unAuthR, "testContract2", []common.Address{unAuthR})
		require.Error(t, err)
		require.False(t, contractHandler.HasUpdates())
	})

	t.Run("remove contract account authorized for removal", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{rAdd})
		require.NoError(t, err)
		_, err = contractHandler.Commit()
		require.NoError(t, err)

		err = contractHandler.RemoveContract(rRemove, "testContract2", []common.Address{rRemove})
		require.NoError(t, err)
		require.True(t, contractHandler.HasUpdates())
	})

	t.Run("try to remove contract with account only authorized for adding", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{rAdd})
		require.NoError(t, err)
		_, err = contractHandler.Commit()
		require.NoError(t, err)

		err = contractHandler.RemoveContract(rAdd, "testContract2", []common.Address{rAdd})
		require.Error(t, err)
		require.False(t, contractHandler.HasUpdates())
	})

	t.Run("remove contract with account authorized for adding and removing", func(t *testing.T) {
		contractHandler := makeHandler()

		err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{rAdd})
		require.NoError(t, err)
		_, err = contractHandler.Commit()
		require.NoError(t, err)

		err = contractHandler.RemoveContract(rBoth, "testContract2", []common.Address{rBoth})
		require.NoError(t, err)
		require.True(t, contractHandler.HasUpdates())
	})
}

func TestContract_DeploymentVouchers(t *testing.T) {

	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView()))
	accounts := state.NewAccounts(sth)

	addressWithVoucher := flow.HexToAddress("01")
	addressWithVoucherRuntime := runtime.Address(addressWithVoucher)
	err := accounts.Create(nil, addressWithVoucher)
	require.NoError(t, err)

	addressNoVoucher := flow.HexToAddress("02")
	addressNoVoucherRuntime := runtime.Address(addressNoVoucher)
	err = accounts.Create(nil, addressNoVoucher)
	require.NoError(t, err)

	contractHandler := handler.NewContractHandler(
		accounts,
		func() bool { return true },
		func() []common.Address {
			return []common.Address{}
		},
		func() []common.Address {
			return []common.Address{}
		},
		func(address runtime.Address, code []byte) (bool, error) {
			if address.String() == addressWithVoucher.String() {
				return true, nil
			}
			return false, nil
		})

	// set contract without voucher
	err = contractHandler.SetContract(
		addressNoVoucherRuntime,
		"TestContract1",
		[]byte("pub contract TestContract1 {}"),
		[]common.Address{
			addressNoVoucherRuntime,
		},
	)
	require.Error(t, err)
	require.False(t, contractHandler.HasUpdates())

	// try to set contract with voucher
	err = contractHandler.SetContract(
		addressWithVoucherRuntime,
		"TestContract2",
		[]byte("pub contract TestContract2 {}"),
		[]common.Address{
			addressWithVoucherRuntime,
		},
	)
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())
}

func TestContract_ContractUpdate(t *testing.T) {

	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView()))
	accounts := state.NewAccounts(sth)

	flowAddress := flow.HexToAddress("01")
	runtimeAddress := runtime.Address(flowAddress)
	err := accounts.Create(nil, flowAddress)
	require.NoError(t, err)

	var authorizationChecked bool

	contractHandler := handler.NewContractHandler(
		accounts,
		func() bool { return true },
		func() []common.Address {
			return []common.Address{}
		},
		func() []common.Address {
			return []common.Address{}
		},
		func(address runtime.Address, code []byte) (bool, error) {
			// Ensure the voucher check is only called once,
			// for the initial contract deployment,
			// and not for the subsequent update
			require.False(t, authorizationChecked)
			authorizationChecked = true
			return true, nil
		},
	)

	// deploy contract with voucher
	err = contractHandler.SetContract(
		runtimeAddress,
		"TestContract",
		[]byte("pub contract TestContract {}"),
		[]common.Address{
			runtimeAddress,
		},
	)
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())

	contractUpdateKeys, err := contractHandler.Commit()
	require.NoError(t, err)
	require.Equal(
		t,
		[]programs.ContractUpdateKey{
			{
				Address: flowAddress,
				Name:    "TestContract",
			},
		},
		contractUpdateKeys,
	)

	// try to update contract without voucher
	err = contractHandler.SetContract(
		runtimeAddress,
		"TestContract",
		[]byte("pub contract TestContract {}"),
		[]common.Address{
			runtimeAddress,
		},
	)
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())
}

func TestContract_DeterministicErrorOnCommit(t *testing.T) {
	mockAccounts := &stateMock.Accounts{}

	mockAccounts.On("ContractExists", mock.Anything, mock.Anything).
		Return(false, nil)

	mockAccounts.On("SetContract", mock.Anything, mock.Anything, mock.Anything).
		Return(func(contractName string, address flow.Address, contract []byte) error {
			return fmt.Errorf("%s %s", contractName, address.Hex())
		})

	contractHandler := handler.NewContractHandler(
		mockAccounts,
		func() bool { return false },
		nil,
		nil,
		nil,
	)

	address1 := runtime.Address(flow.HexToAddress("0000000000000001"))
	address2 := runtime.Address(flow.HexToAddress("0000000000000002"))

	err := contractHandler.SetContract(address2, "A", []byte("ABC"), nil)
	require.NoError(t, err)

	err = contractHandler.SetContract(address1, "B", []byte("ABC"), nil)
	require.NoError(t, err)

	err = contractHandler.SetContract(address1, "A", []byte("ABC"), nil)
	require.NoError(t, err)

	_, err = contractHandler.Commit()
	require.EqualError(t, err, "A 0000000000000001")
}

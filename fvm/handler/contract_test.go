package handler_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

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

	contractHandler := handler.NewContractHandler(accounts, false, nil, nil)

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
}

func TestContract_AuthorizationFunctionality(t *testing.T) {
	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView()))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")
	rAdd := runtime.Address(address)
	err := accounts.Create(nil, address)
	require.NoError(t, err)

	unAuthAdd := flow.HexToAddress("02")
	unAuthRAdd := runtime.Address(unAuthAdd)
	err = accounts.Create(nil, unAuthAdd)
	require.NoError(t, err)

	contractHandler := handler.NewContractHandler(accounts,
		true,
		func() []common.Address { return []common.Address{rAdd} },
		func(address runtime.Address, code []byte) (bool, error) { return false, nil })

	// try to set contract by an unAuthRAdd
	err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{unAuthRAdd})
	require.Error(t, err)
	require.False(t, contractHandler.HasUpdates())

	// set contract by an authorized account
	err = contractHandler.SetContract(rAdd, "testContract2", []byte("ABC"), []common.Address{rAdd})
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())
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

	contractHandler := handler.NewContractHandler(accounts,
		true,
		func() []common.Address { return []common.Address{} },
		func(address runtime.Address, code []byte) (bool, error) {
			if address.String() == addressWithVoucher.String() {
				return true, nil
			}
			return false, nil
		})

	// set contract without voucher
	err = contractHandler.SetContract(addressNoVoucherRuntime, "testContract1", []byte("ABC"), []common.Address{addressNoVoucherRuntime})
	require.Error(t, err)
	require.False(t, contractHandler.HasUpdates())

	// try to set contract with voucher
	err = contractHandler.SetContract(addressWithVoucherRuntime, "testContract2", []byte("ABC"), []common.Address{addressWithVoucherRuntime})
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())
}

func TestContract_DeterministicErrorOnCommit(t *testing.T) {
	mockAccounts := &stateMock.Accounts{}

	mockAccounts.On("SetContract", mock.Anything, mock.Anything, mock.Anything).Return(func(contractName string, address flow.Address, contract []byte) error {
		return fmt.Errorf("%s %s", contractName, address.Hex())
	})

	contractHandler := handler.NewContractHandler(mockAccounts,
		false,
		nil,
		nil)

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

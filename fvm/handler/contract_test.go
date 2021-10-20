package handler_test

import (
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/handler"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
)

func TestContract_ChildMergeFunctionality(t *testing.T) {
	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView(), state.NewInteractionLimiter(state.WithInteractionLimit(false))))
	accounts := state.NewAccounts(sth)
	address := flow.HexToAddress("01")
	rAdd := runtime.Address(address)
	err := accounts.Create(nil, address)
	require.NoError(t, err)

	contractHandler := handler.NewContractHandler(accounts, false, nil)

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
	sth := state.NewStateHolder(state.NewState(utils.NewSimpleView(), state.NewInteractionLimiter(state.WithInteractionLimit(false))))
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
		func() []common.Address { return []common.Address{rAdd} })

	// try to set contract by an unAuthRAdd
	err = contractHandler.SetContract(rAdd, "testContract1", []byte("ABC"), []common.Address{unAuthRAdd})
	require.Error(t, err)
	require.False(t, contractHandler.HasUpdates())

	// set contract by an authorized account
	err = contractHandler.SetContract(rAdd, "testContract2", []byte("ABC"), []common.Address{rAdd})
	require.NoError(t, err)
	require.True(t, contractHandler.HasUpdates())
}

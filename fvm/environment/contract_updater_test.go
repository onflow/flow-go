package environment_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/model/flow"
)

type testContractUpdaterStubs struct {
	deploymentEnabled    bool
	removalEnabled       bool
	deploymentAuthorized []flow.Address
	removalAuthorized    []flow.Address
}

func (p testContractUpdaterStubs) RestrictedDeploymentEnabled() bool {
	return p.deploymentEnabled
}

func (p testContractUpdaterStubs) RestrictedRemovalEnabled() bool {
	return p.removalEnabled
}

func (p testContractUpdaterStubs) GetAuthorizedAccounts(
	path cadence.Path,
) []flow.Address {
	if path == blueprints.ContractDeploymentAuthorizedAddressesPath {
		return p.deploymentAuthorized
	}
	return p.removalAuthorized
}

func TestContract_ChildMergeFunctionality(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	address := flow.HexToAddress("01")
	err := accounts.Create(nil, address)
	require.NoError(t, err)

	contractUpdater := environment.NewContractUpdaterForTesting(
		accounts,
		testContractUpdaterStubs{})

	// no contract initially
	names, err := accounts.GetContractNames(address)
	require.NoError(t, err)
	require.Equal(t, len(names), 0)

	// set contract no need for signing accounts
	err = contractUpdater.SetContract(
		common.AddressLocation{
			Name:    "testContract",
			Address: common.MustBytesToAddress(address.Bytes())},
		[]byte("ABC"),
		nil)
	require.NoError(t, err)
	require.True(t, contractUpdater.HasUpdates())

	// should not be readable from draft
	cont, err := accounts.GetContract("testContract", address)
	require.NoError(t, err)
	require.Equal(t, len(cont), 0)

	// commit
	_, err = contractUpdater.Commit()
	require.NoError(t, err)
	cont, err = accounts.GetContract("testContract", address)
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// rollback
	err = contractUpdater.SetContract(
		common.AddressLocation{
			Name:    "testContract2",
			Address: common.MustBytesToAddress(address.Bytes())},
		[]byte("ABC"),
		nil)
	require.NoError(t, err)
	contractUpdater.Reset()
	require.False(t, contractUpdater.HasUpdates())
	_, err = contractUpdater.Commit()
	require.NoError(t, err)

	// test contract shouldn't be there
	cont, err = accounts.GetContract("testContract2", address)
	require.NoError(t, err)
	require.Equal(t, len(cont), 0)

	// test contract should be there
	cont, err = accounts.GetContract("testContract", address)
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// remove
	err = contractUpdater.RemoveContract(common.AddressLocation{
		Name:    "testContract",
		Address: common.MustBytesToAddress(address.Bytes())}, nil)
	require.NoError(t, err)

	// contract still there because no commit yet
	cont, err = accounts.GetContract("testContract", address)
	require.NoError(t, err)
	require.Equal(t, cont, []byte("ABC"))

	// commit removal
	_, err = contractUpdater.Commit()
	require.NoError(t, err)

	// contract should no longer be there
	cont, err = accounts.GetContract("testContract", address)
	require.NoError(t, err)
	require.Equal(t, []byte(nil), cont)
}

func TestContract_AuthorizationFunctionality(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)

	authAdd := flow.HexToAddress("01")
	err := accounts.Create(nil, authAdd)
	require.NoError(t, err)

	authRemove := flow.HexToAddress("02")
	err = accounts.Create(nil, authRemove)
	require.NoError(t, err)

	authBoth := flow.HexToAddress("03")
	err = accounts.Create(nil, authBoth)
	require.NoError(t, err)

	unAuth := flow.HexToAddress("04")
	err = accounts.Create(nil, unAuth)
	require.NoError(t, err)

	makeUpdater := func() *environment.ContractUpdaterImpl {
		return environment.NewContractUpdaterForTesting(
			accounts,
			testContractUpdaterStubs{
				deploymentEnabled:    true,
				removalEnabled:       true,
				deploymentAuthorized: []flow.Address{authAdd, authBoth},
				removalAuthorized:    []flow.Address{authRemove, authBoth},
			})

	}

	t.Run("try to set contract with unauthorized account", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{unAuth})
		require.Error(t, err)
		require.False(t, contractUpdater.HasUpdates())
	})

	t.Run("try to set contract with account only authorized for removal", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authRemove})
		require.Error(t, err)
		require.False(t, contractUpdater.HasUpdates())
	})

	t.Run("set contract with account authorized for adding", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authAdd})
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())
	})

	t.Run("set contract with account authorized for adding and removing", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authBoth})
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())
	})

	t.Run("try to remove contract with unauthorized account", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authAdd})
		require.NoError(t, err)
		_, err = contractUpdater.Commit()
		require.NoError(t, err)

		err = contractUpdater.RemoveContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]flow.Address{unAuth})
		require.Error(t, err)
		require.False(t, contractUpdater.HasUpdates())
	})

	t.Run("remove contract account authorized for removal", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authAdd})
		require.NoError(t, err)
		_, err = contractUpdater.Commit()
		require.NoError(t, err)

		err = contractUpdater.RemoveContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]flow.Address{authRemove})
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())
	})

	t.Run("try to remove contract with account only authorized for adding", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authAdd})
		require.NoError(t, err)
		_, err = contractUpdater.Commit()
		require.NoError(t, err)

		err = contractUpdater.RemoveContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]flow.Address{authAdd})
		require.Error(t, err)
		require.False(t, contractUpdater.HasUpdates())
	})

	t.Run("remove contract with account authorized for adding and removing", func(t *testing.T) {
		contractUpdater := makeUpdater()

		err = contractUpdater.SetContract(
			common.AddressLocation{
				Name:    "testContract1",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]byte("ABC"),
			[]flow.Address{authAdd})
		require.NoError(t, err)
		_, err = contractUpdater.Commit()
		require.NoError(t, err)

		err = contractUpdater.RemoveContract(
			common.AddressLocation{
				Name:    "testContract2",
				Address: common.MustBytesToAddress(authAdd.Bytes())},
			[]flow.Address{authBoth})
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())
	})
}

func TestContract_DeterministicErrorOnCommit(t *testing.T) {
	mockAccounts := &envMock.Accounts{}

	mockAccounts.On("ContractExists", mock.Anything, mock.Anything).
		Return(false, nil)

	mockAccounts.On("SetContract", mock.Anything, mock.Anything, mock.Anything).
		Return(func(contractName string, address flow.Address, contract []byte) error {
			return fmt.Errorf("%s %s", contractName, address.Hex())
		})

	contractUpdater := environment.NewContractUpdaterForTesting(
		mockAccounts,
		testContractUpdaterStubs{})

	address1 := flow.HexToAddress("0000000000000001")
	address2 := flow.HexToAddress("0000000000000002")

	err := contractUpdater.SetContract(
		common.AddressLocation{
			Name:    "A",
			Address: common.MustBytesToAddress(address2.Bytes())},
		[]byte("ABC"), nil)
	require.NoError(t, err)

	err = contractUpdater.SetContract(
		common.AddressLocation{
			Name:    "B",
			Address: common.MustBytesToAddress(address1.Bytes())},
		[]byte("ABC"), nil)
	require.NoError(t, err)

	err = contractUpdater.SetContract(
		common.AddressLocation{
			Name:    "A",
			Address: common.MustBytesToAddress(address1.Bytes())},
		[]byte("ABC"), nil)
	require.NoError(t, err)

	_, err = contractUpdater.Commit()
	require.EqualError(t, err, "A 0000000000000001")
}

func TestContract_ContractRemoval(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)

	flowAddress := flow.HexToAddress("01")
	err := accounts.Create(nil, flowAddress)
	require.NoError(t, err)

	t.Run("contract removal with restriction", func(t *testing.T) {
		contractUpdater := environment.NewContractUpdaterForTesting(
			accounts,
			testContractUpdaterStubs{
				removalEnabled: true,
			})

		location := common.AddressLocation{
			Name:    "TestContract",
			Address: common.MustBytesToAddress(flowAddress.Bytes())}

		// deploy contract with voucher
		err = contractUpdater.SetContract(
			location,
			[]byte("access(all) contract TestContract {}"),
			[]flow.Address{
				flowAddress,
			},
		)
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())

		contractUpdates, err := contractUpdater.Commit()
		require.NoError(t, err)
		require.Equal(
			t,
			environment.ContractUpdates{
				Updates: []common.AddressLocation{},
				Deploys: []common.AddressLocation{
					location,
				},
				Deletions: []common.AddressLocation{},
			},
			contractUpdates,
		)

		// update should work
		err = contractUpdater.SetContract(
			location,
			[]byte("access(all) contract TestContract {}"),
			[]flow.Address{
				flowAddress,
			},
		)
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())

		// try remove contract should fail
		err = contractUpdater.RemoveContract(
			location,
			[]flow.Address{
				flowAddress,
			},
		)
		require.Error(t, err)
	})

	t.Run("contract removal without restriction", func(t *testing.T) {

		contractUpdater := environment.NewContractUpdaterForTesting(
			accounts,
			testContractUpdaterStubs{})

		location := common.AddressLocation{
			Name:    "TestContract",
			Address: common.MustBytesToAddress(flowAddress.Bytes())}

		// deploy contract with voucher
		err = contractUpdater.SetContract(
			location,
			[]byte("access(all) contract TestContract {}"),
			[]flow.Address{
				flowAddress,
			},
		)
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())

		contractUpdateKeys, err := contractUpdater.Commit()
		require.NoError(t, err)
		require.Equal(
			t,
			environment.ContractUpdates{
				Updates: []common.AddressLocation{
					location,
				},
				Deploys:   []common.AddressLocation{},
				Deletions: []common.AddressLocation{},
			},
			contractUpdateKeys,
		)

		// update should work
		err = contractUpdater.SetContract(
			location,
			[]byte("access(all) contract TestContract {}"),
			[]flow.Address{
				flowAddress,
			},
		)
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())

		// try remove contract should fail
		err = contractUpdater.RemoveContract(
			location,
			[]flow.Address{
				flowAddress,
			},
		)
		require.NoError(t, err)
		require.True(t, contractUpdater.HasUpdates())

		contractUpdateKeys, err = contractUpdater.Commit()
		require.NoError(t, err)
		require.Equal(
			t,
			environment.ContractUpdates{
				Updates: []common.AddressLocation{},
				Deploys: []common.AddressLocation{},
				Deletions: []common.AddressLocation{
					location,
				},
			},
			contractUpdateKeys,
		)
	})
}

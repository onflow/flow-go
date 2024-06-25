package migrations

import (
	"context"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

func TestContractCleanupMigration1(t *testing.T) {

	t.Parallel()

	// Arrange

	address, err := common.HexToAddress("0x4184b8bdf78db9eb")
	require.NoError(t, err)

	flowAddress := flow.ConvertAddress(address)
	owner := flow.AddressToRegisterOwner(flowAddress)

	const contractNameEmpty = "Foo"
	const contractNameNonEmpty = "Bar"

	registersByAccount := registers.NewByAccount()

	err = registersByAccount.Set(
		owner,
		flow.ContractKey(contractNameEmpty),
		// Some whitespace for testing purposes
		[]byte(" \t  \n  "),
	)
	require.NoError(t, err)

	err = registersByAccount.Set(
		owner,
		flow.ContractKey(contractNameNonEmpty),
		[]byte(" \n  \t access(all) contract Bar {} \n \n"),
	)
	require.NoError(t, err)

	encodedContractNames, err := environment.EncodeContractNames([]string{
		// Unsorted and duplicates for testing purposes
		contractNameEmpty,
		contractNameNonEmpty,
		contractNameEmpty,
		contractNameNonEmpty,
		contractNameEmpty,
		contractNameEmpty,
		contractNameNonEmpty,
	})
	require.NoError(t, err)

	err = registersByAccount.Set(
		owner,
		flow.ContractNamesKey,
		encodedContractNames,
	)
	require.NoError(t, err)

	// Act

	checkingMigration := NewContractCleanupMigration()

	log := zerolog.Nop()

	err = checkingMigration.InitMigration(log, registersByAccount, 1)
	require.NoError(t, err)

	accountRegisters := registersByAccount.AccountRegisters(owner)

	err = checkingMigration.MigrateAccount(
		context.Background(),
		address,
		accountRegisters,
	)
	require.NoError(t, err)

	// Assert

	encodedContractNames, err = registersByAccount.Get(
		owner,
		flow.ContractNamesKey,
	)
	require.NoError(t, err)

	contractNames, err := environment.DecodeContractNames(encodedContractNames)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{
			contractNameNonEmpty,
		},
		contractNames,
	)

	contractEmpty, err := registersByAccount.Get(
		owner,
		flow.ContractKey(contractNameEmpty),
	)
	require.NoError(t, err)
	assert.Nil(t, contractEmpty)

	contractNonEmpty, err := registersByAccount.Get(
		owner,
		flow.ContractKey(contractNameNonEmpty),
	)
	require.NoError(t, err)
	assert.NotEmpty(t, contractNonEmpty)
}

func TestContractCleanupMigration2(t *testing.T) {

	t.Parallel()

	// Arrange

	address, err := common.HexToAddress("0x4184b8bdf78db9eb")
	require.NoError(t, err)

	flowAddress := flow.ConvertAddress(address)
	owner := flow.AddressToRegisterOwner(flowAddress)

	const contractNameEmpty1 = "Foo"
	const contractNameEmpty2 = "Bar"

	registersByAccount := registers.NewByAccount()

	err = registersByAccount.Set(
		owner,
		flow.ContractKey(contractNameEmpty1),
		// Some whitespace for testing purposes
		[]byte(" \t  \n  "),
	)
	require.NoError(t, err)

	err = registersByAccount.Set(
		owner,
		flow.ContractKey(contractNameEmpty2),
		[]byte("\n  \t  \n  \t"),
	)
	require.NoError(t, err)

	encodedContractNames, err := environment.EncodeContractNames([]string{
		// Unsorted and duplicates for testing purposes
		contractNameEmpty1,
		contractNameEmpty2,
		contractNameEmpty1,
		contractNameEmpty2,
		contractNameEmpty1,
		contractNameEmpty1,
		contractNameEmpty2,
	})
	require.NoError(t, err)

	err = registersByAccount.Set(
		owner,
		flow.ContractNamesKey,
		encodedContractNames,
	)
	require.NoError(t, err)

	// Act

	checkingMigration := NewContractCleanupMigration()

	log := zerolog.Nop()

	err = checkingMigration.InitMigration(log, registersByAccount, 1)
	require.NoError(t, err)

	accountRegisters := registersByAccount.AccountRegisters(owner)

	err = checkingMigration.MigrateAccount(
		context.Background(),
		address,
		accountRegisters,
	)
	require.NoError(t, err)

	// Assert

	encodedContractNames, err = registersByAccount.Get(
		owner,
		flow.ContractNamesKey,
	)
	require.NoError(t, err)
	assert.Nil(t, encodedContractNames)

	contractEmpty1, err := registersByAccount.Get(
		owner,
		flow.ContractKey(contractNameEmpty1),
	)
	require.NoError(t, err)
	assert.Nil(t, contractEmpty1)

	contractEmpty2, err := registersByAccount.Get(
		owner,
		flow.ContractKey(contractNameEmpty2),
	)
	require.NoError(t, err)
	assert.Nil(t, contractEmpty2)
}

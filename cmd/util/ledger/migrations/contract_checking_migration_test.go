package migrations

import (
	"fmt"
	"sort"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func oldExampleFungibleTokenCode(fungibleTokenAddress flow.Address) string {
	return fmt.Sprintf(
		`
          import FungibleToken from 0x%s

          pub contract ExampleFungibleToken: FungibleToken {
             pub var totalSupply: UFix64

             pub resource Vault {
                 pub var balance: UFix64
             }
          }
        `,
		fungibleTokenAddress.Hex(),
	)
}

func oldExampleNonFungibleTokenCode(fungibleTokenAddress flow.Address) string {
	return fmt.Sprintf(
		`
          import NonFungibleToken from 0x%s

          pub contract ExampleNFT: NonFungibleToken {
          
              pub var totalSupply: UInt64

              pub resource NFT {

                  pub let id: UInt64
              }

              pub resource Collection {

                  pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}
              }
          }
        `,
		fungibleTokenAddress.Hex(),
	)
}

func TestContractCheckingMigrationProgramRecovery(t *testing.T) {

	t.Parallel()

	registersByAccount := registers.NewByAccount()

	// Set up contracts

	const chainID = flow.Testnet
	chain := chainID.Chain()

	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	contracts := map[flow.Address]map[string][]byte{}

	addContract := func(address flow.Address, name string, code []byte) {
		addressContracts, ok := contracts[address]
		if !ok {
			addressContracts = map[string][]byte{}
			contracts[address] = addressContracts
		}
		require.Empty(t, addressContracts[name])
		addressContracts[name] = code
	}

	addSystemContract := func(systemContract systemcontracts.SystemContract, code []byte) {
		addContract(systemContract.Address, systemContract.Name, code)
	}

	env := templates.Environment{}

	addSystemContract(
		systemContracts.ViewResolver,
		coreContracts.ViewResolver(),
	)
	env.ViewResolverAddress = systemContracts.ViewResolver.Address.Hex()

	addSystemContract(
		systemContracts.Burner,
		coreContracts.Burner(),
	)
	env.BurnerAddress = systemContracts.Burner.Address.Hex()

	addSystemContract(
		systemContracts.FungibleToken,
		coreContracts.FungibleToken(env),
	)
	addSystemContract(
		systemContracts.NonFungibleToken,
		coreContracts.NonFungibleToken(env),
	)

	const exampleFungibleTokenContractName = "ExampleFungibleToken"
	const exampleNonFungibleTokenContractName = "ExampleNonFungibleToken"

	// Use an old version of the ExampleFungibleToken contract,
	// and "deploy" it at some arbitrary, high (i.e. non-system) address
	exampleAddress, err := chain.AddressAtIndex(1000)
	require.NoError(t, err)
	addContract(
		exampleAddress,
		exampleFungibleTokenContractName,
		[]byte(oldExampleFungibleTokenCode(systemContracts.FungibleToken.Address)),
	)
	// Use an old version of the ExampleNonFungibleToken contract,
	// and "deploy" it at some arbitrary, high (i.e. non-system) address
	require.NoError(t, err)
	addContract(
		exampleAddress,
		exampleNonFungibleTokenContractName,
		[]byte(oldExampleNonFungibleTokenCode(systemContracts.NonFungibleToken.Address)),
	)

	for address, addressContracts := range contracts {

		for contractName, code := range addressContracts {

			err := registersByAccount.Set(
				string(address[:]),
				flow.ContractKey(contractName),
				code,
			)
			require.NoError(t, err)
		}

		contractNames := make([]string, 0, len(addressContracts))
		for contractName := range addressContracts {
			contractNames = append(contractNames, contractName)
		}
		sort.Strings(contractNames)

		encodedContractNames, err := environment.EncodeContractNames(contractNames)
		require.NoError(t, err)

		err = registersByAccount.Set(
			string(address[:]),
			flow.ContractNamesKey,
			encodedContractNames,
		)
		require.NoError(t, err)
	}

	programs := map[common.Location]*interpreter.Program{}

	rwf := &testReportWriterFactory{}

	// Run contract checking migration

	log := zerolog.Nop()
	checkingMigration := NewContractCheckingMigration(
		log,
		rwf,
		chainID,
		false,
		nil,
		programs,
	)

	err = checkingMigration(registersByAccount)
	require.NoError(t, err)

	reporter := rwf.reportWriters[contractCheckingReporterName]

	assert.Equal(t,
		[]any{
			contractCheckingSuccess{
				AccountAddress: common.Address(systemContracts.NonFungibleToken.Address),
				ContractName:   systemcontracts.ContractNameNonFungibleToken,
				Code:           string(coreContracts.NonFungibleToken(env)),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(systemContracts.ViewResolver.Address),
				ContractName:   systemcontracts.ContractNameViewResolver,
				Code:           string(coreContracts.ViewResolver()),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(systemContracts.Burner.Address),
				ContractName:   systemcontracts.ContractNameBurner,
				Code:           string(coreContracts.Burner()),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(systemContracts.FungibleToken.Address),
				ContractName:   systemcontracts.ContractNameFungibleToken,
				Code:           string(coreContracts.FungibleToken(env)),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(exampleAddress),
				ContractName:   exampleFungibleTokenContractName,
				Code:           oldExampleFungibleTokenCode(systemContracts.FungibleToken.Address),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(exampleAddress),
				ContractName:   exampleNonFungibleTokenContractName,
				Code:           oldExampleNonFungibleTokenCode(systemContracts.NonFungibleToken.Address),
			},
		},
		reporter.entries,
	)

	// Check that the programs are recovered correctly after the migration.

	mr, err := NewInterpreterMigrationRuntime(registersByAccount, chainID, InterpreterMigrationRuntimeConfig{})
	require.NoError(t, err)

	// First, we need to create the example account

	err = mr.Accounts.Create(nil, exampleAddress)
	require.NoError(t, err)

	expectedAddresses := map[flow.Address]struct{}{
		exampleAddress: {},
	}

	err = mr.Commit(expectedAddresses, log)
	require.NoError(t, err)

	// Next, we need to manually store contract values in the example account,
	// simulating the effect of the deploying the original contracts.
	//
	// We need to do so with a new runtime,
	// because the previous runtime's transaction state is finalized.

	mr, err = NewInterpreterMigrationRuntime(registersByAccount, chainID, InterpreterMigrationRuntimeConfig{})
	require.NoError(t, err)

	contractsStorageMap := mr.Storage.GetStorageMap(
		common.Address(exampleAddress),
		runtime.StorageDomainContract,
		true,
	)

	inter := mr.Interpreter

	exampleFungibleTokenContractValue := interpreter.NewCompositeValue(
		inter,
		interpreter.EmptyLocationRange,
		common.NewAddressLocation(
			nil,
			common.Address(exampleAddress),
			exampleFungibleTokenContractName,
		),
		exampleFungibleTokenContractName,
		common.CompositeKindContract,
		[]interpreter.CompositeField{
			{
				Name:  "totalSupply",
				Value: interpreter.NewUnmeteredUFix64ValueWithInteger(42, interpreter.EmptyLocationRange),
			},
		},
		common.Address(exampleAddress),
	)

	contractsStorageMap.SetValue(
		inter,
		interpreter.StringStorageMapKey(exampleFungibleTokenContractName),
		exampleFungibleTokenContractValue,
	)

	exampleNonFungibleTokenContractValue := interpreter.NewCompositeValue(
		inter,
		interpreter.EmptyLocationRange,
		common.NewAddressLocation(
			nil,
			common.Address(exampleAddress),
			exampleNonFungibleTokenContractName,
		),
		exampleNonFungibleTokenContractName,
		common.CompositeKindContract,
		[]interpreter.CompositeField{
			{
				Name:  "totalSupply",
				Value: interpreter.NewUnmeteredUInt64Value(42),
			},
		},
		common.Address(exampleAddress),
	)

	contractsStorageMap.SetValue(
		inter,
		interpreter.StringStorageMapKey(exampleNonFungibleTokenContractName),
		exampleNonFungibleTokenContractValue,
	)

	err = mr.Storage.NondeterministicCommit(inter, false)
	require.NoError(t, err)

	err = mr.Commit(expectedAddresses, log)
	require.NoError(t, err)

	// Setup complete, now we can run the test transactions

	type testCase struct {
		name  string
		code  string
		check func(t *testing.T, err error)
	}

	testCases := []testCase{
		{
			name: exampleFungibleTokenContractName,
			code: fmt.Sprintf(
				`
                  import ExampleFungibleToken from %s

                  transaction {
                      execute {
                          assert(ExampleFungibleToken.totalSupply == 42.0)
                          destroy ExampleFungibleToken.createEmptyVault(
                              vaultType: Type<@ExampleFungibleToken.Vault>()
                          )
                      }
                  }
                `,
				exampleAddress.HexWithPrefix(),
			),
			check: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "Contract ExampleFungibleToken is no longer functional")
				require.ErrorContains(t, err, "createEmptyVault is not available in recovered program.")
			},
		},
		{
			name: exampleNonFungibleTokenContractName,
			code: fmt.Sprintf(
				`
                  import ExampleNonFungibleToken from %s

                  transaction {
                      execute {
                          destroy ExampleNonFungibleToken.createEmptyCollection(
                              nftType: Type<@ExampleNonFungibleToken.NFT>()
                          )
                      }
                  }
                `,
				exampleAddress.HexWithPrefix(),
			),
			check: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorContains(t, err, "Contract ExampleNonFungibleToken is no longer functional")
				require.ErrorContains(t, err, "createEmptyCollection is not available in recovered program.")
			},
		},
	}

	storageSnapshot := snapshot.MapStorageSnapshot{}

	newPayloads := registersByAccount.DestructIntoPayloads(1)

	for _, newPayload := range newPayloads {
		registerID, registerValue, err := convert.PayloadToRegister(newPayload)
		require.NoError(t, err)

		storageSnapshot[registerID] = registerValue
	}

	test := func(testCase testCase) {

		t.Run(testCase.name, func(t *testing.T) {

			txBody := flow.NewTransactionBody().
				SetScript([]byte(testCase.code))

			vm := fvm.NewVirtualMachine()

			ctx := fvm.NewContext(
				fvm.WithChain(chain),
				fvm.WithAuthorizationChecksEnabled(false),
				fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
				fvm.WithCadenceLogging(true),
			)

			_, output, err := vm.Run(
				ctx,
				fvm.Transaction(txBody, 0),
				storageSnapshot,
			)

			require.NoError(t, err)
			testCase.check(t, output.Err)
		})
	}

	for _, testCase := range testCases {
		test(testCase)
	}
}

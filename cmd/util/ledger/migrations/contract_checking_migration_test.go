package migrations

import (
	"fmt"
	"sort"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func oldExampleFungibleTokenCode(fungibleTokenAddress flow.Address) string {
	return fmt.Sprintf(
		`
          import FungibleToken from 0x%s

          pub contract ExampleFungibleToken: FungibleToken {
             pub var totalSupply: UFix64

             pub resource Vault: FungibleToken.Provider, FungibleToken.Receiver, FungibleToken.Balance {
                 pub var balance: UFix64

                 init(balance: UFix64) {
                     self.balance = balance
                 }

                 pub fun withdraw(amount: UFix64): @FungibleToken.Vault {
                     self.balance = self.balance - amount
                     emit TokensWithdrawn(amount: amount, from: self.owner?.address)
                     return <-create Vault(balance: amount)
                 }

                 pub fun deposit(from: @FungibleToken.Vault) {
                     let vault <- from as! @ExampleToken.Vault
                     self.balance = self.balance + vault.balance
                     emit TokensDeposited(amount: vault.balance, to: self.owner?.address)
                     vault.balance = 0.0
                     destroy vault
                 }

                 destroy() {
                     if self.balance > 0.0 {
                         ExampleToken.totalSupply = ExampleToken.totalSupply - self.balance
                     }
                 }
             }

             pub fun createEmptyVault(): @Vault {
                 return <-create Vault(balance: 0.0)
             }

             init() {
                 self.totalSupply = 0.0
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
          
              /// Total supply of ExampleNFTs in existence
              pub var totalSupply: UInt64

              /// The core resource that represents a Non Fungible Token.
              /// New instances will be created using the NFTMinter resource
              /// and stored in the Collection resource
              ///
              pub resource NFT: NonFungibleToken.INFT {
          
                  /// The unique ID that each NFT has
                  pub let id: UInt64

                  init(id: UInt64) {
                      self.id = id
                  }
              }

              init() {
                  // Initialize the total supply
                  self.totalSupply = 0
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

	// Use an old version of the ExampleFungibleToken contract,
	// and "deploy" it at some arbitrary, high (i.e. non-system) address
	exampleAddress, err := chainID.Chain().AddressAtIndex(1000)
	require.NoError(t, err)
	addContract(
		exampleAddress,
		"ExampleFungibleToken",
		[]byte(oldExampleFungibleTokenCode(systemContracts.FungibleToken.Address)),
	)
	// Use an old version of the ExampleNonFungibleToken contract,
	// and "deploy" it at some arbitrary, high (i.e. non-system) address
	require.NoError(t, err)
	addContract(
		exampleAddress,
		"ExampleNonFungibleToken",
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
				ContractName:   "ExampleFungibleToken",
				Code:           oldExampleFungibleTokenCode(systemContracts.FungibleToken.Address),
			},
			contractCheckingSuccess{
				AccountAddress: common.Address(exampleAddress),
				ContractName:   "ExampleNonFungibleToken",
				Code:           oldExampleNonFungibleTokenCode(systemContracts.NonFungibleToken.Address),
			},
		},
		reporter.entries,
	)
}

package virtualmachine_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	execTestutil "github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("transaction execution results in cached program", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Authorizers: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}

		err := execTestutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, tx)

		require.NoError(t, err)
		require.True(t, result.Succeeded())
		require.Nil(t, result.Error)

		// Determine location of transaction
		txID := tx.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)
	})

}

func TestScriptASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)

	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("script execution results in cached program", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteScript(ledger, script)
		require.NoError(t, err)
		require.True(t, result.Succeeded())

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(script)
		location := runtime.ScriptLocation(scriptHash)

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)

	})
}

func TestTransactionWithProgramASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, err := execTestutil.GenerateAccountPrivateKeys(3)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, err := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)
	require.NoError(t, err)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	err = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)
	require.NoError(t, err)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	err = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)
	require.NoError(t, err)

	// Create deployment transaction that imports the FlowToken contract
	useImportTx := flow.TransactionBody{
		Authorizers: []flow.Address{accounts[2]},
		Script: []byte(fmt.Sprintf(`
			import FlowToken from 0x%s
			transaction {
				prepare(signer: AuthAccount) {}
				execute {
					let v <- FlowToken.createEmptyVault()
					destroy v
				}
			}
		`, accounts[1])),
	}
	err = execTestutil.SignTransaction(&useImportTx, accounts[2], privateKeys[1], 0)
	require.NoError(t, err)

	// Deploy the FungibleToken contract interface
	result, err := bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)
	require.NoError(t, err)
	require.True(t, result.Succeeded())
	require.Nil(t, result.Error)

	// Deploy the FlowToken contract
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	require.NoError(t, err)
	require.True(t, result.Succeeded())
	require.Nil(t, result.Error)

	// Run the Use import (FT Vault resource) transaction
	result, err = bc.ExecuteTransaction(ledger, &useImportTx)
	require.NoError(t, err)
	require.True(t, result.Succeeded())
	require.Nil(t, result.Error)

	// Determine location of transaction
	txID := useImportTx.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := vm.ASTCache().GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

func BenchmarkTransactionWithProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(b, err)
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, err := execTestutil.GenerateAccountPrivateKeys(3)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, err := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)
	require.NoError(b, err)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	err = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)
	require.NoError(b, err)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	err = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)
	require.NoError(b, err)

	// Deploy the FungibleToken contract interface.
	result, err := bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)
	require.True(b, result.Succeeded())
	require.NoError(b, err)

	// Deploy the FlowToken contract.
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	require.True(b, result.Succeeded())
	require.NoError(b, err)

	// Create many transactions that imports the FlowToken contract.
	var txs []flow.TransactionBody
	for i := 0; i < 1000; i++ {
		tx := flow.TransactionBody{
			Authorizers: []flow.Address{accounts[2]},
			Script: []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, accounts[1], i)),
		}
		err := execTestutil.SignTransaction(&tx, accounts[2], privateKeys[1], uint64(i))
		if err != nil {
			panic(err)
		}
		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.ExecuteTransaction(ledger, &tx)
			require.True(b, result.Succeeded())
			require.NoError(b, err)
		}
	}

}

type nonFunctioningCache struct{}

func (cache *nonFunctioningCache) GetProgram(location ast.Location) (*ast.Program, error) {
	return nil, nil
}

func (cache *nonFunctioningCache) SetProgram(location ast.Location, program *ast.Program) error {
	return nil
}

func BenchmarkTransactionWithoutProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.NewWithCache(rt, &nonFunctioningCache{})
	require.NoError(b, err)
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, err := execTestutil.GenerateAccountPrivateKeys(3)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, err := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)
	require.NoError(b, err)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	err = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)
	require.NoError(b, err)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	err = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)
	require.NoError(b, err)

	// Deploy the FungibleToken contract interface.
	result, err := bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)
	require.True(b, result.Succeeded())
	require.NoError(b, err)

	// Deploy the FlowToken contract.
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	require.True(b, result.Succeeded())
	require.NoError(b, err)

	// Create many transactions that imports the FlowToken contract.
	var txs []flow.TransactionBody
	for i := 0; i < 1000; i++ {
		tx := flow.TransactionBody{
			Authorizers: []flow.Address{accounts[2]},
			Script: []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, accounts[1], i)),
		}
		_ = execTestutil.SignTransaction(&tx, accounts[2], privateKeys[1], uint64(i))
		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.ExecuteTransaction(ledger, &tx)
			require.True(b, result.Succeeded())
			require.NoError(b, err)
		}
	}
}

func TestProgramASTCacheAvoidRaceCondition(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, err := execTestutil.GenerateAccountPrivateKeys(3)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, err := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)
	require.NoError(t, err)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	err = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)
	require.NoError(t, err)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	err = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)
	require.NoError(t, err)

	// Deploy the FungibleToken contract interface.
	result, err := bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)
	require.True(t, result.Succeeded())
	require.NoError(t, err)

	// Deploy the FlowToken contract.
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	require.True(t, result.Succeeded())
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int, wg *sync.WaitGroup) {
			defer wg.Done()
			result, err := bc.ExecuteScript(ledger, []byte(fmt.Sprintf(`
			import FlowToken from 0x%s
			pub fun main() {
				log("Transaction %d")
				let v <- FlowToken.createEmptyVault()
				destroy v
			}
		`, accounts[1], id)))
			require.True(t, result.Succeeded())
			require.NoError(t, err)
		}(i, &wg)
	}
	wg.Wait()

	// Determine location of transaction
	txID := deployFlowTokenContractTx.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := vm.ASTCache().GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

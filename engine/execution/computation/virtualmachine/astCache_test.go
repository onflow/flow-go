package virtualmachine_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/assert"
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

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.Nil(t, result.Error)

		// Determine location of transaction
		txID := tx.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		assert.NotNil(t, program)
		assert.NoError(t, err)
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
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(script)
		location := runtime.ScriptLocation(scriptHash)

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		assert.NotNil(t, program)
		assert.NoError(t, err)

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
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	// Deploy the FlowToken contract
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	// Run the Use import (FT Vault resource) transaction
	result, err = bc.ExecuteTransaction(ledger, &useImportTx)
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	// Determine location of transaction
	txID := useImportTx.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := vm.ASTCache().GetProgram(location)
	assert.NotNil(t, program)
	assert.NoError(t, err)
}

func BenchmarkTransactionWithProgramASTCache(t *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, _ := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, _ := execTestutil.GenerateAccountPrivateKeys(3)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, _ := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	_ = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	_ = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)

	// Deploy the FungibleToken contract interface.
	_, _ = bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)

	// Deploy the FlowToken contract.
	_, _ = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)

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

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			_, err := bc.ExecuteTransaction(ledger, &tx)
			if err != nil {
				panic(err)
			}
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

func BenchmarkTransactionWithoutProgramASTCache(t *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, _ := virtualmachine.NewWithCache(rt, &nonFunctioningCache{})
	bc := vm.NewBlockContext(&h)

	// Create a number of account private keys.
	privateKeys, _ := execTestutil.GenerateAccountPrivateKeys(3)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger, accounts, _ := execTestutil.BootstrappedLedger(make(virtualmachine.MapLedger), privateKeys)

	// Create FungibleToken deployment transaction.
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(accounts[0])
	_ = execTestutil.SignTransaction(&deployFungibleTokenContractTx, accounts[0], flow.RootAccountPrivateKey, 0)

	// Create FlowToken deployment transaction.
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(accounts[1], accounts[0])
	_ = execTestutil.SignTransaction(&deployFlowTokenContractTx, accounts[1], privateKeys[0], 0)

	// Deploy the FungibleToken contract interface.
	_, _ = bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)

	// Deploy the FlowToken contract.
	_, _ = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)

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

	t.ResetTimer()

	for i := 0; i < t.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			_, err := bc.ExecuteTransaction(ledger, &tx)
			if err != nil {
				panic(err)
			}
		}
	}

}

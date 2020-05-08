package virtualmachine_test

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence/runtime"
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

func TestTransactionWithImportASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)

	bc := vm.NewBlockContext(&h)

	ledger, err := execTestutil.RootBootstrappedLedger()
	require.NoError(t, err)

	deployContractTx := execTestutil.CreateDeployFTContractTransaction()
	err = execTestutil.SignTransactionByRoot(&deployContractTx, 0)
	require.NoError(t, err)

	useImportTx := flow.TransactionBody{
		Authorizers: []flow.Address{unittest.AddressFixture()},
		Script: []byte(`
				import FungibleToken from 0x1
                transaction {
				  prepare(signer: AuthAccount) {}
				  execute {
					  let v <- FungibleToken.createEmptyVault()
					  destroy v
				  }
                }
            `),
	}
	err = execTestutil.SignTransactionByRoot(&useImportTx, 1)
	require.NoError(t, err)

	// Deploy FT contract
	result, err := bc.ExecuteTransaction(ledger, &deployContractTx)

	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	// Use import (FT Vault resource)
	result, err = bc.ExecuteTransaction(ledger, &useImportTx)
	if result.Error != nil {
		fmt.Println(result.Error.ErrorMessage())
	}

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

func BenchmarkTransactionWithImportASTCache(t *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)

	bc := vm.NewBlockContext(&h)

	ledger, err := execTestutil.RootBootstrappedLedger()
	require.NoError(t, err)

	deployContractTx := execTestutil.CreateDeployFTContractTransaction()
	err = execTestutil.SignTransactionByRoot(&deployContractTx, 0)
	require.NoError(t, err)

	useImportTx := flow.TransactionBody{
		Authorizers: []flow.Address{unittest.AddressFixture()},
		Script: []byte(`
				import FungibleToken from 0x1
                transaction {
				  prepare(signer: AuthAccount) {}
				  execute {
				  }
                }
            `),
	}
	err = execTestutil.SignTransactionByRoot(&useImportTx, 1)
	require.NoError(t, err)

	// Deploy FT contract
	result, err := bc.ExecuteTransaction(ledger, &deployContractTx)

	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	t.ResetTimer()

	// Use import (FT Vault resource)
	result, err = bc.ExecuteTransaction(ledger, &useImportTx)

	t.StopTimer()

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

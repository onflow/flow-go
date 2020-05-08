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

	// Create FungibleToken deployment transaction
	deployFungibleTokenContractTx := execTestutil.CreateDeployFungibleTokenContractInterfaceTransaction(flow.RootAddress)
	err = execTestutil.SignTransactionByRoot(&deployFungibleTokenContractTx, 0)
	require.NoError(t, err)

	// Create FlowToken deployment transaction
	deployFlowTokenContractTx := execTestutil.CreateDeployFlowTokenContractTransaction(flow.RootAddress, flow.BytesToAddress([]byte{0}))
	err = execTestutil.SignTransactionByRoot(&deployFlowTokenContractTx, 1)
	require.NoError(t, err)

	// Create deployment transaction that imports the FlowToken contract
	useImportTx := flow.TransactionBody{
		Authorizers: []flow.Address{flow.RootAddress},
		Script: []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
                transaction {
				  prepare(signer: AuthAccount) {}
				  execute {
					  let v <- FlowToken.createEmptyVault()
					  destroy v
				  }
                }
            `, flowTokenAddress),
	}
	err = execTestutil.SignTransactionByRoot(&useImportTx, 2)
	require.NoError(t, err)

	// Deploy the FungibleToken contract interface
	result, err := bc.ExecuteTransaction(ledger, &deployFungibleTokenContractTx)
	assert.NoError(t, err)
	assert.True(t, result.Succeeded())
	assert.Nil(t, result.Error)

	// Deploy the FlowToken contract
	result, err = bc.ExecuteTransaction(ledger, &deployFlowTokenContractTx)
	assert.NoError(t, err)
	// fmt.Println(result.Error.ErrorMessage())
	assert.True(t, result.Succeeded())
	fmt.Println(result.Error.ErrorMessage())
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

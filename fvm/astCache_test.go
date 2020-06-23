package fvm_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const CacheSize = 256

func TestTransactionASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(cache))

	t.Run("transaction execution results in cached program", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Authorizers: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}

		chain := flow.Mainnet.Chain()
		err := testutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(chain)

		result, err := bc.Invoke(fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		require.True(t, result.Succeeded())
		require.Nil(t, result.Error)

		// Determine location of transaction
		txID := tx.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cached program
		program, err := cache.GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)
	})

}

func TestScriptASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(cache))

	t.Run("script execution results in cached program", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := testutil.RootBootstrappedLedger(flow.Mainnet.Chain())

		result, err := bc.Invoke(fvm.Script(script), ledger)
		require.NoError(t, err)
		require.True(t, result.Succeeded())

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(script)
		location := runtime.ScriptLocation(scriptHash)

		// Get cached program
		program, err := cache.GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)

	})
}

func TestTransactionWithProgramASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(cache))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger(chain)
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
	require.NoError(t, err)

	// Create deployment transaction that imports the FlowToken contract
	useImportTx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, fvm.FlowTokenAddress(chain))),
		).
		AddAuthorizer(accounts[0]).
		SetProposalKey(accounts[0], 0, 0).
		SetPayer(chain.ServiceAddress())

	err = testutil.SignPayload(useImportTx, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(useImportTx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// Run the Use import (FT Vault resource) transaction
	result, err := bc.Invoke(fvm.Transaction(useImportTx), ledger)
	require.NoError(t, err)

	if !assert.Nil(t, result.Error) {
		t.Fatal(result.Error.ErrorMessage())
	}

	// Determine location of transaction
	txID := useImportTx.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := cache.GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

func BenchmarkTransactionWithProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(b, err)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(cache))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger(chain)
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
	require.NoError(b, err)

	// Create many transactions that import the FlowToken contract.
	var txs []*flow.TransactionBody

	for i := 0; i < 1000; i++ {
		tx := flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, fvm.FlowTokenAddress(chain), i)),
			).
			AddAuthorizer(accounts[0]).
			SetProposalKey(accounts[0], 0, uint64(i)).
			SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(tx, accounts[0], privateKeys[0])
		require.NoError(b, err)

		err = testutil.SignEnvelope(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(b, err)

		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.Invoke(fvm.Transaction(tx), ledger)
			require.NoError(b, err)

			if !assert.Nil(b, result.Error) {
				b.Fatal(result.Error.ErrorMessage())
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

func BenchmarkTransactionWithoutProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(&nonFunctioningCache{}))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger(chain)
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
	require.NoError(b, err)

	// Create many transactions that import the FlowToken contract.
	var txs []*flow.TransactionBody

	for i := 0; i < 1000; i++ {
		tx := flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, fvm.FlowTokenAddress(chain), i)),
			).
			AddAuthorizer(accounts[0]).
			SetPayer(accounts[0]).
			SetProposalKey(accounts[0], 0, uint64(i))

		err = testutil.SignEnvelope(tx, accounts[0], privateKeys[0])
		require.NoError(b, err)

		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.Invoke(fvm.Transaction(tx), ledger)
			require.True(b, result.Succeeded())
			require.NoError(b, err)
		}
	}
}

func TestProgramASTCacheAvoidRaceCondition(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	bc := vm.NewContext(fvm.WithBlockHeader(&h), fvm.WithASTCache(cache))

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger(chain)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int, wg *sync.WaitGroup) {
			defer wg.Done()
			view := delta.NewView(ledger.Get)

			script := []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				pub fun main() {
					log("Script %d")
					let v <- FlowToken.createEmptyVault()
					destroy v
				}
			`, fvm.FlowTokenAddress(chain), id))

			result, err := bc.Invoke(fvm.Script(script), view)
			if !assert.True(t, result.Succeeded()) {
				t.Log(result.Error.ErrorMessage())
			}
			require.NoError(t, err)
			require.True(t, result.Succeeded())
		}(i, &wg)
	}
	wg.Wait()

	location := runtime.AddressLocation(fvm.FlowTokenAddress(chain).Bytes())

	// Get cached program
	program, err := cache.GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

package fvm_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const CacheSize = 256

func TestTransactionASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	t.Run("transaction execution results in cached program", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `)).
			AddAuthorizer(unittest.AddressFixture())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(chain)

		tx := fvm.Transaction(txBody)

		err = vm.Invoke(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		// Determine location of transaction
		txID := txBody.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cached program
		program, err := cache.GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)
	})
}

func TestScriptASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	t.Run("script execution results in cached program", func(t *testing.T) {
		code := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := testutil.RootBootstrappedLedger(chain)

		script := fvm.Script(code)

		err := vm.Invoke(ctx, script, ledger)
		require.NoError(t, err)

		assert.NoError(t, script.Err)

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(code)
		location := runtime.ScriptLocation(scriptHash)

		// Get cached program
		program, err := cache.GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)

	})
}

func TestTransactionWithProgramASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger(chain)
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
	require.NoError(t, err)

	// Create deployment transaction that imports the FlowToken contract
	txBody := flow.NewTransactionBody().
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

	err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// Run the Use import (FT Vault resource) transaction

	tx := fvm.Transaction(txBody)

	err = vm.Invoke(ctx, tx, ledger)
	require.NoError(t, err)

	assert.NoError(t, tx.Err)

	// Determine location of transaction
	txID := txBody.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := cache.GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

func TestTransactionWithProgramASTCacheConsistentRegTouches(t *testing.T) {
	createLedgerOps := func(withCache bool) map[string]bool {
		rt := runtime.NewInterpreterRuntime()
		h := unittest.BlockHeaderFixture()

		chain := flow.Mainnet.Chain()

		vm := fvm.New(rt, chain)

		cache, err := fvm.NewLRUASTCache(CacheSize)
		require.NoError(t, err)

		options := []fvm.Option{
			fvm.WithBlockHeader(&h),
		}

		if withCache {
			options = append(options, fvm.WithASTCache(cache))
		}

		ctx := fvm.NewContext(options...)

		// Create a number of account private keys.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		ledger := testutil.RootBootstrappedLedger(chain)
		accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
		require.NoError(t, err)

		// Create deployment transaction that imports the FlowToken contract
		txBody := flow.NewTransactionBody().
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

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		// Run the Use import (FT Vault resource) transaction

		tx := fvm.Transaction(txBody)

		err = vm.Invoke(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		return ledger.RegTouchSet
	}

	assert.Equal(t, createLedgerOps(true), createLedgerOps(false))
}

func BenchmarkTransactionWithProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(b, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

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
		for _, txBody := range txs {
			// Run the Use import (FT Vault resource) transaction.
			tx := fvm.Transaction(txBody)

			err := vm.Invoke(ctx, tx, ledger)
			assert.NoError(b, err)

			assert.NoError(b, tx.Err)
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

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	ctx := fvm.NewContext(fvm.WithASTCache(&nonFunctioningCache{}))

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
		for _, txBody := range txs {
			// Run the Use import (FT Vault resource) transaction.
			tx := fvm.Transaction(txBody)

			err := vm.Invoke(ctx, tx, ledger)
			assert.NoError(b, err)

			assert.NoError(b, tx.Err)
		}
	}
}

func TestProgramASTCacheAvoidRaceCondition(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)

		ledger := testutil.RootBootstrappedLedger(chain)

		go func(id int, wg *sync.WaitGroup) {
			defer wg.Done()

			code := []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				pub fun main() {
					log("Script %d")
					let v <- FlowToken.createEmptyVault()
					destroy v
				}
			`, fvm.FlowTokenAddress(chain), id))

			script := fvm.Script(code)

			err := vm.Invoke(ctx, script, ledger)
			require.NoError(t, err)

			assert.NoError(t, script.Err)
		}(i, &wg)
	}
	wg.Wait()

	location := runtime.AddressLocation(fvm.FlowTokenAddress(chain).Bytes())

	// Get cached program
	var program *ast.Program
	program, err = cache.GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

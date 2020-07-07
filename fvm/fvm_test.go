package fvm_test

import (
	"crypto/rand"
	"fmt"
	"strconv"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	fvmmock "github.com/dapperlabs/flow-go/fvm/mock"
	"github.com/dapperlabs/flow-go/fvm/state"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func vmTest(
	f func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger),
	opts ...fvm.Option,
) func(t *testing.T) {
	return func(t *testing.T) {
		rt := runtime.NewInterpreterRuntime()

		chain := flow.Mainnet.Chain()

		vm := fvm.New(rt)

		cache, err := fvm.NewLRUASTCache(CacheSize)
		require.NoError(t, err)

		baseOpts := []fvm.Option{
			fvm.WithChain(chain),
			fvm.WithASTCache(cache),
		}

		opts = append(baseOpts, opts...)

		ctx := fvm.NewContext(opts...)

		ledger := state.NewMapLedger()

		err = vm.Run(
			ctx,
			fvm.Bootstrap(unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply),
			ledger,
		)
		require.NoError(t, err)

		f(t, vm, chain, ctx, ledger)
	}
}

func encodeJSONCDC(t *testing.T, value cadence.Value) []byte {
	b, err := jsoncdc.Encode(value)
	require.NoError(t, err)
	return b
}

func TestBlockContext_ExecuteTransaction(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	t.Run("Success", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `)).
			AddAuthorizer(unittest.AddressFixture())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.Nil(t, tx.Err)
	})

	t.Run("Failure", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  var x: Int

                  prepare(signer: AuthAccount) {
                    self.x = 0
                  }

                  execute {
                    self.x = 1
                  }

                  post {
                    self.x == 2
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.Error(t, tx.Err)
	})

	t.Run("Logs", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  execute {
                    log("foo")
                    log("bar")
                  }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(t, "\"foo\"", tx.Logs[0])
		assert.Equal(t, "\"bar\"", tx.Logs[1])
	})

	t.Run("Events", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {
                    AuthAccount(payer: signer)
                  }
                }
            `)).
			AddAuthorizer(chain.ServiceAddress())

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Events, 1)
		assert.EqualValues(t, flow.EventAccountCreated, tx.Events[0].EventType.ID())
	})
}

func TestBlockContext_DeployContract(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	t.Run("account update with set code succeeds as service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployCounterContractTransaction(accounts[0], chain)

		txBody.SetProposalKey(chain.ServiceAddress(), 0, 0)
		txBody.SetPayer(chain.ServiceAddress())

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignEnvelope(txBody, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		// Create an account private key.
		privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
		require.NoError(t, err)

		txBody := testutil.DeployUnauthorizedCounterContractTransaction(accounts[0])

		err = testutil.SignTransaction(txBody, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.Error(t, tx.Err)

		expectedErr := "Execution failed:\ncode deployment requires authorization from the service account\n"

		assert.Equal(t, expectedErr, tx.Err.Error())
		assert.Equal(t, (&fvm.ExecutionError{}).Code(), tx.Err.Code())
	})
}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	arg2, _ := jsoncdc.Encode(cadence.NewString("foo"))

	var tests = []struct {
		label       string
		script      string
		args        [][]byte
		authorizers []flow.Address
		check       func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:  "No parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:  "Single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 1)
				assert.Equal(t, "42", tx.Logs[0])
			},
		},
		{
			label:  "Multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 2)
				assert.Equal(t, "42", tx.Logs[0])
				assert.Equal(t, `"foo"`, tx.Logs[1])
			},
		},
		{
			label: "Parameters and authorizer",
			script: `
                transaction(x: Int, y: String) {
                    prepare(acct: AuthAccount) { log(acct.address) }
                    execute { log(x); log(y) }
                }`,
			args:        [][]byte{arg1, arg2},
			authorizers: []flow.Address{chain.ServiceAddress()},
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				assert.ElementsMatch(t, []string{"0x" + chain.ServiceAddress().Hex(), "42", `"foo"`}, tx.Logs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetArguments(tt.args)

			for _, authorizer := range tt.authorizers {
				txBody.AddAuthorizer(authorizer)
			}

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			tt.check(t, tx)
		})
	}
}

func gasLimitScript(depth int) string {
	return fmt.Sprintf(`
        pub fun foo(_ i: Int) {
            if i <= 0 {
                return
            }
            log("foo")
            foo(i-1)
        }

        transaction { execute { foo(%d) } }
    `, depth)
}

func TestBlockContext_ExecuteTransaction_GasLimit(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, tx *fvm.TransactionProcedure)
	}{
		{
			label:    "Zero",
			script:   gasLimitScript(100),
			gasLimit: 0,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				// gas limit of zero is ignored by runtime
				require.NoError(t, tx.Err)
			},
		},
		{
			label:    "Insufficient",
			script:   gasLimitScript(100),
			gasLimit: 5,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				assert.Error(t, tx.Err)
			},
		},
		{
			label:    "Sufficient",
			script:   gasLimitScript(100),
			gasLimit: 1000,
			check: func(t *testing.T, tx *fvm.TransactionProcedure) {
				require.NoError(t, tx.Err)
				require.Len(t, tx.Logs, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			txBody := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetGasLimit(tt.gasLimit)

			ledger := testutil.RootBootstrappedLedger(vm, ctx)

			err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
			require.NoError(t, err)

			tx := fvm.Transaction(txBody)

			err = vm.Run(ctx, tx, ledger)
			require.NoError(t, err)

			tt.check(t, tx)
		})
	}
}

var createAccountScript = []byte(`
    transaction {
        prepare(signer: AuthAccount) {
            let acct = AuthAccount(payer: signer)
        }
    }
`)

func TestBlockContext_ExecuteTransaction_CreateAccount(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys, chain)
	require.NoError(t, err)

	addAccountCreatorTemplate := `
    import FlowServiceAccount from 0x%s
    transaction {
        let serviceAccountAdmin: &FlowServiceAccount.Administrator
        prepare(signer: AuthAccount) {
            // Borrow reference to FlowServiceAccount Administrator resource.
            //
            self.serviceAccountAdmin = signer.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
                ?? panic("Unable to borrow reference to administrator resource")
        }
        execute {
            // Add account to account creator whitelist.
            //
            // Will emit AccountCreatorAdded(accountCreator: accountCreator).
            //
            self.serviceAccountAdmin.addAccountCreator(0x%s)
        }
    }
    `

	addAccountCreator := func(account flow.Address, seqNum uint64) {
		script := []byte(
			fmt.Sprintf(addAccountCreatorTemplate,
				chain.ServiceAddress().String(),
				account.String(),
			),
		)

		txBody := flow.NewTransactionBody().
			SetScript(script).
			AddAuthorizer(chain.ServiceAddress())

		err = testutil.SignTransactionAsServiceAccount(txBody, seqNum, chain)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	}

	removeAccountCreatorTemplate := `
    import FlowServiceAccount from 0x%s
    transaction {
        let serviceAccountAdmin: &FlowServiceAccount.Administrator
        prepare(signer: AuthAccount) {
            // Borrow reference to FlowServiceAccount Administrator resource.
            //
            self.serviceAccountAdmin = signer.borrow<&FlowServiceAccount.Administrator>(from: /storage/flowServiceAdmin)
                ?? panic("Unable to borrow reference to administrator resource")
        }
        execute {
            // Remove account from account creator whitelist.
            //
            // Will emit AccountCreatorRemoved(accountCreator: accountCreator).
            //
            self.serviceAccountAdmin.removeAccountCreator(0x%s)
        }
    }
    `

	removeAccountCreator := func(account flow.Address, seqNum uint64) {
		script := []byte(
			fmt.Sprintf(
				removeAccountCreatorTemplate,
				chain.ServiceAddress(),
				account.String(),
			),
		)

		txBody := flow.NewTransactionBody().
			SetScript(script).
			AddAuthorizer(chain.ServiceAddress())

		err = testutil.SignTransactionAsServiceAccount(txBody, seqNum, chain)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	}

	t.Run("Invalid account creator", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript(createAccountScript).
			AddAuthorizer(accounts[0])

		err = testutil.SignPayload(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.Error(t, tx.Err)
	})

	t.Run("Valid account creator", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript(createAccountScript).
			AddAuthorizer(chain.ServiceAddress())

		err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("Account creation succeeds when added to authorized accountCreators", func(t *testing.T) {
		addAccountCreator(accounts[0], 1)

		txBody := flow.NewTransactionBody().
			SetScript(createAccountScript).
			SetPayer(accounts[0]).
			SetProposalKey(accounts[0], 0, 0).
			AddAuthorizer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)
	})

	t.Run("Account creation fails when removed from authorized accountCreators", func(t *testing.T) {
		removeAccountCreator(accounts[0], 2)

		txBody := flow.NewTransactionBody().
			SetScript(createAccountScript).
			SetPayer(accounts[0]).
			SetProposalKey(accounts[0], 0, 0).
			AddAuthorizer(accounts[0])

		err = testutil.SignEnvelope(txBody, accounts[0], privateKeys[0])
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err := vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.Error(t, tx.Err)
	})
}

func TestBlockContext_ExecuteScript(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	t.Run("script success", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger)
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
	})

	t.Run("script failure", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                assert(1 == 2)
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger)
		assert.NoError(t, err)

		assert.Error(t, script.Err)
	})

	t.Run("script logs", func(t *testing.T) {
		code := []byte(`
            pub fun main(): Int {
                log("foo")
                log("bar")
                return 42
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(ctx, script, ledger)
		assert.NoError(t, err)

		assert.NoError(t, script.Err)
		require.Len(t, script.Logs, 2)
		assert.Equal(t, "\"foo\"", script.Logs[0])
		assert.Equal(t, "\"bar\"", script.Logs[1])
	})
}

func TestBlockContext_GetBlockInfo(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	blocks := new(fvmmock.Blocks)

	block1 := unittest.BlockFixture()
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block3 := unittest.BlockWithParentFixture(block2.Header)

	blocks.On("ByHeight", block1.Header.Height).Return(&block1, nil)
	blocks.On("ByHeight", block2.Header.Height).Return(&block2, nil)

	type logPanic struct{}
	blocks.On("ByHeight", block3.Header.Height).Run(func(args mock.Arguments) { panic(logPanic{}) })

	blockCtx := fvm.NewContextFromParent(ctx, fvm.WithBlocks(blocks), fvm.WithBlockHeader(block1.Header))

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        log(block)

                        let nextBlock = getBlock(at: block.height + UInt64(1))
                        log(nextBlock)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err = vm.Run(blockCtx, tx, ledger)
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block1.Header.Height, block1.ID(),
			float64(block1.Header.Timestamp.Unix())), tx.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Header.Height, block2.ID(),
			float64(block2.Header.Timestamp.Unix())), tx.Logs[1])
	})

	t.Run("works as script", func(t *testing.T) {
		code := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                log(block)

                let nextBlock = getBlock(at: block.height + UInt64(1))
                log(nextBlock)
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		script := fvm.Script(code)

		err := vm.Run(blockCtx, script, ledger)
		assert.NoError(t, err)

		assert.NoError(t, script.Err)

		require.Len(t, script.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block1.Header.Height, block1.ID(),
			float64(block1.Header.Timestamp.Unix())), script.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Header.Height, block2.ID(),
			float64(block2.Header.Timestamp.Unix())), script.Logs[1])
	})

	t.Run("panics if external function panics in transaction", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let block = getCurrentBlock()
                        let nextBlock = getBlock(at: block.height + UInt64(2))
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_ = vm.Run(blockCtx, fvm.Transaction(tx), ledger)
		})
	})

	t.Run("panics if external function panics in script", func(t *testing.T) {
		script := []byte(`
            pub fun main() {
                let block = getCurrentBlock()
                let nextBlock = getBlock(at: block.height + UInt64(2))
            }
        `)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_ = vm.Run(blockCtx, fvm.Script(script), ledger)
		})
	})
}

func TestBlockContext_GetAccount(t *testing.T) {
	const count = 100

	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	sequenceNumber := uint64(0)

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	createAccount := func() (flow.Address, crypto.PublicKey) {
		privateKey, txBody := testutil.CreateAccountCreationTransaction(t, chain)

		err := testutil.SignTransactionAsServiceAccount(txBody, sequenceNumber, chain)
		require.NoError(t, err)

		sequenceNumber++

		rootHasher := hash.NewSHA2_256()

		err = txBody.SignEnvelope(
			chain.ServiceAddress(),
			0,
			unittest.ServiceAccountPrivateKey.PrivateKey,
			rootHasher,
		)
		require.NoError(t, err)

		// execute the transaction
		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		require.NoError(t, err)

		assert.NoError(t, tx.Err)

		assert.Len(t, tx.Events, 2)
		assert.EqualValues(t, flow.EventAccountCreated, tx.Events[0].EventType.ID())

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		address := flow.BytesToAddress(tx.Events[0].Fields[0].(cadence.Address).Bytes())

		return address, privateKey.PublicKey(fvm.AccountKeyWeightThreshold).PublicKey
	}

	addressGen := chain.NewAddressGenerator()
	// skip the addresses of 4 reserved accounts
	for i := 0; i < 4; i++ {
		_, err := addressGen.NextAddress()
		require.NoError(t, err)
	}

	// create a bunch of accounts
	accounts := make(map[flow.Address]crypto.PublicKey, count)
	for i := 0; i < count; i++ {
		address, key := createAccount()
		expectedAddress, err := addressGen.NextAddress()
		require.NoError(t, err)

		assert.Equal(t, expectedAddress, address)
		accounts[address] = key
	}

	// happy path - get each of the created account and check if it is the right one
	t.Run("get accounts", func(t *testing.T) {
		for address, expectedKey := range accounts {

			account, err := vm.GetAccount(ctx, address, ledger)
			require.NoError(t, err)

			assert.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address, err := addressGen.NextAddress()
		require.NoError(t, err)

		var account *flow.Account
		account, err = vm.GetAccount(ctx, address, ledger)
		assert.Equal(t, fvm.ErrAccountNotFound, err)
		assert.Nil(t, account)
	})
}

func TestBlockContext_UnsafeRandom(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	header := flow.Header{Height: 42}

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache), fvm.WithBlockHeader(&header))

	t.Run("works as transaction", func(t *testing.T) {
		txBody := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                    execute {
                        let rand = unsafeRandom()
                        log(rand)
                    }
                }
            `))

		err := testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger(vm, ctx)
		require.NoError(t, err)

		tx := fvm.Transaction(txBody)

		err = vm.Run(ctx, tx, ledger)
		assert.NoError(t, err)

		assert.NoError(t, tx.Err)

		require.Len(t, tx.Logs, 1)

		num, err := strconv.ParseUint(tx.Logs[0], 10, 64)
		require.NoError(t, err)
		require.Equal(t, uint64(0xb9c618010e32a0fb), num)
	})
}

func TestBlockContext_ExecuteTransaction_CreateAccount_WithMonotonicAddresses(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.MonotonicEmulator.Chain()

	vm := fvm.New(rt)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithChain(chain), fvm.WithASTCache(cache))

	ledger := testutil.RootBootstrappedLedger(vm, ctx)

	txBody := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	err = testutil.SignTransactionAsServiceAccount(txBody, 0, chain)
	require.NoError(t, err)

	tx := fvm.Transaction(txBody)

	err = vm.Run(ctx, tx, ledger)
	assert.NoError(t, err)

	assert.NoError(t, tx.Err)

	require.Len(t, tx.Events, 1)
	require.Equal(t, string(flow.EventAccountCreated), tx.Events[0].EventType.TypeID)
	assert.Equal(t, flow.HexToAddress("05"), flow.Address(tx.Events[0].Fields[0].(cadence.Address)))
}

func TestSignatureVerification(t *testing.T) {
	code := []byte(`
      import Crypto

      pub fun main(
          rawPublicKeys: [[UInt8]],
          message: [UInt8], 
          signatures: [[UInt8]],
          weight: UFix64,
      ): Bool {
          let keyList = Crypto.KeyList()
        
          for rawPublicKey in rawPublicKeys {
              keyList.add(
                  Crypto.PublicKey(
                      publicKey: rawPublicKey,
                      signatureAlgorithm: Crypto.ECDSA_P256
                  ),
                  hashAlgorithm: Crypto.SHA3_256,
                  weight: weight,
              )
          }

          let signatureSet: [Crypto.KeyListSignature] = []

          var i = 0
          for signature in signatures {
              signatureSet.append(
                  Crypto.KeyListSignature(
                      keyIndex: i,
                      signature: signature
                  )
              )
              i = i + 1
          }

          return keyList.isValid(
              signatureSet: signatureSet,
              signedData: message,
          )
      }
    `)

	createKey := func() (privateKey crypto.PrivateKey, publicKey cadence.Array) {
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)

		var err error

		_, err = rand.Read(seed)
		require.NoError(t, err)

		privateKey, err = crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)

		publicKey = testutil.BytesToCadenceArray(
			privateKey.PublicKey().Encode(),
		)

		return privateKey, publicKey
	}

	createMessage := func(m string) (signableMessage []byte, message cadence.Array) {
		signableMessage = []byte(m)

		message = testutil.BytesToCadenceArray(signableMessage)

		return signableMessage, message
	}

	signMessage := func(privateKey crypto.PrivateKey, m []byte) cadence.Array {
		message := append(
			flow.UserDomainTag[:],
			m...,
		)

		signature, err := privateKey.Sign(message, hash.NewSHA3_256())
		require.NoError(t, err)

		return testutil.BytesToCadenceArray(signature)
	}

	t.Run("Single key", vmTest(
		func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			privateKey, publicKey := createKey()
			signableMessage, message := createMessage("foo")
			signature := signMessage(privateKey, signableMessage)
			weight, _ := cadence.NewUFix64("1.0")

			publicKeys := cadence.NewArray([]cadence.Value{
				publicKey,
			})

			signatures := cadence.NewArray([]cadence.Value{
				signature,
			})

			t.Run("Valid", func(t *testing.T) {
				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(true), script.Value)
			})

			t.Run("Invalid message", func(t *testing.T) {
				_, invalidRawMessage := createMessage("bar")

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, invalidRawMessage),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(false), script.Value)
			})

			t.Run("Invalid signature", func(t *testing.T) {
				invalidPrivateKey, _ := createKey()
				invalidRawSignature := signMessage(invalidPrivateKey, signableMessage)

				invalidRawSignatures := cadence.NewArray([]cadence.Value{
					invalidRawSignature,
				})

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, invalidRawSignatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(false), script.Value)
			})

			t.Run("Malformed public key", func(t *testing.T) {
				invalidPublicKey := testutil.BytesToCadenceArray([]byte{1, 2, 3})

				invalidPublicKeys := cadence.NewArray([]cadence.Value{
					invalidPublicKey,
				})

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, invalidPublicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				require.NoError(t, err)
				assert.Error(t, script.Err)
			})
		},
	))

	t.Run("Multiple keys", vmTest(
		func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger state.Ledger) {
			privateKeyA, publicKeyA := createKey()
			privateKeyB, publicKeyB := createKey()
			privateKeyC, publicKeyC := createKey()

			publicKeys := cadence.NewArray([]cadence.Value{
				publicKeyA,
				publicKeyB,
				publicKeyC,
			})

			signableMessage, message := createMessage("foo")

			signatureA := signMessage(privateKeyA, signableMessage)
			signatureB := signMessage(privateKeyB, signableMessage)
			signatureC := signMessage(privateKeyC, signableMessage)

			weight, _ := cadence.NewUFix64("0.5")

			t.Run("3 of 3", func(t *testing.T) {
				signatures := cadence.NewArray([]cadence.Value{
					signatureA,
					signatureB,
					signatureC,
				})

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(true), script.Value)
			})

			t.Run("2 of 3", func(t *testing.T) {
				signatures := cadence.NewArray([]cadence.Value{
					signatureA,
					signatureB,
				})

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(true), script.Value)
			})

			t.Run("1 of 3", func(t *testing.T) {
				signatures := cadence.NewArray([]cadence.Value{
					signatureA,
				})

				script := fvm.Script(code).WithArguments(
					encodeJSONCDC(t, publicKeys),
					encodeJSONCDC(t, message),
					encodeJSONCDC(t, signatures),
					encodeJSONCDC(t, weight),
				)

				err := vm.Run(ctx, script, ledger)
				assert.NoError(t, err)
				assert.NoError(t, script.Err)

				assert.Equal(t, cadence.NewBool(false), script.Value)
			})
		},
	))
}

package fvm_test

import (
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
	execTestutil "github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/fvm"
	vmMock "github.com/dapperlabs/flow-go/fvm/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func vmTest(
	f func(t *testing.T, vm *fvm.VirtualMachine, chain flow.Chain, ctx fvm.Context, ledger fvm.Ledger),
	opts ...fvm.Option,
) func(t *testing.T) {
	return func(t *testing.T) {
		rt := runtime.NewInterpreterRuntime()

		chain := flow.Mainnet.Chain()

		vm := fvm.New(rt, chain)

		cache, err := fvm.NewLRUASTCache(CacheSize)
		require.NoError(t, err)

		baseOpts := []fvm.Option{fvm.WithASTCache(cache)}
		opts = append(baseOpts, opts...)

		ctx := fvm.NewContext(opts...)

		ledger := make(fvm.MapLedger)

		_, err = vm.Invoke(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply), ledger)
		require.NoError(t, err)

		f(t, vm, chain, ctx, ledger)
	}
}

func TestBlockContext_ExecuteTransaction(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	t.Run("Success", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `)).
			AddAuthorizer(unittest.AddressFixture())

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())
		assert.Nil(t, result.Error)
	})

	t.Run("Failure", func(t *testing.T) {
		tx := flow.NewTransactionBody().
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

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.False(t, result.Succeeded())
		assert.NotNil(t, result.Error)
	})

	t.Run("Logs", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  execute {
				    log("foo")
				    log("bar")
				  }
                }
            `))

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})

	t.Run("Events", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {
				    AuthAccount(payer: signer)
				  }
                }
            `)).
			AddAuthorizer(chain.ServiceAddress())

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())
		if !assert.Nil(t, result.Error) {
			t.Log(result.Error)
		}

		require.Len(t, result.Events, 1)
		assert.EqualValues(t, "flow.AccountCreated", result.Events[0].EventType.ID())
	})
}

func TestBlockContext_DeployContract(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	t.Run("account update with set code succeeds as service account", func(t *testing.T) {
		ledger := execTestutil.RootBootstrappedLedger(chain)

		// Create an account private key.
		privateKeys, err := execTestutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := execTestutil.CreateAccounts(vm, ledger, privateKeys, chain)
		require.NoError(t, err)

		tx := execTestutil.DeployCounterContractTransaction(accounts[0], chain)

		tx.SetProposalKey(chain.ServiceAddress(), 0, 0)
		tx.SetPayer(chain.ServiceAddress())

		err = execTestutil.SignPayload(tx, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = execTestutil.SignEnvelope(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(t, err)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())

		if !assert.Nil(t, result.Error) {
			t.Log(result.Error)
		}
	})

	t.Run("account update with set code fails if not signed by service account", func(t *testing.T) {
		ledger := execTestutil.RootBootstrappedLedger(chain)

		// Create an account private key.
		privateKeys, err := execTestutil.GenerateAccountPrivateKeys(1)
		require.NoError(t, err)

		// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
		accounts, err := execTestutil.CreateAccounts(vm, ledger, privateKeys, chain)
		require.NoError(t, err)

		tx := execTestutil.DeployUnauthorizedCounterContractTransaction(accounts[0])

		err = execTestutil.SignTransaction(tx, accounts[0], privateKeys[0], 0)
		require.NoError(t, err)

		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.False(t, result.Succeeded())
		assert.NotNil(t, result.Error)

		expectedErr := "Execution failed:\ncode deployment requires authorization from the service account\n"

		assert.Equal(t, expectedErr, result.Error.Error())
		assert.Equal(t, (&fvm.ExecutionError{}).Code(), result.Error.Code())
	})
}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	arg2, _ := jsoncdc.Encode(cadence.NewString("foo"))

	var tests = []struct {
		label       string
		script      string
		args        [][]byte
		authorizers []flow.Address
		check       func(t *testing.T, result *fvm.InvocationResult)
	}{
		{
			label:  "No parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, result *fvm.InvocationResult) {
				assert.NotNil(t, result.Error)
			},
		},
		{
			label:  "Single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, result *fvm.InvocationResult) {
				require.Nil(t, result.Error)
				require.Len(t, result.Logs, 1)
				assert.Equal(t, "42", result.Logs[0])
			},
		},
		{
			label:  "Multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, result *fvm.InvocationResult) {
				require.Nil(t, result.Error)
				require.Len(t, result.Logs, 2)
				assert.Equal(t, "42", result.Logs[0])
				assert.Equal(t, `"foo"`, result.Logs[1])
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
			check: func(t *testing.T, result *fvm.InvocationResult) {
				require.Nil(t, result.Error)
				assert.ElementsMatch(t, []string{"0x" + chain.ServiceAddress().Hex(), "42", `"foo"`}, result.Logs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			tx := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetArguments(tt.args)

			for _, authorizer := range tt.authorizers {
				tx.AddAuthorizer(authorizer)
			}

			ledger := execTestutil.RootBootstrappedLedger(chain)

			err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
			require.NoError(t, err)

			result, err := vm.Invoke(ctx, fvm.Transaction(tx), ledger)
			require.NoError(t, err)

			tt.check(t, result)
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

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, result *fvm.InvocationResult)
	}{
		{
			label:    "Zero",
			script:   gasLimitScript(100),
			gasLimit: 0,
			check: func(t *testing.T, result *fvm.InvocationResult) {
				// gas limit of zero is ignored by runtime
				require.Nil(t, result.Error)
			},
		},
		{
			label:    "Insufficient",
			script:   gasLimitScript(100),
			gasLimit: 5,
			check: func(t *testing.T, result *fvm.InvocationResult) {
				assert.NotNil(t, result.Error)
			},
		},
		{
			label:    "Sufficient",
			script:   gasLimitScript(100),
			gasLimit: 1000,
			check: func(t *testing.T, result *fvm.InvocationResult) {
				require.Nil(t, result.Error)
				require.Len(t, result.Logs, 100)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			tx := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetGasLimit(tt.gasLimit)

			ledger := execTestutil.RootBootstrappedLedger(chain)

			err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
			require.NoError(t, err)

			result, err := vm.Invoke(ctx, fvm.Transaction(tx), ledger)
			require.NoError(t, err)

			tt.check(t, result)
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

func TestBlockContext_ExecuteScript(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	t.Run("script success", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		result, err := vm.Invoke(ctx, fvm.Script(script), ledger)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
	})

	t.Run("script failure", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				assert 1 == 2
				return 42
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		result, err := vm.Invoke(ctx, fvm.Script(script), ledger)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.NotNil(t, result.Error)
	})

	t.Run("script logs", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				log("foo")
				log("bar")
				return 42
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		result, err := vm.Invoke(ctx, fvm.Script(script), ledger)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})
}

func TestBlockContext_GetBlockInfo(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	blocks := new(vmMock.Blocks)

	block1 := unittest.BlockFixture()
	block2 := unittest.BlockWithParentFixture(block1.Header)
	block3 := unittest.BlockWithParentFixture(block2.Header)

	blocks.On("ByHeight", block1.Header.Height).Return(&block1, nil)
	blocks.On("ByHeight", block2.Header.Height).Return(&block2, nil)

	type logPanic struct{}
	blocks.On("ByHeight", block3.Header.Height).Run(func(args mock.Arguments) { panic(logPanic{}) })

	blockCtx := fvm.NewContextFromParent(ctx, fvm.WithBlocks(blocks), fvm.WithBlockHeader(block1.Header))

	t.Run("works as transaction", func(t *testing.T) {
		tx := flow.NewTransactionBody().
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

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)
		require.NoError(t, err)

		result, err := vm.Invoke(blockCtx, fvm.Transaction(tx), ledger)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block1.Header.Height, block1.ID(),
			float64(block1.Header.Timestamp.Unix())), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Header.Height, block2.ID(),
			float64(block2.Header.Timestamp.Unix())), result.Logs[1])
	})

	t.Run("works as script", func(t *testing.T) {
		script := []byte(`
			pub fun main() {
				let block = getCurrentBlock()
				log(block)

				let nextBlock = getBlock(at: block.height + UInt64(1))
				log(nextBlock)
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		result, err := vm.Invoke(blockCtx, fvm.Script(script), ledger)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())

		require.Len(t, result.Logs, 2)
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block1.Header.Height, block1.ID(),
			float64(block1.Header.Timestamp.Unix())), result.Logs[0])
		assert.Equal(t, fmt.Sprintf("Block(height: %v, id: 0x%x, timestamp: %.8f)", block2.Header.Height, block2.ID(),
			float64(block2.Header.Timestamp.Unix())), result.Logs[1])
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

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)
		require.NoError(t, err)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_, _ = vm.Invoke(blockCtx, fvm.Transaction(tx), ledger)
		})
	})

	t.Run("panics if external function panics in script", func(t *testing.T) {
		script := []byte(`
			pub fun main() {
				let block = getCurrentBlock()
				let nextBlock = getBlock(at: block.height + UInt64(2))
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger(chain)

		assert.PanicsWithValue(t, interpreter.ExternalError{
			Recovered: logPanic{},
		}, func() {
			_, _ = vm.Invoke(blockCtx, fvm.Script(script), ledger)
		})
	})
}

func TestBlockContext_GetAccount(t *testing.T) {
	const count = 100

	rt := runtime.NewInterpreterRuntime()

	chain := flow.Mainnet.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	sequenceNumber := uint64(0)

	ledger := execTestutil.RootBootstrappedLedger(chain)

	createAccount := func() (flow.Address, crypto.PublicKey) {
		privateKey, tx := execTestutil.CreateAccountCreationTransaction(t, chain)

		err := execTestutil.SignTransactionAsServiceAccount(tx, sequenceNumber, chain)
		require.NoError(t, err)

		sequenceNumber++

		rootHasher := hash.NewSHA2_256()

		err = tx.SignEnvelope(
			chain.ServiceAddress(),
			0,
			unittest.ServiceAccountPrivateKey.PrivateKey,
			rootHasher,
		)
		require.NoError(t, err)

		// execute the transaction
		var result *fvm.InvocationResult
		result, err = vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())
		if !assert.Nil(t, result.Error) {
			t.Log(result.Error)
		}

		assert.Len(t, result.Events, 2)
		assert.EqualValues(t, flow.EventAccountCreated, result.Events[0].EventType.ID())

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		address := flow.BytesToAddress(result.Events[0].Fields[0].(cadence.Address).Bytes())

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

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	header := flow.Header{Height: 42}

	ctx := fvm.NewContext(fvm.WithASTCache(cache), fvm.WithBlockHeader(&header))

	t.Run("works as transaction", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
				transaction {
					execute {
						let rand = unsafeRandom()
						log(rand)
					}
				}
			`))

		err := execTestutil.SignTransactionAsServiceAccount(tx, 0, chain)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger(chain)
		require.NoError(t, err)

		result, err := vm.Invoke(ctx, fvm.Transaction(tx), ledger)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())

		require.Len(t, result.Logs, 1)

		num, err := strconv.ParseUint(result.Logs[0], 10, 64)
		require.NoError(t, err)
		require.Equal(t, uint64(0xb9c618010e32a0fb), num)
	})
}

func TestBlockContext_ExecuteTransaction_CreateAccount_WithMonotonicAddresses(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	chain := flow.Emulator.Chain()

	vm := fvm.New(rt, chain)

	cache, err := fvm.NewLRUASTCache(CacheSize)
	require.NoError(t, err)

	ctx := fvm.NewContext(fvm.WithASTCache(cache))

	ledger := execTestutil.RootBootstrappedLedger(chain)

	validTx := flow.NewTransactionBody().
		SetScript(createAccountScript).
		AddAuthorizer(chain.ServiceAddress())

	err = execTestutil.SignTransactionAsServiceAccount(validTx, 0, chain)
	require.NoError(t, err)

	result, err := vm.Invoke(ctx, fvm.Transaction(validTx), ledger)
	require.NoError(t, err)

	if !assert.True(t, result.Succeeded()) {
		t.Fatal(result.Error.Error())
	}

	require.Len(t, result.Events, 1)
	require.Equal(t, string(flow.EventAccountCreated), result.Events[0].EventType.TypeID)
	assert.Equal(t, flow.HexToAddress("05"), flow.Address(result.Events[0].Fields[0].(cadence.Address)))
}

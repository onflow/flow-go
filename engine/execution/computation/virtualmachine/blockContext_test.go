package virtualmachine_test

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	execTestutil "github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockContext_ExecuteTransaction(t *testing.T) {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("transaction success", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `)).
			AddAuthorizer(unittest.AddressFixture())

		err := execTestutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.Nil(t, result.Error)
	})

	t.Run("transaction failure", func(t *testing.T) {
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

		err := execTestutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.NotNil(t, result.Error)
	})

	t.Run("transaction logs", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  execute {
				    log("foo")
				    log("bar")
				  }
                }
            `))

		err := execTestutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})

	t.Run("transaction events", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {
				    AuthAccount(payer: signer)
				  }
                }
            `)).
			AddAuthorizer(flow.RootAddress)

		err := execTestutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())
		if !assert.Nil(t, result.Error) {
			t.Log(result.Error.ErrorMessage())
		}

		require.Len(t, result.Events, 1)
		assert.EqualValues(t, "flow.AccountCreated", result.Events[0].EventType.ID())
	})
}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	assert.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	arg2, _ := jsoncdc.Encode(cadence.NewString("foo"))

	var tests = []struct {
		label       string
		script      string
		args        [][]byte
		authorizers []flow.Address
		check       func(t *testing.T, result *virtualmachine.TransactionResult)
	}{
		{
			label:  "no parameters",
			script: `transaction { execute { log("Hello, World!") } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				assert.NotNil(t, result.Error)
			},
		},
		{
			label:  "single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.Nil(t, result.Error)
				require.Len(t, result.Logs, 1)
				assert.Equal(t, "42", result.Logs[0])
			},
		},
		{
			label:  "multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.Nil(t, result.Error)
				require.Len(t, result.Logs, 2)
				assert.Equal(t, "42", result.Logs[0])
				assert.Equal(t, `"foo"`, result.Logs[1])
			},
		},
		{
			label: "parameters and authorizer",
			script: `
				transaction(x: Int, y: String) {
					prepare(acct: AuthAccount) { log(acct.address) }
					execute { log(x); log(y) }
				}`,
			args:        [][]byte{arg1, arg2},
			authorizers: []flow.Address{flow.RootAddress},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.Nil(t, result.Error)
				assert.ElementsMatch(t, []string{"0x1", "42", `"foo"`}, result.Logs)
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

			ledger := execTestutil.RootBootstrappedLedger()

			err = execTestutil.SignTransactionByRoot(tx, 0)
			require.NoError(t, err)

			result, err := bc.ExecuteTransaction(ledger, tx)
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

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	assert.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	var tests = []struct {
		label    string
		script   string
		gasLimit uint64
		check    func(t *testing.T, result *virtualmachine.TransactionResult)
	}{
		{
			label:    "zero",
			script:   gasLimitScript(100), // 100 function calls
			gasLimit: 0,
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				// gas limit of zero is ignored by runtime
				require.Nil(t, result.Error)
			},
		},
		{
			label:    "insufficient",
			script:   gasLimitScript(100), // 100 function calls
			gasLimit: 5,
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				assert.NotNil(t, result.Error)
			},
		},
		{
			label:    "sufficient",
			script:   gasLimitScript(100), // 100 function calls
			gasLimit: 1000,
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
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

			ledger := execTestutil.RootBootstrappedLedger()

			err = execTestutil.SignTransactionByRoot(tx, 0)
			require.NoError(t, err)

			result, err := bc.ExecuteTransaction(ledger, tx)
			require.NoError(t, err)

			tt.check(t, result)
		})
	}
}

func TestBlockContext_ExecuteTransaction_CreateAccount(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	assert.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	privateKeys, err := execTestutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	ledger := execTestutil.RootBootstrappedLedger()
	accounts, err := execTestutil.CreateAccounts(vm, ledger, privateKeys)
	require.NoError(t, err)

	createAccountScript := []byte(`
	  transaction {
	    prepare(signer: AuthAccount) {
	  	  let acct = AuthAccount(payer: signer)
	    }
	  }
	`)

	t.Run("Invalid account creator", func(t *testing.T) {
		invalidTx := flow.NewTransactionBody().
			SetScript(createAccountScript).
			AddAuthorizer(accounts[0])

		err = execTestutil.SignPayload(invalidTx, accounts[0], privateKeys[0])
		require.NoError(t, err)

		err = execTestutil.SignTransactionByRoot(invalidTx, 0)
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, invalidTx)
		require.NoError(t, err)

		assert.False(t, result.Succeeded())
	})

	t.Run("Valid account creator", func(t *testing.T) {
		validTx := flow.NewTransactionBody().
			SetScript(createAccountScript).
			AddAuthorizer(flow.RootAddress)

		err = execTestutil.SignTransactionByRoot(validTx, 0)
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, validTx)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())
	})
}

func TestBlockContext_ExecuteScript(t *testing.T) {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	t.Run("script success", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteScript(ledger, script)
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

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteScript(ledger, script)

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

		ledger := execTestutil.RootBootstrappedLedger()

		result, err := bc.ExecuteScript(ledger, script)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})
}

func TestBlockContext_GetAccount(t *testing.T) {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())
	count := 10
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h)

	sequenceNumber := 0

	ledger := execTestutil.RootBootstrappedLedger()

	ledgerAccess := virtualmachine.NewLedgerDAL(ledger)

	createAccount := func() (flow.Address, crypto.PublicKey) {

		// create a random seed for the key
		seed := make([]byte, 48)
		_, err := rand.Read(seed)
		require.Nil(t, err)

		// generate a unique key
		key, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		assert.NoError(t, err)

		// get the key bytes
		accountKey := flow.AccountPublicKey{
			PublicKey: key.PublicKey(),
			SignAlgo:  key.Algorithm(),
			HashAlgo:  hash.SHA3_256,
		}
		keyBytes, err := flow.EncodeRuntimeAccountPublicKey(accountKey)
		assert.NoError(t, err)

		// define the cadence script
		script := fmt.Sprintf(`
			transaction {
			  prepare(signer: AuthAccount) {
				let acct = AuthAccount(payer: signer)
				acct.addPublicKey("%s".decodeHex())
			  }
			}
		`, hex.EncodeToString(keyBytes))

		// create the transaction to create the account
		tx := flow.NewTransactionBody().
			SetScript([]byte(script)).
			SetPayer(flow.RootAddress).
			SetProposalKey(flow.RootAddress, 0, uint64(sequenceNumber)).
			AddAuthorizer(flow.RootAddress)

		sequenceNumber++

		rootHasher, err := hash.NewHasher(unittest.RootAccountPrivateKey.HashAlgo)
		require.NoError(t, err)

		err = tx.SignEnvelope(flow.RootAddress, 0, unittest.RootAccountPrivateKey.PrivateKey, rootHasher)
		require.NoError(t, err)

		// execute the transaction
		result, err := bc.ExecuteTransaction(ledger, tx)
		require.NoError(t, err)

		assert.True(t, result.Succeeded())
		if !assert.Nil(t, result.Error) {
			t.Log(result.Error.ErrorMessage())
		}

		assert.Len(t, result.Events, 2)
		assert.EqualValues(t, flow.EventAccountCreated, result.Events[0].EventType.ID())

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		address := flow.BytesToAddress(result.Events[0].Fields[0].(cadence.Address).Bytes())

		return address, key.PublicKey()
	}

	// create a bunch of accounts
	accounts := make(map[flow.Address]crypto.PublicKey, count)
	for i := 0; i < count; i++ {
		address, key := createAccount()
		accounts[address] = key
	}

	// happy path - get each of the created account and check if it is the right one
	t.Run("get accounts", func(t *testing.T) {
		for address, expectedKey := range accounts {

			account := ledgerAccess.GetAccount(address)

			assert.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address := flow.HexToAddress(fmt.Sprintf("%d", count+1))
		account := ledgerAccess.GetAccount(address)
		assert.Nil(t, account)
	})
}

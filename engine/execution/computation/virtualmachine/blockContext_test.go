package virtualmachine_test

import (
	"fmt"
	"math/rand"
	"strings"
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

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	t.Run("transaction success", func(t *testing.T) {
		tx := flow.NewTransactionBody().
			SetScript([]byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `)).
			AddAuthorizer(unittest.AddressFixture())

		err := execTestutil.SignTransactionbyRoot(tx, 0)
		require.NoError(t, err)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)
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

		err := execTestutil.SignTransactionbyRoot(tx, 0)
		require.NoError(t, err)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
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

		err := execTestutil.SignTransactionbyRoot(tx, 0)
		require.NoError(t, err)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

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
                  execute {
				    AuthAccount(publicKeys: [], code: [])
				  }
                }
            `))

		err := execTestutil.SignTransactionbyRoot(tx, 0)
		require.NoError(t, err)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)

		require.Len(t, result.Events, 1)
		assert.EqualValues(t, "flow.AccountCreated", result.Events[0].EventType.ID())
	})
}

func TestBlockContext_ExecuteTransaction_WithArguments(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	arg1, _ := jsoncdc.Encode(cadence.NewInt(42))
	arg2, _ := jsoncdc.Encode(cadence.NewString("foo"))

	var transactionArgsTests = []struct {
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
				assert.Error(t, result.Error)
			},
		},
		{
			label:  "single parameter",
			script: `transaction(x: Int) { execute { log(x) } }`,
			args:   [][]byte{arg1},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.NoError(t, result.Error)
				require.Len(t, result.Logs, 1)
				assert.Equal(t, "42", result.Logs[0])
			},
		},
		{
			label:  "multiple parameters",
			script: `transaction(x: Int, y: String) { execute { log(x); log(y) } }`,
			args:   [][]byte{arg1, arg2},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.NoError(t, result.Error)
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
			authorizers: []flow.Address{flow.HexToAddress("01")},
			check: func(t *testing.T, result *virtualmachine.TransactionResult) {
				require.NoError(t, result.Error)
				assert.ElementsMatch(t, []string{"0x1", "42", `"foo"`}, result.Logs)
			},
		},
	}

	for _, tt := range transactionArgsTests {
		t.Run(tt.label, func(t *testing.T) {
			tx := flow.NewTransactionBody().
				SetScript([]byte(tt.script)).
				SetArguments(tt.args)

			for _, authorizer := range tt.authorizers {
				tx.AddAuthorizer(authorizer)
			}

			ledger, err := execTestutil.RootBootstrappedLedger()
			require.NoError(t, err)

			err = execTestutil.SignTransactionbyRoot(tx, 0)
			require.NoError(t, err)
			//seq++

			result, err := bc.ExecuteTransaction(ledger, tx)
			require.NoError(t, err)

			tt.check(t, result)
		})
	}
}

func TestBlockContext_ExecuteScript(t *testing.T) {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())
	rt := runtime.NewInterpreterRuntime()

	h := unittest.BlockHeaderFixture()

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	t.Run("script success", func(t *testing.T) {
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
	})

	t.Run("script failure", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				assert 1 == 2
				return 42
			}
		`)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

		result, err := bc.ExecuteScript(ledger, script)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})

	t.Run("script logs", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				log("foo")
				log("bar")
				return 42
			}
		`)

		ledger, err := execTestutil.RootBootstrappedLedger()
		require.NoError(t, err)

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

	vm := virtualmachine.New(rt)
	bc := vm.NewBlockContext(&h)

	sequenceNumber := 0

	ledger, err := execTestutil.RootBootstrappedLedger()
	require.NoError(t, err)

	ledgerAccess := virtualmachine.LedgerAccess{Ledger: ledger}

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

		// encode the bytes to cadence string
		encodedKey := languageEncodeBytesArray(keyBytes)

		// define the cadence script
		script := fmt.Sprintf(`
                transaction {
                  execute {
				    AuthAccount(publicKeys: %s, code: [])
				  }
                }
            `,
			encodedKey)

		// create the transaction to create the account
		tx := &flow.TransactionBody{
			Script: []byte(script),
			Payer:  flow.RootAddress,
			ProposalKey: flow.ProposalKey{
				Address:        flow.RootAddress,
				KeyID:          0,
				SequenceNumber: uint64(sequenceNumber),
			},
		}

		sequenceNumber++

		rootHasher, err := hash.NewHasher(flow.RootAccountPrivateKey.HashAlgo)
		require.NoError(t, err)

		err = tx.SignEnvelope(flow.RootAddress, 0, flow.RootAccountPrivateKey.PrivateKey, rootHasher)
		require.NoError(t, err)

		// execute the transaction
		result, err := bc.ExecuteTransaction(ledger, tx)
		require.NoError(t, err)
		require.True(t, result.Succeeded())
		require.NoError(t, result.Error)
		require.Len(t, result.Events, 1)
		require.EqualValues(t, flow.EventAccountCreated, result.Events[0].EventType.ID())

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

func languageEncodeBytesArray(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return strings.Join(strings.Fields(fmt.Sprintf("%d", [][]byte{b})), ",")
}

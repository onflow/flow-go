package virtualmachine_test

import (
	"crypto/rand"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dapperlabs/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
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
		tx := &flow.TransactionBody{
			ScriptAccounts: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)
	})

	t.Run("transaction failure", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
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
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)

		assert.NoError(t, err)
		assert.False(t, result.Succeeded())
		assert.Error(t, result.Error)
	})

	t.Run("transaction logs", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
                transaction {
                  execute {
				    log("foo")
				    log("bar")
				  }
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		require.Len(t, result.Logs, 2)
		assert.Equal(t, "\"foo\"", result.Logs[0])
		assert.Equal(t, "\"bar\"", result.Logs[1])
	})

	t.Run("transaction events", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Script: []byte(`
                transaction {
                  execute {
				    AuthAccount(publicKeys: [], code: [])
				  }
                }
            `),
		}

		ledger := make(virtualmachine.MapLedger)

		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)

		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)

		require.Len(t, result.Events, 1)
		assert.EqualValues(t, "flow.AccountCreated", result.Events[0].Type.ID())
	})
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

		ledger := make(virtualmachine.MapLedger)

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

		ledger := make(virtualmachine.MapLedger)

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

		ledger := make(virtualmachine.MapLedger)

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

	ledger := make(virtualmachine.MapLedger)

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
		keyBytes, err := flow.EncodeAccountPublicKey(accountKey)
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
		}

		// execute the transaction
		result, err := bc.ExecuteTransaction(ledger, tx)
		assert.NoError(t, err)
		assert.True(t, result.Succeeded())
		assert.NoError(t, result.Error)
		require.Len(t, result.Events, 1)
		assert.EqualValues(t, flow.EventAccountCreated, result.Events[0].Type.ID())

		// read the address of the account created (e.g. "0x01" and convert it to flow.address)
		value := fmt.Sprintf("%v", result.Events[0].Fields[0].Value)
		address := flow.HexToAddress(fmt.Sprintf("0%s", value[2:]))

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

			account := bc.GetAccount(ledger, address)

			assert.Len(t, account.Keys, 1)
			actualKey := account.Keys[0].PublicKey
			assert.Equal(t, expectedKey, actualKey)
		}
	})

	// non-happy path - get an account that was never created
	t.Run("get a non-existing account", func(t *testing.T) {
		address := flow.HexToAddress(fmt.Sprintf("%d", count+1))
		account := bc.GetAccount(ledger, address)
		assert.Nil(t, account)
	})
}

func languageEncodeBytesArray(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return strings.Join(strings.Fields(fmt.Sprintf("%d", [][]byte{b})), ",")
}

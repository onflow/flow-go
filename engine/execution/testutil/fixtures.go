package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func CreateContractDeploymentTransaction(contract string, authorizer flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.setCode("%s".decodeHex())
              }
            }`, encoded)),
		).
		AddAuthorizer(authorizer)
}

func SignPayload(
	tx *flow.TransactionBody,
	account flow.Address,
	privateKey flow.AccountPrivateKey,
) error {
	hasher, err := hash.NewHasher(privateKey.HashAlgo)
	if err != nil {
		return fmt.Errorf("failed to create hasher: %w", err)
	}

	err = tx.SignPayload(account, 0, privateKey.PrivateKey, hasher)

	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	return nil
}

func SignEnvelope(tx *flow.TransactionBody, account flow.Address, privateKey flow.AccountPrivateKey) error {
	hasher, err := hash.NewHasher(privateKey.HashAlgo)
	if err != nil {
		return fmt.Errorf("failed to create hasher: %w", err)
	}

	err = tx.SignEnvelope(account, 0, privateKey.PrivateKey, hasher)

	if err != nil {
		return fmt.Errorf("failed to sign transaction: %w", err)
	}

	return nil
}

func SignTransaction(
	tx *flow.TransactionBody,
	address flow.Address,
	privateKey flow.AccountPrivateKey,
	seqNum uint64,
) error {
	tx.SetProposalKey(address, 0, seqNum)
	tx.SetPayer(address)
	return SignEnvelope(tx, address, privateKey)
}

func SignTransactionByRoot(tx *flow.TransactionBody, seqNum uint64) error {
	return SignTransaction(tx, flow.ServiceAddress(), unittest.ServiceAccountPrivateKey, seqNum)
}

// Generate a number of private keys
func GenerateAccountPrivateKeys(numKeys int) ([]flow.AccountPrivateKey, error) {
	var privateKeys []flow.AccountPrivateKey

	for i := 0; i < numKeys; i++ {
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)

		_, err := rand.Read(seed)
		if err != nil {
			return nil, err
		}

		privateKey, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		if err != nil {
			return nil, err
		}

		flowPrivateKey := flow.AccountPrivateKey{
			PrivateKey: privateKey,
			SignAlgo:   crypto.ECDSAP256,
			HashAlgo:   hash.SHA2_256,
		}

		privateKeys = append(privateKeys, flowPrivateKey)
	}

	return privateKeys, nil
}

// CreateAccounts inserts accounts into the ledger using the provided private keys.
func CreateAccounts(
	vm virtualmachine.VirtualMachine,
	ledger virtualmachine.Ledger,
	privateKeys []flow.AccountPrivateKey,
) ([]flow.Address, error) {
	ctx := vm.NewBlockContext(nil)

	var accounts []flow.Address

	script := []byte(`
	  transaction(publicKey: [Int]) {
	    prepare(signer: AuthAccount) {
	  	  let acct = AuthAccount(payer: signer)
	  	  acct.addPublicKey(publicKey)
	    }
	  }
	`)

	for _, privateKey := range privateKeys {
		accountKey := privateKey.PublicKey(virtualmachine.AccountKeyWeightThreshold)
		encAccountKey, _ := flow.EncodeRuntimeAccountPublicKey(accountKey)
		cadAccountKey := bytesToCadenceArray(encAccountKey)
		encCadAccountKey, _ := jsoncdc.Encode(cadAccountKey)

		tx := flow.NewTransactionBody().
			SetScript(script).
			AddArgument(encCadAccountKey).
			AddAuthorizer(flow.ServiceAddress())

		result, err := ctx.ExecuteTransaction(ledger, tx, virtualmachine.SkipVerification)
		if err != nil {
			return nil, err
		}

		if result.Error != nil {
			return nil, fmt.Errorf("failed to create account: %s", result.Error.ErrorMessage())
		}

		var addr flow.Address

		for _, event := range result.Events {
			if event.EventType.ID() == string(flow.EventAccountCreated) {
				addr = event.Fields[0].ToGoValue().([8]byte)
				break
			}

			return nil, errors.New("no account creation event emitted")
		}

		accounts = append(accounts, addr)
	}

	return accounts, nil
}

func RootBootstrappedLedger() virtualmachine.Ledger {
	ledger := make(virtualmachine.MapLedger)
	bootstrap.BootstrapView(ledger, unittest.ServiceAccountPublicKey, unittest.GenesisTokenSupply)
	return ledger
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewInt(int(b))
	}

	return cadence.NewArray(values)
}

// CreateCreateAccountTransaction creates a transaction which will create a new account and returns the randomly
// generated private key and the transaction
func CreateCreateAccountTransaction(
	t *testing.T,
	code []byte,
	authorizers []flow.Address,
) (crypto.PrivateKey, *flow.TransactionBody) {
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

	prepare := "prepare("
	for i := range authorizers {
		prepare += fmt.Sprintf("signer%v: AuthAccount", i)
	}
	prepare += ") { }"

	// define the cadence script
	script := fmt.Sprintf(`
			transaction {
			  %s
			  execute {
			    AuthAccount(publicKeys: %s, code: "%s".decodeHex())
			  }
			}
		`,
		prepare, encodedKey, hex.EncodeToString(code))

	// create the transaction to create the account
	tx := flow.TransactionBody{
		Script:      []byte(script),
		Authorizers: authorizers,
	}

	return key, &tx
}

func languageEncodeBytesArray(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return strings.Join(strings.Fields(fmt.Sprintf("%d", [][]byte{b})), ",")
}

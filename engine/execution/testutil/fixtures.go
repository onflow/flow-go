package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func CreateContractDeploymentTransaction(contractName string, contract string, authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, contractName, encoded)),
		).
		AddAuthorizer(authorizer).
		AddAuthorizer(chain.ServiceAddress())
}

func UpdateContractDeploymentTransaction(contractName string, contract string, authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.contracts.update__experimental(name: "%s", code: "%s".decodeHex())
              }
            }`, contractName, encoded)),
		).
		AddAuthorizer(authorizer).
		AddAuthorizer(chain.ServiceAddress())
}

func CreateUnauthorizedContractDeploymentTransaction(contractName string, contract string, authorizer flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, contractName, encoded)),
		).
		AddAuthorizer(authorizer)
}

func SignPayload(
	tx *flow.TransactionBody,
	account flow.Address,
	privateKey flow.AccountPrivateKey,
) error {
	hasher, err := utils.NewHasher(privateKey.HashAlgo)
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
	hasher, err := utils.NewHasher(privateKey.HashAlgo)
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

func SignTransactionAsServiceAccount(tx *flow.TransactionBody, seqNum uint64, chain flow.Chain) error {
	return SignTransaction(tx, chain.ServiceAddress(), unittest.ServiceAccountPrivateKey, seqNum)
}

// GenerateAccountPrivateKeys generates a number of private keys.
func GenerateAccountPrivateKeys(numberOfPrivateKeys int) ([]flow.AccountPrivateKey, error) {
	var privateKeys []flow.AccountPrivateKey
	for i := 0; i < numberOfPrivateKeys; i++ {
		pk, err := GenerateAccountPrivateKey()
		if err != nil {
			return nil, err
		}
		privateKeys = append(privateKeys, pk)
	}

	return privateKeys, nil
}

// GenerateAccountPrivateKey generates a private key.
func GenerateAccountPrivateKey() (flow.AccountPrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
	_, err := rand.Read(seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}
	privateKey, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}
	pk := flow.AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   crypto.ECDSAP256,
		HashAlgo:   hash.SHA2_256,
	}
	return pk, nil
}

// CreateAccounts inserts accounts into the ledger using the provided private keys.
func CreateAccounts(
	vm *fvm.VirtualMachine,
	ledger state.Ledger,
	programs *fvm.Programs,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) ([]flow.Address, error) {
	return CreateAccountsWithSimpleAddresses(vm, ledger, programs, privateKeys, chain)
}

func CreateAccountsWithSimpleAddresses(
	vm *fvm.VirtualMachine,
	ledger state.Ledger,
	programs *fvm.Programs,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) ([]flow.Address, error) {
	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvocator(zerolog.Nop()),
		),
	)

	var accounts []flow.Address

	script := []byte(`
	  transaction(publicKey: [UInt8]) {
	    prepare(signer: AuthAccount) {
	  	  let acct = AuthAccount(payer: signer)
	  	  acct.addPublicKey(publicKey)
	    }
	  }
	`)

	serviceAddress := chain.ServiceAddress()

	for i, privateKey := range privateKeys {
		accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)
		encAccountKey, _ := flow.EncodeRuntimeAccountPublicKey(accountKey)
		cadAccountKey := BytesToCadenceArray(encAccountKey)
		encCadAccountKey, _ := jsoncdc.Encode(cadAccountKey)

		txBody := flow.NewTransactionBody().
			SetScript(script).
			AddArgument(encCadAccountKey).
			AddAuthorizer(serviceAddress)

		tx := fvm.Transaction(txBody, uint32(i))
		err := vm.Run(ctx, tx, ledger, programs)
		if err != nil {
			return nil, err
		}

		if tx.Err != nil {
			return nil, fmt.Errorf("failed to create account: %w", tx.Err)
		}

		var addr flow.Address

		for _, event := range tx.Events {
			if event.Type == flow.EventAccountCreated {
				data, err := jsoncdc.Decode(event.Payload)
				if err != nil {
					return nil, errors.New("error decoding events")
				}
				addr = flow.Address(data.(cadence.Event).Fields[0].(cadence.Address))
				break
			}

			return nil, errors.New("no account creation event emitted")
		}

		accounts = append(accounts, addr)
	}

	return accounts, nil
}

func RootBootstrappedLedger(vm *fvm.VirtualMachine, ctx fvm.Context) *state.MapLedger {
	ledger := state.NewMapLedger()
	programs := fvm.NewEmptyPrograms()

	bootstrap := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	)

	_ = vm.Run(
		ctx,
		bootstrap,
		ledger,
		programs,
	)

	return ledger
}

func BytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
}

// CreateAccountCreationTransaction creates a transaction which will create a new account.
//
// This function returns a randomly generated private key and the transaction.
func CreateAccountCreationTransaction(t *testing.T, chain flow.Chain) (flow.AccountPrivateKey, *flow.TransactionBody) {
	accountKey, err := GenerateAccountPrivateKey()
	require.NoError(t, err)

	keyBytes, err := flow.EncodeRuntimeAccountPublicKey(accountKey.PublicKey(1000))
	require.NoError(t, err)

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
		AddAuthorizer(chain.ServiceAddress())

	return accountKey, tx
}

// CreateAddAccountKeyTransaction generates a tx that adds a key to an account.
func CreateAddAccountKeyTransaction(t *testing.T, accountKey *flow.AccountPrivateKey) *flow.TransactionBody {
	keyBytes, err := flow.EncodeRuntimeAccountPublicKey(accountKey.PublicKey(1000))
	require.NoError(t, err)

	// encode the bytes to cadence string
	encodedKey := languageEncodeBytes(keyBytes)

	script := fmt.Sprintf(`
        transaction {
          prepare(signer: AuthAccount) {
            signer.addPublicKey(%s)
          }
        }
   	`, encodedKey)

	return &flow.TransactionBody{
		Script: []byte(script),
	}
}

// CreateRemoveAccountKeyTransaction generates a tx that removes a key from an account.
func CreateRemoveAccountKeyTransaction(index int) *flow.TransactionBody {
	script := fmt.Sprintf(`
		transaction {
		  prepare(signer: AuthAccount) {
	    	signer.removePublicKey(%d)
		  }
		}
	`, index)

	return &flow.TransactionBody{
		Script: []byte(script),
	}
}

func languageEncodeBytes(b []byte) string {
	if len(b) == 0 {
		return "[]"
	}
	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

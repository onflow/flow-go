package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	fvmUtils "github.com/onflow/flow-go/fvm/utils"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/utils/unittest"
)

func CreateContractDeploymentTransaction(contractName string, contract string, authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {

	encoded := hex.EncodeToString([]byte(contract))

	script := []byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.contracts.add(name: "%s", code: "%s".decodeHex())
              }
            }`, contractName, encoded))

	txBody := flow.NewTransactionBody().
		SetScript(script).
		AddAuthorizer(authorizer).
		AddAuthorizer(chain.ServiceAddress())

	// to synthetically generate event using Cadence code we would need a lot of
	// copying, so its easier to just hardcode the json string
	// TODO - extract parts of Cadence to make exporting events easy without interpreter

	interpreterHash := runtime.CodeToHashValue(script)
	hashElements := interpreterHash.Elements()

	valueStrings := make([]string, len(hashElements))

	for i, value := range hashElements {
		uint8 := value.(interpreter.UInt8Value)
		valueStrings[i] = fmt.Sprintf("{\"type\":\"UInt8\",\"value\":\"%d\"}", uint8)
	}

	return txBody
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
	view state.View,
	programs *programs.Programs,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) ([]flow.Address, error) {
	return CreateAccountsWithSimpleAddresses(vm, view, programs, privateKeys, chain)
}

func CreateAccountsWithSimpleAddresses(
	vm *fvm.VirtualMachine,
	view state.View,
	programs *programs.Programs,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) ([]flow.Address, error) {
	ctx := fvm.NewContext(
		zerolog.Nop(),
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvoker(zerolog.Nop()),
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
		err := vm.Run(ctx, tx, view, programs)
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
		}
		if addr == flow.EmptyAddress {
			return nil, errors.New("no account creation event emitted")
		}
		accounts = append(accounts, addr)
	}

	return accounts, nil
}

func RootBootstrappedLedger(vm *fvm.VirtualMachine, ctx fvm.Context) state.View {
	view := fvmUtils.NewSimpleView()
	programs := programs.NewEmptyPrograms()

	// set 0 clusters to pass n_collectors >= n_clusters check
	epochConfig := epochs.DefaultEpochConfig()
	epochConfig.NumCollectorClusters = 0
	bootstrap := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithEpochConfig(epochConfig),
	)

	_ = vm.Run(
		ctx,
		bootstrap,
		view,
		programs,
	)

	return view
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

// CreateAddAnAccountKeyMultipleTimesTransaction generates a tx that adds a key several times to an account.
// this can be used to exhaust an account's storage.
func CreateAddAnAccountKeyMultipleTimesTransaction(t *testing.T, accountKey *flow.AccountPrivateKey, counts int) *flow.TransactionBody {
	keyBytes, err := flow.EncodeRuntimeAccountPublicKey(accountKey.PublicKey(1000))
	require.NoError(t, err)

	script := []byte(`
        transaction(counts: Int, key: [UInt8]) {
          prepare(signer: AuthAccount) {
			var i = 0
			while i < counts {
				i = i + 1
				signer.addPublicKey(key)
			}
          }
        }
   	`)

	arg1, err := jsoncdc.Encode(cadence.NewInt(counts))
	require.NoError(t, err)

	arg2, err := jsoncdc.Encode(bytesToCadenceArray(keyBytes))
	require.NoError(t, err)

	addKeysTx := &flow.TransactionBody{
		Script: []byte(script),
	}
	addKeysTx = addKeysTx.AddArgument(arg1).AddArgument(arg2)
	return addKeysTx
}

// CreateAddAccountKeyTransaction generates a tx that adds a key to an account.
func CreateAddAccountKeyTransaction(t *testing.T, accountKey *flow.AccountPrivateKey) *flow.TransactionBody {
	keyBytes, err := flow.EncodeRuntimeAccountPublicKey(accountKey.PublicKey(1000))
	require.NoError(t, err)

	script := []byte(`
        transaction(key: [UInt8]) {
          prepare(signer: AuthAccount) {
            signer.addPublicKey(key)
          }
        }
   	`)

	arg, err := jsoncdc.Encode(bytesToCadenceArray(keyBytes))
	require.NoError(t, err)

	addKeysTx := &flow.TransactionBody{
		Script: []byte(script),
	}
	addKeysTx = addKeysTx.AddArgument(arg)

	return addKeysTx
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values)
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

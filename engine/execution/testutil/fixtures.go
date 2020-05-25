package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

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
			AddAuthorizer(flow.RootAddress)

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

func SignTransactionByRoot(tx *flow.TransactionBody, seqNum uint64) error {
	tx.SetProposalKey(flow.RootAddress, 0, seqNum)
	tx.SetPayer(flow.RootAddress)
	return SignEnvelope(tx, flow.RootAddress, unittest.RootAccountPrivateKey)
}

func RootBootstrappedLedger() virtualmachine.Ledger {
	ledger := make(virtualmachine.MapLedger)
	bootstrap.BootstrapView(ledger, unittest.RootAccountPublicKey, unittest.InitialTokenSupply)
	return ledger
}

func CreateRootAccountInLedger(ledger virtualmachine.Ledger) error {
	l := virtualmachine.NewLedgerDAL(ledger)

	accountKey := unittest.RootAccountPrivateKey.PublicKey(virtualmachine.AccountKeyWeightThreshold)

	return l.CreateAccountWithAddress(
		flow.RootAddress,
		[]flow.AccountPublicKey{accountKey},
	)
}

func bytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewInt(int(b))
	}

	return cadence.NewArray(values)
}

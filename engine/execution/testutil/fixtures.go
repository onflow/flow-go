package testutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/state/protocol"
	protocolMock "github.com/onflow/flow-go/state/protocol/mock"
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

func UpdateContractUnathorizedDeploymentTransaction(contractName string, contract string, authorizer flow.Address) *flow.TransactionBody {
	encoded := hex.EncodeToString([]byte(contract))

	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.contracts.update__experimental(name: "%s", code: "%s".decodeHex())
              }
            }`, contractName, encoded)),
		).
		AddAuthorizer(authorizer)
}

func RemoveContractDeploymentTransaction(contractName string, authorizer flow.Address, chain flow.Chain) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount, service: AuthAccount) {
                signer.contracts.remove(name: "%s")
              }
            }`, contractName)),
		).
		AddAuthorizer(authorizer).
		AddAuthorizer(chain.ServiceAddress())
}

func RemoveContractUnathorizedDeploymentTransaction(contractName string, authorizer flow.Address) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`transaction {
              prepare(signer: AuthAccount) {
                signer.contracts.remove(name: "%s")
              }
            }`, contractName)),
		).
		AddAuthorizer(authorizer)
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
	seed := make([]byte, crypto.KeyGenSeedMinLen)
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
	vm fvm.VM,
	snapshotTree snapshot.SnapshotTree,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) (
	snapshot.SnapshotTree,
	[]flow.Address,
	error,
) {
	return CreateAccountsWithSimpleAddresses(
		vm,
		snapshotTree,
		privateKeys,
		chain)
}

func CreateAccountsWithSimpleAddresses(
	vm fvm.VM,
	snapshotTree snapshot.SnapshotTree,
	privateKeys []flow.AccountPrivateKey,
	chain flow.Chain,
) (
	snapshot.SnapshotTree,
	[]flow.Address,
	error,
) {
	ctx := fvm.NewContext(
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	var accounts []flow.Address

	scriptTemplate := `
        transaction(publicKey: [UInt8]) {
            prepare(signer: AuthAccount) {
                let acct = AuthAccount(payer: signer)
                let publicKey2 = PublicKey(
                    publicKey: publicKey,
                    signatureAlgorithm: SignatureAlgorithm.%s
                )
                acct.keys.add(
                    publicKey: publicKey2,
                    hashAlgorithm: HashAlgorithm.%s,
                    weight: %d.0
                )
            }
	    }`

	serviceAddress := chain.ServiceAddress()

	for _, privateKey := range privateKeys {
		accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)
		encPublicKey := accountKey.PublicKey.Encode()
		cadPublicKey := BytesToCadenceArray(encPublicKey)
		encCadPublicKey, _ := jsoncdc.Encode(cadPublicKey)

		script := []byte(
			fmt.Sprintf(
				scriptTemplate,
				accountKey.SignAlgo.String(),
				accountKey.HashAlgo.String(),
				accountKey.Weight,
			),
		)

		txBody := flow.NewTransactionBody().
			SetScript(script).
			AddArgument(encCadPublicKey).
			AddAuthorizer(serviceAddress)

		tx := fvm.Transaction(txBody, 0)
		executionSnapshot, output, err := vm.Run(ctx, tx, snapshotTree)
		if err != nil {
			return snapshotTree, nil, err
		}

		if output.Err != nil {
			return snapshotTree, nil, fmt.Errorf(
				"failed to create account: %w",
				output.Err)
		}

		snapshotTree = snapshotTree.Append(executionSnapshot)

		var addr flow.Address

		for _, event := range output.Events {
			if event.Type == flow.EventAccountCreated {
				data, err := ccf.Decode(nil, event.Payload)
				if err != nil {
					return snapshotTree, nil, errors.New(
						"error decoding events")
				}
				addr = flow.ConvertAddress(
					data.(cadence.Event).Fields[0].(cadence.Address))
				break
			}
		}
		if addr == flow.EmptyAddress {
			return snapshotTree, nil, errors.New(
				"no account creation event emitted")
		}
		accounts = append(accounts, addr)
	}

	return snapshotTree, accounts, nil
}

func RootBootstrappedLedger(
	vm fvm.VM,
	ctx fvm.Context,
	additionalOptions ...fvm.BootstrapProcedureOption,
) snapshot.SnapshotTree {
	// set 0 clusters to pass n_collectors >= n_clusters check
	epochConfig := epochs.DefaultEpochConfig()
	epochConfig.NumCollectorClusters = 0

	options := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		fvm.WithEpochConfig(epochConfig),
	}

	options = append(options, additionalOptions...)

	bootstrap := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		options...,
	)

	executionSnapshot, _, err := vm.Run(ctx, bootstrap, nil)
	if err != nil {
		panic(err)
	}
	return snapshot.NewSnapshotTree(nil).Append(executionSnapshot)
}

func BytesToCadenceArray(l []byte) cadence.Array {
	values := make([]cadence.Value, len(l))
	for i, b := range l {
		values[i] = cadence.NewUInt8(b)
	}

	return cadence.NewArray(values).WithType(cadence.NewVariableSizedArrayType(cadence.NewUInt8Type()))
}

// CreateAccountCreationTransaction creates a transaction which will create a new account.
//
// This function returns a randomly generated private key and the transaction.
func CreateAccountCreationTransaction(t testing.TB, chain flow.Chain) (flow.AccountPrivateKey, *flow.TransactionBody) {
	accountKey, err := GenerateAccountPrivateKey()
	require.NoError(t, err)
	encPublicKey := accountKey.PublicKey(1000).PublicKey.Encode()
	cadPublicKey := BytesToCadenceArray(encPublicKey)
	encCadPublicKey, err := jsoncdc.Encode(cadPublicKey)
	require.NoError(t, err)

	// define the cadence script
	script := fmt.Sprintf(`
        transaction(publicKey: [UInt8]) {
            prepare(signer: AuthAccount) {
				let acct = AuthAccount(payer: signer)
                let publicKey2 = PublicKey(
                    publicKey: publicKey,
                    signatureAlgorithm: SignatureAlgorithm.%s
                )
                acct.keys.add(
                    publicKey: publicKey2,
                    hashAlgorithm: HashAlgorithm.%s,
                    weight: 1000.0
                )
            }
	    }`,
		accountKey.SignAlgo.String(),
		accountKey.HashAlgo.String(),
	)

	// create the transaction to create the account
	tx := flow.NewTransactionBody().
		SetScript([]byte(script)).
		AddArgument(encCadPublicKey).
		AddAuthorizer(chain.ServiceAddress())

	return accountKey, tx
}

// CreateMultiAccountCreationTransaction creates a transaction which will create many (n) new account.
//
// This function returns a randomly generated private key and the transaction.
func CreateMultiAccountCreationTransaction(t *testing.T, chain flow.Chain, n int) (flow.AccountPrivateKey, *flow.TransactionBody) {
	accountKey, err := GenerateAccountPrivateKey()
	require.NoError(t, err)
	encPublicKey := accountKey.PublicKey(1000).PublicKey.Encode()
	cadPublicKey := BytesToCadenceArray(encPublicKey)
	encCadPublicKey, err := jsoncdc.Encode(cadPublicKey)
	require.NoError(t, err)

	// define the cadence script
	script := fmt.Sprintf(`
        transaction(publicKey: [UInt8]) {
            prepare(signer: AuthAccount) {
                var i = 0
                while i < %d {
                    let account = AuthAccount(payer: signer)
                    let publicKey2 = PublicKey(
                        publicKey: publicKey,
                        signatureAlgorithm: SignatureAlgorithm.%s
                    )
                    account.keys.add(
                        publicKey: publicKey2,
                        hashAlgorithm: HashAlgorithm.%s,
                        weight: 1000.0
                    )
                    i = i + 1
                }
            }
	    }`,
		n,
		accountKey.SignAlgo.String(),
		accountKey.HashAlgo.String(),
	)

	// create the transaction to create the account
	tx := flow.NewTransactionBody().
		SetScript([]byte(script)).
		AddArgument(encCadPublicKey).
		AddAuthorizer(chain.ServiceAddress())

	return accountKey, tx
}

// CreateAddAnAccountKeyMultipleTimesTransaction generates a tx that adds a key several times to an account.
// this can be used to exhaust an account's storage.
func CreateAddAnAccountKeyMultipleTimesTransaction(t *testing.T, accountKey *flow.AccountPrivateKey, counts int) *flow.TransactionBody {
	script := []byte(fmt.Sprintf(`
      transaction(counts: Int, key: [UInt8]) {
        prepare(signer: AuthAccount) {
          var i = 0
          while i < counts {
            i = i + 1
            let publicKey2 = PublicKey(
              publicKey: key,
              signatureAlgorithm: SignatureAlgorithm.%s
            )
            signer.keys.add(
              publicKey: publicKey2,
              hashAlgorithm: HashAlgorithm.%s,
              weight: 1000.0
            )
	      }
        }
      }
   	`, accountKey.SignAlgo.String(), accountKey.HashAlgo.String()))

	arg1, err := jsoncdc.Encode(cadence.NewInt(counts))
	require.NoError(t, err)

	encPublicKey := accountKey.PublicKey(1000).PublicKey.Encode()
	cadPublicKey := BytesToCadenceArray(encPublicKey)
	arg2, err := jsoncdc.Encode(cadPublicKey)
	require.NoError(t, err)

	addKeysTx := &flow.TransactionBody{
		Script: script,
	}
	addKeysTx = addKeysTx.AddArgument(arg1).AddArgument(arg2)
	return addKeysTx
}

// CreateAddAccountKeyTransaction generates a tx that adds a key to an account.
func CreateAddAccountKeyTransaction(t *testing.T, accountKey *flow.AccountPrivateKey) *flow.TransactionBody {
	keyBytes := accountKey.PublicKey(1000).PublicKey.Encode()

	script := []byte(`
        transaction(key: [UInt8]) {
          prepare(signer: AuthAccount) {
            let acct = AuthAccount(payer: signer)
            let publicKey2 = PublicKey(
              publicKey: key,
              signatureAlgorithm: SignatureAlgorithm.%s
            )
            signer.keys.add(
              publicKey: publicKey2,
              hashAlgorithm: HashAlgorithm.%s,
              weight: %d.0
            )
          }
        }
   	`)

	arg, err := jsoncdc.Encode(bytesToCadenceArray(keyBytes))
	require.NoError(t, err)

	addKeysTx := &flow.TransactionBody{
		Script: script,
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

// TODO(ramtin): when we get rid of BlockExecutionData, this could move to the global unittest fixtures
// TrieUpdates are internal data to the ledger package and should not have leaked into
// packages like uploader in the first place
func ComputationResultFixture(t *testing.T) *execution.ComputationResult {
	startState := unittest.StateCommitmentFixture()
	update1, err := ledger.NewUpdate(
		ledger.State(startState),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(3, []byte{33})}),
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(1, []byte{11})}),
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(2, []byte{1, 1}), ledger.NewKeyPart(3, []byte{2, 5})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
			nil,
			[]byte{3, 3, 3, 3, 3},
		},
	)
	require.NoError(t, err)

	trieUpdate1, err := pathfinder.UpdateToTrieUpdate(update1, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update2, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{},
		[]ledger.Value{},
	)
	require.NoError(t, err)

	trieUpdate2, err := pathfinder.UpdateToTrieUpdate(update2, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update3, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(9, []byte{6})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
		},
	)
	require.NoError(t, err)

	trieUpdate3, err := pathfinder.UpdateToTrieUpdate(update3, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	update4, err := ledger.NewUpdate(
		ledger.State(unittest.StateCommitmentFixture()),
		[]ledger.Key{
			ledger.NewKey([]ledger.KeyPart{ledger.NewKeyPart(9, []byte{6})}),
		},
		[]ledger.Value{
			[]byte{21, 37},
		},
	)
	require.NoError(t, err)

	trieUpdate4, err := pathfinder.UpdateToTrieUpdate(update4, complete.DefaultPathFinderVersion)
	require.NoError(t, err)

	executableBlock := unittest.ExecutableBlockFixture([][]flow.Identifier{
		{unittest.IdentifierFixture()},
		{unittest.IdentifierFixture()},
		{unittest.IdentifierFixture()},
	}, &startState)

	blockExecResult := execution.NewPopulatedBlockExecutionResult(executableBlock)
	blockExecResult.CollectionExecutionResultAt(0).AppendTransactionResults(
		flow.EventsList{
			unittest.EventFixture("what", 0, 0, unittest.IdentifierFixture(), 2),
			unittest.EventFixture("ever", 0, 1, unittest.IdentifierFixture(), 22),
		},
		nil,
		nil,
		flow.TransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			ErrorMessage:    "",
			ComputationUsed: 23,
			MemoryUsed:      101,
		},
	)
	blockExecResult.CollectionExecutionResultAt(1).AppendTransactionResults(
		flow.EventsList{
			unittest.EventFixture("what", 2, 0, unittest.IdentifierFixture(), 2),
			unittest.EventFixture("ever", 2, 1, unittest.IdentifierFixture(), 22),
			unittest.EventFixture("ever", 2, 2, unittest.IdentifierFixture(), 2),
			unittest.EventFixture("ever", 2, 3, unittest.IdentifierFixture(), 22),
		},
		nil,
		nil,
		flow.TransactionResult{
			TransactionID:   unittest.IdentifierFixture(),
			ErrorMessage:    "fail",
			ComputationUsed: 1,
			MemoryUsed:      22,
		},
	)

	return &execution.ComputationResult{
		BlockExecutionResult: blockExecResult,
		BlockAttestationResult: &execution.BlockAttestationResult{
			BlockExecutionData: &execution_data.BlockExecutionData{
				ChunkExecutionDatas: []*execution_data.ChunkExecutionData{
					{TrieUpdate: trieUpdate1},
					{TrieUpdate: trieUpdate2},
					{TrieUpdate: trieUpdate3},
					{TrieUpdate: trieUpdate4},
				},
			},
		},
		ExecutionReceipt: &flow.ExecutionReceipt{
			ExecutionResult: flow.ExecutionResult{
				Chunks: flow.ChunkList{
					{EndState: unittest.StateCommitmentFixture()},
					{EndState: unittest.StateCommitmentFixture()},
					{EndState: unittest.StateCommitmentFixture()},
					{EndState: unittest.StateCommitmentFixture()},
				},
			},
		},
	}
}

// EntropyProviderFixture returns an entropy provider mock that
// supports RandomSource().
// If input is nil, a random source fixture is generated.
func EntropyProviderFixture(source []byte) environment.EntropyProvider {
	if source == nil {
		source = unittest.SignatureFixture()
	}
	provider := envMock.EntropyProvider{}
	provider.On("RandomSource").Return(source, nil)
	return &provider
}

// ProtocolStateWithSourceFixture returns a protocol state mock that only
// supports AtBlockID to return a snapshot mock.
// The snapshot mock only supports RandomSource().
// If input is nil, a random source fixture is generated.
func ProtocolStateWithSourceFixture(source []byte) protocol.State {
	if source == nil {
		source = unittest.SignatureFixture()
	}
	snapshot := &protocolMock.Snapshot{}
	snapshot.On("RandomSource").Return(source, nil)
	state := protocolMock.State{}
	state.On("AtBlockID", mock.Anything).Return(snapshot)
	return &state
}

package execution

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/stdlib"

	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	synctest "github.com/onflow/flow-go/module/state_synchronization/requester/unittest"
	"github.com/onflow/flow-go/storage"
	pebbleStorage "github.com/onflow/flow-go/storage/pebble"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestScripts(t *testing.T) {
	suite.Run(t, new(scriptTestSuite))
}

type scriptTestSuite struct {
	suite.Suite
	scripts       *Scripts
	registerIndex storage.RegisterIndex
	vm            *fvm.VirtualMachine
	vmCtx         fvm.Context
	chain         flow.Chain
	height        uint64
	snapshot      snapshot.SnapshotTree
	dbDir         string
}

func (s *scriptTestSuite) TestScriptExecution() {
	s.Run("Simple Script Execution", func() {
		number := int64(42)
		code := []byte(fmt.Sprintf("access(all) fun main(): Int { return %d; }", number))

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Require().NoError(err)
		val, err := jsoncdc.Decode(nil, result)
		s.Require().NoError(err)
		s.Assert().Equal(number, val.(cadence.Int).Value.Int64())
	})

	s.Run("Get Block", func() {
		code := []byte(fmt.Sprintf(`access(all) fun main(): UInt64 {
			getBlock(at: %d)!
			return getCurrentBlock().height
		}`, s.height))

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Require().NoError(err)
		val, err := jsoncdc.Decode(nil, result)
		s.Require().NoError(err)
		// make sure that the returned block height matches the current one set
		s.Assert().Equal(s.height, uint64(val.(cadence.UInt64)))
	})

	s.Run("Handle not found Register", func() {
		// use a non-existing address to trigger register get function
		code := []byte("import Foo from 0x01; access(all) fun main() { }")

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height)
		s.Assert().Error(err)
		s.Assert().Nil(result)
	})

	s.Run("Valid Argument", func() {
		code := []byte("access(all) fun main(foo: Int): Int { return foo }")
		arg := cadence.NewInt(2)
		encoded, err := jsoncdc.Encode(arg)
		s.Require().NoError(err)

		result, err := s.scripts.ExecuteAtBlockHeight(
			context.Background(),
			code,
			[][]byte{encoded},
			s.height,
		)
		s.Require().NoError(err)
		s.Assert().Equal(encoded, result)
	})

	s.Run("Invalid Argument", func() {
		code := []byte("access(all) fun main(foo: Int): Int { return foo }")
		invalid := [][]byte{[]byte("i")}

		result, err := s.scripts.ExecuteAtBlockHeight(context.Background(), code, invalid, s.height)
		s.Assert().Nil(result)
		var coded errors.CodedError
		s.Require().True(errors.As(err, &coded))
		fmt.Println(coded.Code(), coded.Error())
		s.Assert().Equal(errors.ErrCodeInvalidArgumentError, coded.Code())
	})
}

func (s *scriptTestSuite) TestGetAccount() {
	s.Run("Get Service Account", func() {
		address := s.chain.ServiceAddress()
		account, err := s.scripts.GetAccountAtBlockHeight(context.Background(), address, s.height)
		s.Require().NoError(err)
		s.Assert().Equal(address, account.Address)
		s.Assert().NotZero(account.Balance)
		s.Assert().NotZero(len(account.Contracts))
	})

	s.Run("Get New Account", func() {
		address := s.createAccount()
		account, err := s.scripts.GetAccountAtBlockHeight(context.Background(), address, s.height)
		s.Require().NoError(err)
		s.Require().Equal(address, account.Address)
		s.Assert().Zero(account.Balance)
	})
}

func (s *scriptTestSuite) TestGetAccountBalance() {
	address := s.createAccount()
	var transferAmount uint64 = 100000000
	s.transferTokens(address, transferAmount)
	balance, err := s.scripts.GetAccountBalance(context.Background(), address, s.height)
	s.Require().NoError(err)
	s.Require().Equal(transferAmount, balance)
}

func (s *scriptTestSuite) TestGetAccountKeys() {
	address := s.createAccount()
	publicKey := s.addAccountKey(address, accountKeyAPIVersionV2)

	accountKeys, err := s.scripts.GetAccountKeys(context.Background(), address, s.height)
	s.Require().NoError(err)
	s.Assert().Equal(1, len(accountKeys))
	s.Assert().Equal(publicKey.PublicKey, accountKeys[0].PublicKey)
	s.Assert().Equal(publicKey.SignAlgo, accountKeys[0].SignAlgo)
	s.Assert().Equal(publicKey.HashAlgo, accountKeys[0].HashAlgo)
	s.Assert().Equal(publicKey.Weight, accountKeys[0].Weight)

}

func (s *scriptTestSuite) SetupTest() {
	lockManager := storage.NewTestingLockManager()
	logger := unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	entropyProvider := testutil.ProtocolStateWithSourceFixture(nil)
	blockchain := unittest.BlockchainFixture(10)
	headers := newBlockHeadersStorage(blockchain)

	s.chain = flow.Emulator.Chain()
	s.snapshot = snapshot.NewSnapshotTree(nil)
	s.vm = fvm.NewVirtualMachine()
	s.vmCtx = fvm.NewContext(
		s.chain,
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)
	s.height = blockchain[0].Height

	s.dbDir = unittest.TempDir(s.T())
	db := pebbleStorage.NewBootstrappedRegistersWithPathForTest(s.T(), s.dbDir, s.height, s.height)
	pebbleRegisters, err := pebbleStorage.NewRegisters(db, pebbleStorage.PruningDisabled)
	s.Require().NoError(err)
	s.registerIndex = pebbleRegisters

	derivedChainData, err := derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	s.Require().NoError(err)

	index := indexer.New(
		logger,
		metrics.NewNoopCollector(),
		nil,
		s.registerIndex,
		headers,
		nil,
		nil,
		nil,
		nil,
		nil,
		flow.Testnet,
		derivedChainData,
		nil,
		nil,
		lockManager,
		nil, // accountTxIndex
	)

	s.scripts = NewScripts(
		logger,
		metrics.NewNoopCollector(),
		s.chain.ChainID(),
		entropyProvider,
		headers,
		index.RegisterValue,
		query.NewDefaultConfig(),
		derivedChainData,
		true,
	)

	s.bootstrap()
}

func (s *scriptTestSuite) TearDownTest() {
	s.Require().NoError(os.RemoveAll(s.dbDir))
}

func (s *scriptTestSuite) bootstrap() {
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, out, err := s.vm.Run(
		s.vmCtx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		s.snapshot)

	s.Require().NoError(err)
	s.Require().NoError(out.Err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)
}

func (s *scriptTestSuite) createAccount() flow.Address {
	const createAccountTransaction = `
		transaction {
		  prepare(signer: auth(Storage, Capabilities) &Account) {
			let account = Account(payer: signer)
		  }
		}`

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(createAccountTransaction)).
		SetPayer(unittest.RandomAddressFixture()).
		AddAuthorizer(s.chain.ServiceAddress()).
		Build()
	s.Require().NoError(err)

	executionSnapshot, output, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(txBody, 0),
		s.snapshot,
	)
	s.Require().NoError(err)
	s.Require().NoError(output.Err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)

	var accountCreatedEvents []flow.Event
	for _, event := range output.Events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	s.Require().Len(accountCreatedEvents, 1)

	data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
	s.Require().NoError(err)

	return flow.Address(
		cadence.SearchFieldByName(
			data.(cadence.Event),
			stdlib.AccountEventAddressParameter.Identifier,
		).(cadence.Address),
	)
}

func (s *scriptTestSuite) transferTokens(accountAddress flow.Address, amount uint64) {
	transferTx, err := transferTokensTx(s.chain).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(amount))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(accountAddress))).
		AddAuthorizer(s.chain.ServiceAddress()).
		SetPayer(accountAddress).
		Build()
	s.Require().NoError(err)

	executionSnapshot, _, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(transferTx, 0),
		s.snapshot,
	)
	s.Require().NoError(err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)
}

type accountKeyAPIVersion string

const (
	accountKeyAPIVersionV1 accountKeyAPIVersion = "V1"
	accountKeyAPIVersionV2 accountKeyAPIVersion = "V2"
)

func (s *scriptTestSuite) addAccountKey(
	accountAddress flow.Address,
	apiVersion accountKeyAPIVersion,
) flow.AccountPublicKey {
	const addAccountKeyTransaction = `
transaction(key: [UInt8]) {
  prepare(signer: auth(AddKey) &Account) {
	let publicKey = PublicKey(
		publicKey: key,
		signatureAlgorithm: SignatureAlgorithm.ECDSA_P256
	 )
    signer.keys.add(
		publicKey: publicKey,
		hashAlgorithm: HashAlgorithm.SHA3_256,
		weight: 1000.0
	)
  }
}
`
	privateKey, err := unittest.AccountKeyDefaultFixture()
	s.Require().NoError(err)

	publicKey, encodedCadencePublicKey := newAccountKey(s.T(), privateKey, apiVersion)

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(addAccountKeyTransaction)).
		SetPayer(accountAddress).
		AddArgument(encodedCadencePublicKey).
		AddAuthorizer(accountAddress).
		Build()
	s.Require().NoError(err)

	executionSnapshot, _, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(txBody, 0),
		s.snapshot,
	)
	s.Require().NoError(err)

	s.height++
	err = s.registerIndex.Store(executionSnapshot.UpdatedRegisters(), s.height)
	s.Require().NoError(err)

	s.snapshot = s.snapshot.Append(executionSnapshot)

	return publicKey
}

func newAccountKey(
	tb testing.TB,
	privateKey *flow.AccountPrivateKey,
	apiVersion accountKeyAPIVersion,
) (
	publicKey flow.AccountPublicKey,
	encodedCadencePublicKey []byte,
) {
	publicKey = privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	var publicKeyBytes []byte
	if apiVersion == accountKeyAPIVersionV1 {
		var err error
		publicKeyBytes, err = flow.EncodeRuntimeAccountPublicKey(publicKey)
		require.NoError(tb, err)
	} else {
		publicKeyBytes = publicKey.PublicKey.Encode()
	}

	cadencePublicKey := testutil.BytesToCadenceArray(publicKeyBytes)
	encodedCadencePublicKey, err := jsoncdc.Encode(cadencePublicKey)
	require.NoError(tb, err)

	return publicKey, encodedCadencePublicKey
}

func newBlockHeadersStorage(blocks []*flow.Block) storage.Headers {
	blocksByHeight := make(map[uint64]*flow.Block)
	for _, b := range blocks {
		blocksByHeight[b.Height] = b
	}

	return synctest.MockBlockHeaderStorage(synctest.WithByHeight(blocksByHeight))
}

func transferTokensTx(chain flow.Chain) *flow.TransactionBodyBuilder {
	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	return flow.NewTransactionBodyBuilder().
		SetScript([]byte(fmt.Sprintf(
			`
	// This transaction is a template for a transaction that
	// could be used by anyone to send tokens to another account
	// that has been set up to receive tokens.
	//
	// The withdraw amount and the account from getAccount
	// would be the parameters to the transaction

	import FungibleToken from 0x%s
	import FlowToken from 0x%s

	transaction(amount: UFix64, to: Address) {

	// The Vault resource that holds the tokens that are being transferred
	let sentVault: @{FungibleToken.Vault}

	prepare(signer: auth(BorrowValue) &Account) {

	// Get a reference to the signer's stored vault
	let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdraw) &FlowToken.Vault>(from: /storage/flowTokenVault)
	?? panic("Could not borrow reference to the owner's Vault!")

	// Withdraw tokens from the signer's stored vault
	self.sentVault <- vaultRef.withdraw(amount: amount)
	}

	execute {

	// Get the recipient's public account object
	let recipient = getAccount(to)

	// Get a reference to the recipient's Receiver
	let receiverRef = recipient.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
	?? panic("Could not borrow receiver reference to the recipient's Vault")

	// Deposit the withdrawn tokens in the recipient's receiver
	receiverRef.deposit(from: <-self.sentVault)
	}
	}`,
			sc.FungibleToken.Address.Hex(),
			sc.FlowToken.Address.Hex(),
		)),
		)
}

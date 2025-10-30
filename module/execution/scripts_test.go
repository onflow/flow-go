package execution

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/stdlib"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/engine/execution/testutil"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type accountKeyAPIVersion string

const (
	accountKeyAPIVersionV1 accountKeyAPIVersion = "V1"
	accountKeyAPIVersionV2 accountKeyAPIVersion = "V2"
)

func TestScripts(t *testing.T) {
	suite.Run(t, new(scriptTestSuite))
}

type scriptTestSuite struct {
	suite.Suite
	logger           zerolog.Logger
	headers          *storagemock.Headers
	registerSnapshot *storagemock.RegisterSnapshotReader
	vm               *fvm.VirtualMachine
	vmCtx            fvm.Context
	chain            flow.Chain
	height           uint64
	snapshot         snapshot.SnapshotTree
	entropyProvider  protocol.SnapshotExecutionSubsetProvider
	derivedChainData *derived.DerivedChainData
}

func (s *scriptTestSuite) SetupTest() {
	s.logger = unittest.LoggerForTest(s.Suite.T(), zerolog.InfoLevel)
	s.entropyProvider = testutil.ProtocolStateWithSourceFixture(nil)
	s.headers = storagemock.NewHeaders(s.T())
	s.registerSnapshot = storagemock.NewRegisterSnapshotReader(s.T())
	s.chain = flow.Emulator.Chain()
	s.snapshot = snapshot.NewSnapshotTree(nil)
	s.vm = fvm.NewVirtualMachine()
	s.vmCtx = fvm.NewContext(
		fvm.WithChain(s.chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	)

	blockCount := 10
	blockMap := make(map[uint64]*flow.Block, blockCount)
	rootBlock := unittest.Block.Genesis(flow.Emulator)
	blockMap[rootBlock.Height] = rootBlock
	s.height = rootBlock.Height

	parent := rootBlock.ToHeader()
	for i := 0; i < blockCount; i++ {
		block := unittest.BlockWithParentFixture(parent)
		// update for next iteration
		parent = block.ToHeader()
		blockMap[block.Height] = block
	}
	s.headers.On("ByHeight", mock.AnythingOfType("uint64")).Return(
		mocks.ConvertStorageOutput(
			mocks.StorageMapGetter(blockMap),
			func(block *flow.Block) *flow.Header { return block.ToHeader() },
		),
	).Maybe()
	var err error
	s.derivedChainData, err = derived.NewDerivedChainData(derived.DefaultDerivedDataCacheSize)
	require.NoError(s.T(), err)
}

// TestScriptExecution tests the ExecuteAtBlockHeight function.
// Test cases:
// 1. Executes a script that gets the current block height.
// 2. Errors when height is not indexed.
func (s *scriptTestSuite) TestScriptExecution() {
	scripts := s.defaultScripts()
	code := []byte(fmt.Sprintf(`access(all) fun main(): UInt64 {
			getBlock(at: %d)!
			return getCurrentBlock().height
		}`, s.height))

	s.Run("get block script execution", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(s.snapshot, nil).Once()

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height, s.registerSnapshot)
		require.NoError(s.T(), err)
		val, err := jsoncdc.Decode(nil, result)
		require.NoError(s.T(), err)
		// make sure that the returned block height matches the current one set
		require.Equal(s.T(), s.height, uint64(val.(cadence.UInt64)))
	})

	s.Run("error when height is not indexed  ", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(nil, storage.ErrHeightNotIndexed).Once()

		result, err := scripts.ExecuteAtBlockHeight(context.Background(), code, nil, s.height, s.registerSnapshot)
		require.Error(s.T(), err)
		require.Nil(s.T(), result)
		require.ErrorIs(s.T(), err, storage.ErrHeightNotIndexed)
	})
}

// TestGetAccount verifies retrieval of account information at a specific block height.
// Test cases:
// 1. Retrieves a newly created account and checks its address and balance.
// 2. Returns an error when the block height is not indexed.
func (s *scriptTestSuite) TestGetAccount() {
	scripts := s.defaultScripts()
	address := s.createAccount()

	s.Run("Get New Account", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(s.snapshot, nil).Once()

		account, err := scripts.GetAccountAtBlockHeight(context.Background(), address, s.height, s.registerSnapshot)
		require.NoError(s.T(), err)
		require.Equal(s.T(), address, account.Address)
		require.Zero(s.T(), account.Balance)
	})

	s.Run("error when height is not indexed  ", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(nil, storage.ErrHeightNotIndexed).Once()

		account, err := scripts.GetAccountAtBlockHeight(context.Background(), address, s.height, s.registerSnapshot)
		require.Error(s.T(), err)
		require.Nil(s.T(), account)
		require.ErrorIs(s.T(), err, storage.ErrHeightNotIndexed)
	})
}

// TestGetAccountBalance verifies that GetAccountBalance returns the correct balance for an account.
// Test cases:
// 1. Transfers tokens to an account and verifies the correct balance is returned.
// 2. Returns an error when the block height is not indexed.
func (s *scriptTestSuite) TestGetAccountBalance() {
	scripts := s.defaultScripts()
	address := s.createAccount()
	var transferAmount uint64 = 100000000
	s.transferTokens(address, transferAmount)

	s.Run("get account balance", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(s.snapshot, nil).Once()

		balance, err := scripts.GetAccountBalance(context.Background(), address, s.height, s.registerSnapshot)
		require.NoError(s.T(), err)
		require.Equal(s.T(), transferAmount, balance)
	})

	s.Run("error when height is not indexed  ", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(nil, storage.ErrHeightNotIndexed).Once()

		balance, err := scripts.GetAccountBalance(context.Background(), address, s.height, s.registerSnapshot)
		require.Error(s.T(), err)
		require.Equal(s.T(), uint64(0), balance)
		require.ErrorIs(s.T(), err, storage.ErrHeightNotIndexed)
	})
}

// TestGetAccountKeys verifies that GetAccountKeys returns all public keys for an account.
// Test cases:
// 1. Adds a key to an account and verifies its properties.
// 2. Returns an error when the block height is not indexed.
func (s *scriptTestSuite) TestGetAccountKeys() {
	scripts := s.defaultScripts()
	address := s.createAccount()
	publicKey := s.addAccountKey(address, accountKeyAPIVersionV2)

	s.Run("get account balance", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(s.snapshot, nil).Once()

		accountKeys, err := scripts.GetAccountKeys(context.Background(), address, s.height, s.registerSnapshot)
		require.NoError(s.T(), err)
		require.Equal(s.T(), 1, len(accountKeys))
		require.Equal(s.T(), publicKey.PublicKey, accountKeys[0].PublicKey)
		require.Equal(s.T(), publicKey.SignAlgo, accountKeys[0].SignAlgo)
		require.Equal(s.T(), publicKey.HashAlgo, accountKeys[0].HashAlgo)
		require.Equal(s.T(), publicKey.Weight, accountKeys[0].Weight)
	})

	s.Run("error when height is not indexed  ", func() {
		s.registerSnapshot.On("StorageSnapshot", s.height).Return(nil, storage.ErrHeightNotIndexed).Once()

		accountKeys, err := scripts.GetAccountKeys(context.Background(), address, s.height, s.registerSnapshot)
		require.Error(s.T(), err)
		require.Nil(s.T(), accountKeys)
		require.ErrorIs(s.T(), err, storage.ErrHeightNotIndexed)
	})
}

// defaultScripts returns a pre-configured Scripts instance with default parameters for testing.
func (s *scriptTestSuite) defaultScripts() *Scripts {
	scripts := NewScripts(
		s.logger,
		metrics.NewNoopCollector(),
		s.chain.ChainID(),
		s.entropyProvider,
		s.headers,
		query.NewDefaultConfig(),
		s.derivedChainData,
		true,
		0,
		math.MaxUint64,
		nil,
	)

	s.bootstrap()

	return scripts
}

func (s *scriptTestSuite) bootstrap() {
	bootstrapOpts := []fvm.BootstrapProcedureOption{
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	executionSnapshot, out, err := s.vm.Run(
		s.vmCtx,
		fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOpts...),
		s.snapshot)

	require.NoError(s.T(), err)
	require.NoError(s.T(), out.Err)
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
	require.NoError(s.T(), err)

	executionSnapshot, output, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(txBody, 0),
		s.snapshot,
	)
	require.NoError(s.T(), err)
	require.NoError(s.T(), output.Err)

	s.snapshot = s.snapshot.Append(executionSnapshot)

	var accountCreatedEvents []flow.Event
	for _, event := range output.Events {
		if event.Type != flow.EventAccountCreated {
			continue
		}
		accountCreatedEvents = append(accountCreatedEvents, event)
		break
	}
	require.Len(s.T(), accountCreatedEvents, 1)

	data, err := ccf.Decode(nil, accountCreatedEvents[0].Payload)
	require.NoError(s.T(), err)

	return flow.ConvertAddress(
		cadence.SearchFieldByName(
			data.(cadence.Event),
			stdlib.AccountEventAddressParameter.Identifier,
		).(cadence.Address))
}

func (s *scriptTestSuite) transferTokens(accountAddress flow.Address, amount uint64) {
	transferTx, err := transferTokensTx(s.chain).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(amount))).
		AddArgument(jsoncdc.MustEncode(cadence.Address(accountAddress))).
		AddAuthorizer(s.chain.ServiceAddress()).
		SetPayer(accountAddress).
		Build()
	require.NoError(s.T(), err)

	executionSnapshot, _, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(transferTx, 0),
		s.snapshot,
	)
	require.NoError(s.T(), err)

	s.snapshot = s.snapshot.Append(executionSnapshot)
}

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
	require.NoError(s.T(), err)

	publicKey, encodedCadencePublicKey := newAccountKey(s.T(), privateKey, apiVersion)

	txBody, err := flow.NewTransactionBodyBuilder().
		SetScript([]byte(addAccountKeyTransaction)).
		SetPayer(accountAddress).
		AddArgument(encodedCadencePublicKey).
		AddAuthorizer(accountAddress).
		Build()
	require.NoError(s.T(), err)

	executionSnapshot, _, err := s.vm.Run(
		s.vmCtx,
		fvm.Transaction(txBody, 0),
		s.snapshot,
	)
	require.NoError(s.T(), err)

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

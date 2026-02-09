package extended

import (
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// ===== Test Setup Helpers =====

func newAccountTxIndexerForTest(
	t *testing.T,
	chainID flow.ChainID,
	latestHeight uint64,
) (*AccountTransactions, *indexes.AccountTransactions, storage.LockManager, storage.DB) {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

	lm := storage.NewTestingLockManager()
	store, err := indexes.NewAccountTransactions(db, latestHeight)
	require.NoError(t, err)

	return NewAccountTransactions(zerolog.Nop(), store, chainID, lm), store, lm, db
}

// createTestEvent creates a CCF-encoded flow event with arbitrary cadence fields.
func createTestEvent(
	t *testing.T,
	txIndex uint32,
	eventTypeName string,
	fields []cadence.Field,
	values []cadence.Value,
) flow.Event {
	location := common.NewAddressLocation(nil, common.Address{}, "Test")
	eventCadenceType := cadence.NewEventType(location, eventTypeName, fields, nil)
	event := cadence.NewEvent(values).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             flow.EventType("A..Test." + eventTypeName),
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// indexBlock runs IndexBlockData with proper locking and batch commit.
func indexBlock(
	t *testing.T,
	indexer *AccountTransactions,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) {
	err := unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
	require.NoError(t, err)
}

// indexBlockExpectError runs IndexBlockData and returns the error.
func indexBlockExpectError(
	t *testing.T,
	indexer *AccountTransactions,
	lm storage.LockManager,
	db storage.DB,
	data BlockData,
) error {
	return unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
			return indexer.IndexBlockData(lctx, data, rw)
		})
	})
}

// assertAccountTx verifies that a specific transaction is indexed for the given address
// with the expected authorizer status.
func assertAccountTx(
	t *testing.T,
	store *indexes.AccountTransactions,
	height uint64,
	addr flow.Address,
	txID flow.Identifier,
	expectedIsAuth bool,
) {
	t.Helper()
	results, err := store.TransactionsByAddress(addr, height, height)
	require.NoError(t, err)

	for _, r := range results {
		if r.TransactionID == txID {
			assert.Equal(t, expectedIsAuth, r.IsAuthorizer,
				"address %s tx %s: expected isAuthorizer=%v", addr, txID, expectedIsAuth)
			return
		}
	}
	t.Errorf("transaction %s not found for address %s at height %d", txID, addr, height)
}

// assertTransactionCount verifies the total number of transactions for an address at a height.
func assertTransactionCount(
	t *testing.T,
	store *indexes.AccountTransactions,
	height uint64,
	addr flow.Address,
	expectedCount int,
) {
	t.Helper()
	results, err := store.TransactionsByAddress(addr, height, height)
	require.NoError(t, err)
	require.Len(t, results, expectedCount,
		"expected %d transactions for address %s at height %d", expectedCount, addr, height)
}

// assertNoTransactions verifies that no transactions are indexed for the given address.
func assertNoTransactions(
	t *testing.T,
	store *indexes.AccountTransactions,
	height uint64,
	addr flow.Address,
) {
	t.Helper()
	assertTransactionCount(t, store, height, addr, 0)
}

// ===== Basic Indexing Tests =====

func TestAccountTransactionsIndexer(t *testing.T) {
	const testHeight = uint64(100)

	t.Run("empty block", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: nil,
			Events:       map[uint32][]flow.Event{},
		})

		// Verify height was updated
		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, testHeight, latest)

		// No transactions for any address
		assertNoTransactions(t, store, testHeight, unittest.RandomAddressFixture())
	})

	t.Run("single transaction with distinct accounts", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		proposer := unittest.RandomAddressFixture()
		auth1 := unittest.RandomAddressFixture()
		auth2 := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: proposer}
				tb.Authorizers = []flow.Address{auth1, auth2}
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, false)
		assertAccountTx(t, store, testHeight, proposer, txID, false)
		assertAccountTx(t, store, testHeight, auth1, txID, true)
		assertAccountTx(t, store, testHeight, auth2, txID, true)
	})

	t.Run("payer is also authorizer", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		proposer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: proposer}
				tb.Authorizers = []flow.Address{payer}
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, proposer, txID, false)
		// payer should be deduplicated to one entry
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	t.Run("payer is all roles", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: payer}
				tb.Authorizers = []flow.Address{payer}
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	t.Run("multiple transactions", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		account1 := unittest.RandomAddressFixture()
		account2 := unittest.RandomAddressFixture()
		account3 := unittest.RandomAddressFixture()

		tx1 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account1
				tb.ProposalKey = flow.ProposalKey{Address: account1}
				tb.Authorizers = []flow.Address{account1}
			},
		)

		tx2 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account2
				tb.ProposalKey = flow.ProposalKey{Address: account2}
				tb.Authorizers = []flow.Address{account2}
			},
		)

		tx3 := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = account3
				tb.ProposalKey = flow.ProposalKey{Address: account3}
				tb.Authorizers = []flow.Address{account3}
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx1, &tx2, &tx3},
			Events:       map[uint32][]flow.Event{},
		})

		assertAccountTx(t, store, testHeight, account1, tx1.ID(), true)
		assertAccountTx(t, store, testHeight, account2, tx2.ID(), true)
		assertAccountTx(t, store, testHeight, account3, tx3.ID(), true)

		// Each account should only have 1 transaction
		assertTransactionCount(t, store, testHeight, account1, 1)
		assertTransactionCount(t, store, testHeight, account2, 1)
		assertTransactionCount(t, store, testHeight, account3, 1)
	})
}

// ===== CCF Event Creation Helpers =====

// createFTDepositedEvent creates a CCF-encoded FungibleToken.Deposited event
func createFTDepositedEvent(t *testing.T, eventType flow.EventType, txIndex uint32, toAddr *flow.Address) flow.Event {
	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"FungibleToken.Deposited",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "amount", Type: cadence.UFix64Type},
			{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
			{Identifier: "toUUID", Type: cadence.UInt64Type},
			{Identifier: "depositedUUID", Type: cadence.UInt64Type},
			{Identifier: "balanceAfter", Type: cadence.UFix64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.FlowToken.Vault"),
		cadence.UFix64(100_00000000), // 100.0
		toValue,
		cadence.UInt64(1),
		cadence.UInt64(2),
		cadence.UFix64(200_00000000),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createFTWithdrawnEvent creates a CCF-encoded FungibleToken.Withdrawn event
func createFTWithdrawnEvent(t *testing.T, eventType flow.EventType, txIndex uint32, fromAddr *flow.Address) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"FungibleToken.Withdrawn",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "amount", Type: cadence.UFix64Type},
			{Identifier: "from", Type: cadence.NewOptionalType(cadence.AddressType)},
			{Identifier: "fromUUID", Type: cadence.UInt64Type},
			{Identifier: "withdrawnUUID", Type: cadence.UInt64Type},
			{Identifier: "balanceAfter", Type: cadence.UFix64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.FlowToken.Vault"),
		cadence.UFix64(50_00000000),
		fromValue,
		cadence.UInt64(1),
		cadence.UInt64(3),
		cadence.UFix64(150_00000000),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createNFTDepositedEvent creates a CCF-encoded NonFungibleToken.Deposited event
func createNFTDepositedEvent(t *testing.T, eventType flow.EventType, txIndex uint32, toAddr *flow.Address) flow.Event {
	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"NonFungibleToken.Deposited",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "uuid", Type: cadence.UInt64Type},
			{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
			{Identifier: "collectionUUID", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.TopShot.NFT"),
		cadence.UInt64(42),
		cadence.UInt64(100),
		toValue,
		cadence.UInt64(5),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// createNFTWithdrawnEvent creates a CCF-encoded NonFungibleToken.Withdrawn event
func createNFTWithdrawnEvent(t *testing.T, eventType flow.EventType, txIndex uint32, fromAddr *flow.Address) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventCadenceType := cadence.NewEventType(
		location,
		"NonFungibleToken.Withdrawn",
		[]cadence.Field{
			{Identifier: "type", Type: cadence.StringType},
			{Identifier: "id", Type: cadence.UInt64Type},
			{Identifier: "uuid", Type: cadence.UInt64Type},
			{Identifier: "from", Type: cadence.NewOptionalType(cadence.AddressType)},
			{Identifier: "providerUUID", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		cadence.String("A.0x1.TopShot.NFT"),
		cadence.UInt64(42),
		cadence.UInt64(100),
		fromValue,
		cadence.UInt64(5),
	}).WithType(eventCadenceType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             eventType,
		TransactionIndex: txIndex,
		EventIndex:       0,
		Payload:          payload,
	}
}

// ===== Event Address Extraction Tests =====

func TestAccountTransactionsIndexer_EventAddresses(t *testing.T) {
	const testHeight = uint64(100)

	simpleTx := func(payer flow.Address) flow.TransactionBody {
		return unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})
	}

	// --- Generic event field type tests ---

	t.Run("event with direct Address field", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		eventAddr := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "AccountCreated",
			[]cadence.Field{
				{Identifier: "address", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(eventAddr),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, eventAddr, txID, false)
	})

	t.Run("event with Optional Address field non-nil", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		optAddr := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "Transfer",
			[]cadence.Field{
				{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
			},
			[]cadence.Value{
				cadence.NewOptional(cadence.NewAddress(optAddr)),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, optAddr, txID, false)
	})

	t.Run("event with Optional Address field nil", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "Transfer",
			[]cadence.Field{
				{Identifier: "to", Type: cadence.NewOptionalType(cadence.AddressType)},
			},
			[]cadence.Value{
				cadence.NewOptional(nil),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	t.Run("event with multiple Address fields", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		addr1 := unittest.RandomAddressFixture()
		addr2 := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "MultiTransfer",
			[]cadence.Field{
				{Identifier: "from", Type: cadence.AddressType},
				{Identifier: "to", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(addr1),
				cadence.NewAddress(addr2),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, addr1, txID, false)
		assertAccountTx(t, store, testHeight, addr2, txID, false)
	})

	t.Run("event with mixed Address and Optional Address fields", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		directAddr := unittest.RandomAddressFixture()
		optionalAddr := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "ComplexEvent",
			[]cadence.Field{
				{Identifier: "name", Type: cadence.StringType},
				{Identifier: "creator", Type: cadence.AddressType},
				{Identifier: "amount", Type: cadence.UFix64Type},
				{Identifier: "recipient", Type: cadence.NewOptionalType(cadence.AddressType)},
				{Identifier: "count", Type: cadence.UInt64Type},
			},
			[]cadence.Value{
				cadence.String("test"),
				cadence.NewAddress(directAddr),
				cadence.UFix64(100_00000000),
				cadence.NewOptional(cadence.NewAddress(optionalAddr)),
				cadence.UInt64(5),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, directAddr, txID, false)
		assertAccountTx(t, store, testHeight, optionalAddr, txID, false)
	})

	t.Run("event with no address fields", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "AmountUpdated",
			[]cadence.Field{
				{Identifier: "amount", Type: cadence.UFix64Type},
				{Identifier: "name", Type: cadence.StringType},
			},
			[]cadence.Value{
				cadence.UFix64(100_00000000),
				cadence.String("test"),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertTransactionCount(t, store, testHeight, payer, 1)
		assertNoTransactions(t, store, testHeight, unittest.RandomAddressFixture())
	})

	t.Run("event with Optional non-address is ignored", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "ValueChanged",
			[]cadence.Field{
				{Identifier: "value", Type: cadence.NewOptionalType(cadence.UInt64Type)},
				{Identifier: "label", Type: cadence.NewOptionalType(cadence.StringType)},
			},
			[]cadence.Value{
				cadence.NewOptional(cadence.UInt64(42)),
				cadence.NewOptional(cadence.String("test")),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	// --- FT/NFT event tests ---

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	ftAddr := sc.FungibleToken.Address.Hex()
	nftAddr := sc.NonFungibleToken.Address.Hex()
	ftDepositedType := flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Deposited", ftAddr))
	ftWithdrawnType := flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Withdrawn", ftAddr))
	nftDepositedType := flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Deposited", nftAddr))
	nftWithdrawnType := flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Withdrawn", nftAddr))

	t.Run("FT deposited event adds recipient", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {depositEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, recipient, txID, false)
	})

	t.Run("FT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		withdrawEvent := createFTWithdrawnEvent(t, ftWithdrawnType, 0, &sender)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {withdrawEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, sender, txID, false)
	})

	t.Run("NFT deposited event adds recipient", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		depositEvent := createNFTDepositedEvent(t, nftDepositedType, 0, &recipient)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {depositEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, recipient, txID, false)
	})

	t.Run("NFT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		withdrawEvent := createNFTWithdrawnEvent(t, nftWithdrawnType, 0, &sender)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {withdrawEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, sender, txID, false)
	})

	t.Run("FT event with nil address is skipped", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, nil)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {depositEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	// --- Deduplication tests ---

	t.Run("event address same as payer does not duplicate", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "AccountCreated",
			[]cadence.Field{
				{Identifier: "address", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(payer),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		// Payer should appear exactly once despite also being in the event
		assertTransactionCount(t, store, testHeight, payer, 1)
		assertAccountTx(t, store, testHeight, payer, tx.ID(), true)
	})

	t.Run("event address does not override authorizer status", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		// recipient is an authorizer on the transaction
		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{recipient}
		})

		// event also references recipient (as non-authorizer), should not downgrade
		event := createTestEvent(t, 0, "Transfer",
			[]cadence.Field{
				{Identifier: "to", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(recipient),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, recipient, txID, true) // authorizer status preserved
		assertAccountTx(t, store, testHeight, payer, txID, false)
		assertTransactionCount(t, store, testHeight, recipient, 1)
	})

	// --- Multi-event / multi-transaction tests ---

	t.Run("multiple events in same transaction", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		withdrawEvent := createFTWithdrawnEvent(t, ftWithdrawnType, 0, &sender)
		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {withdrawEvent, depositEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, sender, txID, false)
		assertAccountTx(t, store, testHeight, recipient, txID, false)
	})

	t.Run("events across multiple transactions with correct txIndex", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		account1 := unittest.RandomAddressFixture()
		account2 := unittest.RandomAddressFixture()
		recipient1 := unittest.RandomAddressFixture()
		recipient2 := unittest.RandomAddressFixture()

		tx1 := simpleTx(account1)
		tx2 := simpleTx(account2)

		event1 := createFTDepositedEvent(t, ftDepositedType, 0, &recipient1)
		event2 := createNFTDepositedEvent(t, nftDepositedType, 1, &recipient2)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx1, &tx2},
			Events: map[uint32][]flow.Event{
				0: {event1},
				1: {event2},
			},
		})

		// tx1: account1 (authorizer) + recipient1 (from FT event)
		assertAccountTx(t, store, testHeight, account1, tx1.ID(), true)
		assertAccountTx(t, store, testHeight, recipient1, tx1.ID(), false)

		// tx2: account2 (authorizer) + recipient2 (from NFT event)
		assertAccountTx(t, store, testHeight, account2, tx2.ID(), true)
		assertAccountTx(t, store, testHeight, recipient2, tx2.ID(), false)

		// recipients should only be associated with their respective transactions
		assertTransactionCount(t, store, testHeight, recipient1, 1)
		assertTransactionCount(t, store, testHeight, recipient2, 1)
	})

	t.Run("mixed generic and FT events in same block", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		createdAddr := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		accountCreatedEvent := createTestEvent(t, 0, "AccountCreated",
			[]cadence.Field{
				{Identifier: "address", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(createdAddr),
			},
		)
		ftDepositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {accountCreatedEvent, ftDepositEvent}},
		})

		txID := tx.ID()
		assertAccountTx(t, store, testHeight, payer, txID, true)
		assertAccountTx(t, store, testHeight, createdAddr, txID, false)
		assertAccountTx(t, store, testHeight, recipient, txID, false)
	})

	// --- Error tests ---

	t.Run("malformed event payload returns error", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		badEvent := flow.Event{
			Type:             ftDepositedType,
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          []byte("not valid ccf"),
		}

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {badEvent},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to extract addresses from event")

		// Height should not have been updated
		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, testHeight-1, latest)
	})
}

// ===== Sentinel Error Tests =====

func TestAccountTransactionsIndexer_SentinelErrors(t *testing.T) {
	const testHeight = uint64(100)

	t.Run("returns ErrFutureHeight for height beyond latest+1", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight+10))
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
		})
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("returns ErrAlreadyIndexed for height below latest", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight-5))
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
		})
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})

	t.Run("same height as latest is a noop", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight-1))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
		})
		require.NoError(t, err)

		// Height should not have changed
		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, testHeight-1, latest)
	})
}

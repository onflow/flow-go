package extended

import (
	"fmt"
	"os"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

func newBatchForTest(t *testing.T) storage.Batch {
	pdb, dbDir := unittest.TempPebbleDB(t)
	batch := pebbleimpl.NewReaderBatchWriter(pdb)
	t.Cleanup(func() {
		require.NoError(t, batch.Close())
		require.NoError(t, pdb.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})
	return batch
}

func newAccountTxIndexerForTest(
	t *testing.T,
	chainID flow.ChainID,
	latestHeight uint64,
) (*AccountTransactions, *storagemock.AccountTransactions) {
	store := storagemock.NewAccountTransactions(t)
	store.On("LatestIndexedHeight").Return(latestHeight, nil).Maybe()
	return NewAccountTransactions(zerolog.Nop(), store, chainID), store
}

func TestAccountTransactionsIndexer(t *testing.T) {
	const testHeight = uint64(100)

	t.Run("empty block", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)

		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: nil,
			Events:       map[uint32][]flow.Event{},
		}, batch)
		require.NoError(t, err)
		assert.Empty(t, captured)
	})

	t.Run("single transaction with distinct accounts", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

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

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)

		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 4)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, false)
		assertAccountEntry(t, addrMap, proposer, false)
		assertAccountEntry(t, addrMap, auth1, true)
		assertAccountEntry(t, addrMap, auth2, true)
	})

	t.Run("payer is also authorizer", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		proposer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(
			func(tb *flow.TransactionBody) {
				tb.Payer = payer
				tb.ProposalKey = flow.ProposalKey{Address: proposer}
				tb.Authorizers = []flow.Address{payer}
			},
		)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)

		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, proposer, false)
	})

	t.Run("multiple transactions", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

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

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)

		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx1, &tx2, &tx3},
			Events:       map[uint32][]flow.Event{},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 3)

		entriesByTx := make(map[flow.Identifier][]access.AccountTransaction)
		for _, entry := range captured {
			entriesByTx[entry.TransactionID] = append(entriesByTx[entry.TransactionID], entry)
		}

		tx1Addrs := toAddressMap(entriesByTx[tx1.ID()])
		require.Len(t, tx1Addrs, 1)
		assertAccountEntry(t, tx1Addrs, account1, true)

		tx2Addrs := toAddressMap(entriesByTx[tx2.ID()])
		require.Len(t, tx2Addrs, 1)
		assertAccountEntry(t, tx2Addrs, account2, true)

		tx3Addrs := toAddressMap(entriesByTx[tx3.ID()])
		require.Len(t, tx3Addrs, 1)
		assertAccountEntry(t, tx3Addrs, account3, true)
	})
}

// Helper functions for creating CCF-encoded transfer events

// createFTDepositedEvent creates a CCF-encoded FungibleToken.Deposited event
func createFTDepositedEvent(t *testing.T, eventType flow.EventType, txIndex uint32, toAddr *flow.Address) flow.Event {
	// Build the "to" field as optional Address
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

func TestAccountTransactionsIndexer_TransferEvents(t *testing.T) {
	const testHeight = uint64(100)
	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	ftAddr := sc.FungibleToken.Address.Hex()
	nftAddr := sc.NonFungibleToken.Address.Hex()
	ftDepositedType := flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Deposited", ftAddr))
	ftWithdrawnType := flow.EventType(fmt.Sprintf("A.%s.FungibleToken.Withdrawn", ftAddr))
	nftDepositedType := flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Deposited", nftAddr))
	nftWithdrawnType := flow.EventType(fmt.Sprintf("A.%s.NonFungibleToken.Withdrawn", nftAddr))

	t.Run("FT deposited event adds recipient", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("FT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createFTWithdrawnEvent(t, ftWithdrawnType, 0, &sender)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {withdrawEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
	})

	t.Run("NFT deposited event adds recipient", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createNFTDepositedEvent(t, nftDepositedType, 0, &recipient)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("NFT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createNFTWithdrawnEvent(t, nftWithdrawnType, 0, &sender)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {withdrawEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
	})

	t.Run("transfer event with nil address is skipped", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, nil)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 1)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
	})

	t.Run("transfer recipient same as payer does not duplicate", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &payer)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 1)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
	})

	t.Run("transfer recipient does not override authorizer status", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{recipient}
		})

		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 2)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, recipient, true)
		assertAccountEntry(t, addrMap, payer, false)
	})

	t.Run("multiple transfer events in same transaction", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()
		sender := unittest.RandomAddressFixture()
		recipient := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		withdrawEvent := createFTWithdrawnEvent(t, ftWithdrawnType, 0, &sender)
		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, &recipient)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {withdrawEvent, depositEvent},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 3)

		addrMap := toAddressMap(captured)
		assertAccountEntry(t, addrMap, payer, true)
		assertAccountEntry(t, addrMap, sender, false)
		assertAccountEntry(t, addrMap, recipient, false)
	})

	t.Run("events across multiple transactions with correct txIndex", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		account1 := unittest.RandomAddressFixture()
		account2 := unittest.RandomAddressFixture()
		recipient1 := unittest.RandomAddressFixture()
		recipient2 := unittest.RandomAddressFixture()

		tx1 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = account1
			tb.ProposalKey = flow.ProposalKey{Address: account1}
			tb.Authorizers = []flow.Address{account1}
		})

		tx2 := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = account2
			tb.ProposalKey = flow.ProposalKey{Address: account2}
			tb.Authorizers = []flow.Address{account2}
		})

		event1 := createFTDepositedEvent(t, ftDepositedType, 0, &recipient1)
		event2 := createNFTDepositedEvent(t, nftDepositedType, 1, &recipient2)

		var captured []access.AccountTransaction
		store.
			On("Store", testHeight, mock.AnythingOfType("[]access.AccountTransaction"), mock.Anything).
			Run(func(args mock.Arguments) {
				captured = args.Get(1).([]access.AccountTransaction)
			}).
			Return(nil).
			Once()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx1, &tx2},
			Events: map[uint32][]flow.Event{
				0: {event1},
				1: {event2},
			},
		}, batch)
		require.NoError(t, err)
		require.Len(t, captured, 4)

		entriesByTx := make(map[flow.Identifier][]access.AccountTransaction)
		for _, e := range captured {
			entriesByTx[e.TransactionID] = append(entriesByTx[e.TransactionID], e)
		}

		tx1Addrs := toAddressMap(entriesByTx[tx1.ID()])
		require.Len(t, tx1Addrs, 2)
		assertAccountEntry(t, tx1Addrs, account1, true)
		assertAccountEntry(t, tx1Addrs, recipient1, false)

		tx2Addrs := toAddressMap(entriesByTx[tx2.ID()])
		require.Len(t, tx2Addrs, 2)
		assertAccountEntry(t, tx2Addrs, account2, true)
		assertAccountEntry(t, tx2Addrs, recipient2, false)
	})

	t.Run("malformed event payload returns error", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store := newAccountTxIndexerForTest(t, flow.Testnet, testHeight-1)

		payer := unittest.RandomAddressFixture()

		tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})

		badEvent := flow.Event{
			Type:             ftDepositedType,
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          []byte("not valid ccf"),
		}

		store.
			On("Store", mock.Anything, mock.Anything, mock.Anything).
			Return(nil).
			Maybe()

		batch := newBatchForTest(t)
		err := indexer.IndexBlockData(BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events: map[uint32][]flow.Event{
				0: {badEvent},
			},
		}, batch)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to extract addresses from event")
	})
}

func toAddressMap(entries []access.AccountTransaction) map[flow.Address]bool {
	addrMap := make(map[flow.Address]bool)
	for _, e := range entries {
		addrMap[e.Address] = e.IsAuthorizer
	}
	return addrMap
}

func assertAccountEntry(t *testing.T, addrMap map[flow.Address]bool, addr flow.Address, expectedIsAuthorizer bool) {
	isAuthorizer, ok := addrMap[addr]
	require.True(t, ok)
	assert.Equal(t, expectedIsAuthorizer, isAuthorizer)
}

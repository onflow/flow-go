package extended

import (
	"fmt"
	"os"
	"testing"

	"github.com/jordanschalm/lockctx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/indexes"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/operation/pebbleimpl"
	"github.com/onflow/flow-go/utils/unittest"
)

// ===== Basic Indexing Tests =====

func TestAccountTransactionsIndexer(t *testing.T) {
	t.Parallel()
	const testHeight = uint64(100)

	t.Run("empty block", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertTransactionCount(t, store, testHeight, unittest.RandomAddressFixture(), 0)
	})

	t.Run("single transaction with distinct accounts", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, []access.TransactionRole{access.TransactionRolePayer})
		assertAccountTxRoles(t, store, testHeight, proposer, txID, []access.TransactionRole{access.TransactionRoleProposer})
		assertAccountTxRoles(t, store, testHeight, auth1, txID, []access.TransactionRole{access.TransactionRoleAuthorizer})
		assertAccountTxRoles(t, store, testHeight, auth2, txID, []access.TransactionRole{access.TransactionRoleAuthorizer})
	})

	t.Run("payer is also authorizer", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, []access.TransactionRole{access.TransactionRoleAuthorizer, access.TransactionRolePayer})
		assertAccountTxRoles(t, store, testHeight, proposer, txID, []access.TransactionRole{access.TransactionRoleProposer})
		// payer should be deduplicated to one entry
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	t.Run("payer is all roles", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, []access.TransactionRole{access.TransactionRoleAuthorizer, access.TransactionRolePayer, access.TransactionRoleProposer})
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	t.Run("multiple transactions", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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

		allRoles := []access.TransactionRole{access.TransactionRoleAuthorizer, access.TransactionRolePayer, access.TransactionRoleProposer}
		assertAccountTxRoles(t, store, testHeight, account1, tx1.ID(), allRoles)
		assertAccountTxRoles(t, store, testHeight, account2, tx2.ID(), allRoles)
		assertAccountTxRoles(t, store, testHeight, account3, tx3.ID(), allRoles)

		// Each account should only have 1 transaction
		assertTransactionCount(t, store, testHeight, account1, 1)
		assertTransactionCount(t, store, testHeight, account2, 1)
		assertTransactionCount(t, store, testHeight, account3, 1)
	})
}

// ===== Event Address Extraction Tests =====

func TestAccountTransactionsIndexer_EventAddresses(t *testing.T) {
	t.Parallel()
	const testHeight = uint64(100)

	simpleTx := func(payer flow.Address) flow.TransactionBody {
		return unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
			tb.Payer = payer
			tb.ProposalKey = flow.ProposalKey{Address: payer}
			tb.Authorizers = []flow.Address{payer}
		})
	}

	// Common role sets for simpleTx (payer=proposer=authorizer) and event-only addresses
	simpleTxRoles := []access.TransactionRole{access.TransactionRoleAuthorizer, access.TransactionRolePayer, access.TransactionRoleProposer}
	interactionRoles := []access.TransactionRole{access.TransactionRoleInteracted}

	// --- Generic event address extraction ---
	// Tests that extractAddresses handles all cadence field types correctly in a single event:
	// Address fields are extracted, Optional<Address> (non-nil) are extracted,
	// Optional<Address> (nil) are skipped, and non-address fields are ignored.

	t.Run("extracts addresses from mixed event field types", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		payer := unittest.RandomAddressFixture()
		directAddr := unittest.RandomAddressFixture()
		optionalAddr := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		event := createTestEvent(t, 0, "ComplexEvent",
			[]cadence.Field{
				{Identifier: "name", Type: cadence.StringType},                                // non-address: ignored
				{Identifier: "creator", Type: cadence.AddressType},                            // Address: extracted
				{Identifier: "amount", Type: cadence.UFix64Type},                              // non-address: ignored
				{Identifier: "recipient", Type: cadence.NewOptionalType(cadence.AddressType)}, // Optional<Address> non-nil: extracted
				{Identifier: "nilAddr", Type: cadence.NewOptionalType(cadence.AddressType)},   // Optional<Address> nil: skipped
				{Identifier: "optNum", Type: cadence.NewOptionalType(cadence.UInt64Type)},     // Optional<UInt64>: ignored
				{Identifier: "count", Type: cadence.UInt64Type},                               // non-address: ignored
			},
			[]cadence.Value{
				cadence.String("test"),
				cadence.NewAddress(directAddr),
				cadence.UFix64(100_00000000),
				cadence.NewOptional(cadence.NewAddress(optionalAddr)),
				cadence.NewOptional(nil),
				cadence.NewOptional(cadence.UInt64(42)),
				cadence.UInt64(5),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, directAddr, txID, interactionRoles)
		assertAccountTxRoles(t, store, testHeight, optionalAddr, txID, interactionRoles)
		// payer + 2 event addresses = 3 total entries for this tx
		assertTransactionCount(t, store, testHeight, payer, 1)
		assertTransactionCount(t, store, testHeight, directAddr, 1)
		assertTransactionCount(t, store, testHeight, optionalAddr, 1)
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
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, recipient, txID, interactionRoles)
	})

	t.Run("FT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, sender, txID, interactionRoles)
	})

	t.Run("NFT deposited event adds recipient", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, recipient, txID, interactionRoles)
	})

	t.Run("NFT withdrawn event adds sender", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, sender, txID, interactionRoles)
	})

	t.Run("FT event with nil address is skipped", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)
		depositEvent := createFTDepositedEvent(t, ftDepositedType, 0, nil)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {depositEvent}},
		})

		txID := tx.ID()
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertTransactionCount(t, store, testHeight, payer, 1)
	})

	// --- Deduplication tests ---

	t.Run("event address same as payer does not duplicate", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		// Payer has all simpleTx roles plus Interaction from the event
		simpleTxPlusInteraction := []access.TransactionRole{
			access.TransactionRoleAuthorizer, access.TransactionRolePayer, access.TransactionRoleProposer, access.TransactionRoleInteracted,
		}
		assertAccountTxRoles(t, store, testHeight, payer, tx.ID(), simpleTxPlusInteraction)
	})

	t.Run("duplicate event addresses in same transaction are deduplicated", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		payer := unittest.RandomAddressFixture()
		eventAddr := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		// Two different events both referencing the same address
		event1 := createTestEvent(t, 0, "Transfer",
			[]cadence.Field{
				{Identifier: "to", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(eventAddr),
			},
		)
		event2 := createTestEvent(t, 0, "Approval",
			[]cadence.Field{
				{Identifier: "from", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(eventAddr),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event1, event2}},
		})

		// eventAddr should appear exactly once with a single Interaction role
		assertTransactionCount(t, store, testHeight, eventAddr, 1)
		assertAccountTxRoles(t, store, testHeight, eventAddr, tx.ID(), interactionRoles)
	})

	t.Run("event address does not override authorizer status", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		// recipient is authorizer and also referenced in event → both roles preserved
		assertAccountTxRoles(t, store, testHeight, recipient, txID, []access.TransactionRole{access.TransactionRoleAuthorizer, access.TransactionRoleInteracted})
		// payer is payer + proposer (not authorizer in this test)
		assertAccountTxRoles(t, store, testHeight, payer, txID, []access.TransactionRole{access.TransactionRolePayer, access.TransactionRoleProposer})
		assertTransactionCount(t, store, testHeight, recipient, 1)
	})

	// --- Multi-event / multi-transaction tests ---

	t.Run("multiple events in same transaction", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, sender, txID, interactionRoles)
		assertAccountTxRoles(t, store, testHeight, recipient, txID, interactionRoles)
	})

	t.Run("events across multiple transactions with correct txIndex", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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

		// tx1: account1 (payer+proposer+authorizer) + recipient1 (from FT event)
		assertAccountTxRoles(t, store, testHeight, account1, tx1.ID(), simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, recipient1, tx1.ID(), interactionRoles)

		// tx2: account2 (payer+proposer+authorizer) + recipient2 (from NFT event)
		assertAccountTxRoles(t, store, testHeight, account2, tx2.ID(), simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, recipient2, tx2.ID(), interactionRoles)

		// recipients should only be associated with their respective transactions
		assertTransactionCount(t, store, testHeight, recipient1, 1)
		assertTransactionCount(t, store, testHeight, recipient2, 1)
	})

	t.Run("mixed generic and FT events in same block", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertAccountTxRoles(t, store, testHeight, createdAddr, txID, interactionRoles)
		assertAccountTxRoles(t, store, testHeight, recipient, txID, interactionRoles)
	})

	// --- Invalid address filtering ---

	t.Run("event address invalid for chain is ignored", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		payer := unittest.RandomAddressFixture()
		tx := simpleTx(payer)

		// Use an address that is invalid for testnet's linear code address scheme.
		invalidAddr := flow.BytesToAddress([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
		require.False(t, flow.Testnet.Chain().IsValid(invalidAddr), "test requires an invalid address")

		event := createTestEvent(t, 0, "SomeEvent",
			[]cadence.Field{
				{Identifier: "account", Type: cadence.AddressType},
			},
			[]cadence.Value{
				cadence.NewAddress(invalidAddr),
			},
		)

		indexBlock(t, indexer, lm, db, BlockData{
			Header:       header,
			Transactions: []*flow.TransactionBody{&tx},
			Events:       map[uint32][]flow.Event{0: {event}},
		})

		txID := tx.ID()
		assertAccountTxRoles(t, store, testHeight, payer, txID, simpleTxRoles)
		assertTransactionCount(t, store, testHeight, invalidAddr, 0)
	})

	// --- Error tests ---

	t.Run("malformed event payload returns error", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

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

		// Store should not have been initialized
		_, err = store.LatestIndexedHeight()
		require.ErrorIs(t, err, storage.ErrNotBootstrapped)
	})
}

// ===== Height Validation Tests =====

func TestAccountTransactionsIndexer_HeightValidation(t *testing.T) {
	t.Parallel()
	const testHeight = uint64(100)

	t.Run("uninitialized indexer accepts first expected height", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		indexBlock(t, indexer, lm, db, BlockData{
			Header: header,
		})

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, testHeight, latest)
	})

	t.Run("uninitialized indexer returns ErrFutureHeight for wrong height", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight+10))
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
		})
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("uninitialized indexer returns ErrAlreadyIndexed for height below first", func(t *testing.T) {
		header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight-5))
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		err := indexBlockExpectError(t, indexer, lm, db, BlockData{
			Header: header,
		})
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})

	t.Run("initialized indexer accepts next sequential height", func(t *testing.T) {
		indexer, store, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		// Initialize by indexing the first block
		header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexBlock(t, indexer, lm, db, BlockData{Header: header1})

		// Index the next sequential block
		header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight+1))
		indexBlock(t, indexer, lm, db, BlockData{Header: header2})

		latest, err := store.LatestIndexedHeight()
		require.NoError(t, err)
		assert.Equal(t, testHeight+1, latest)
	})

	t.Run("initialized indexer returns ErrFutureHeight for non-sequential height", func(t *testing.T) {
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		// Initialize
		header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexBlock(t, indexer, lm, db, BlockData{Header: header1})

		// Skip a height
		header3 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight+5))
		err := indexBlockExpectError(t, indexer, lm, db, BlockData{Header: header3})
		require.ErrorIs(t, err, ErrFutureHeight)
	})

	t.Run("initialized indexer returns ErrAlreadyIndexed for past height", func(t *testing.T) {
		indexer, _, lm, db := newAccountTxIndexerForTest(t, flow.Testnet, testHeight)

		// Initialize
		header1 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		indexBlock(t, indexer, lm, db, BlockData{Header: header1})

		// Try to re-index the same height
		header2 := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))
		err := indexBlockExpectError(t, indexer, lm, db, BlockData{Header: header2})
		require.ErrorIs(t, err, ErrAlreadyIndexed)
	})
}

// ===== Mock-Based Error Path Tests =====

func TestAccountTransactionsIndexer_NextHeight(t *testing.T) {
	t.Parallel()

	t.Run("unexpected error from LatestIndexedHeight propagates", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsBootstrapper(t)
		unexpectedErr := fmt.Errorf("disk I/O error")
		mockStore.On("LatestIndexedHeight").Return(uint64(0), unexpectedErr)

		lm := storage.NewTestingLockManager()
		indexer, err := NewAccountTransactions(unittest.Logger(), mockStore, flow.Testnet, lm)
		require.NoError(t, err)

		_, err = indexer.NextHeight()
		require.Error(t, err)
		require.ErrorIs(t, err, unexpectedErr)
	})

	t.Run("inconsistent state: not bootstrapped but initialized", func(t *testing.T) {
		mockStore := storagemock.NewAccountTransactionsBootstrapper(t)
		mockStore.On("LatestIndexedHeight").Return(uint64(0), storage.ErrNotBootstrapped)
		mockStore.On("UninitializedFirstHeight").Return(uint64(42), true)

		lm := storage.NewTestingLockManager()
		indexer, err := NewAccountTransactions(unittest.Logger(), mockStore, flow.Testnet, lm)
		require.NoError(t, err)

		_, err = indexer.NextHeight()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "but index is initialized")
	})
}

func TestAccountTransactionsIndexer_StoreErrorPropagation(t *testing.T) {
	t.Parallel()

	const testHeight = uint64(100)

	mockStore := storagemock.NewAccountTransactionsBootstrapper(t)
	// NextHeight() calls LatestIndexedHeight -> returns 99 -> NextHeight = 100
	mockStore.On("LatestIndexedHeight").Return(testHeight-1, nil)

	storeErr := fmt.Errorf("unexpected storage error")
	mockStore.On("Store", mock.Anything, mock.Anything, testHeight, mock.Anything).Return(storeErr)

	lm := storage.NewTestingLockManager()
	indexer, err := NewAccountTransactions(unittest.Logger(), mockStore, flow.Testnet, lm)
	require.NoError(t, err)

	header := unittest.BlockHeaderFixtureOnChain(flow.Testnet, unittest.WithHeaderHeight(testHeight))

	// Call IndexBlockData directly with nil batch since the mock Store doesn't use it
	err = unittest.WithLock(t, lm, storage.LockIndexAccountTransactions, func(lctx lockctx.Context) error {
		return indexer.IndexBlockData(lctx, BlockData{Header: header}, nil)
	})
	require.Error(t, err)
	require.ErrorIs(t, err, storeErr)
}

// ===== Test Setup Helpers =====

func newAccountTxIndexerForTest(
	t *testing.T,
	chainID flow.ChainID,
	latestHeight uint64,
) (*AccountTransactions, storage.AccountTransactionsBootstrapper, storage.LockManager, storage.DB) {
	pdb, dbDir := unittest.TempPebbleDB(t)
	db := pebbleimpl.ToDB(pdb)
	t.Cleanup(func() {
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(dbDir))
	})

	lm := storage.NewTestingLockManager()
	store, err := indexes.NewAccountTransactionsBootstrapper(db, latestHeight)
	require.NoError(t, err)

	indexer, err := NewAccountTransactions(unittest.Logger(), store, chainID, lm)
	require.NoError(t, err)
	return indexer, store, lm, db
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

// assertAccountTxRoles verifies that a specific transaction is indexed for the given address
// with the expected roles.
func assertAccountTxRoles(
	t *testing.T,
	store storage.AccountTransactionsReader,
	height uint64,
	addr flow.Address,
	txID flow.Identifier,
	expectedRoles []access.TransactionRole,
) {
	t.Helper()
	page, err := store.TransactionsByAddress(addr, 1000, nil, nil)
	require.NoError(t, err)

	for _, r := range page.Transactions {
		if r.TransactionID == txID && r.BlockHeight == height {
			assert.Equal(t, expectedRoles, r.Roles,
				"address %s tx %s: expected roles=%v, got roles=%v", addr, txID, expectedRoles, r.Roles)
			return
		}
	}
	t.Errorf("transaction %s not found for address %s at height %d", txID, addr, height)
}

// assertTransactionCount verifies the total number of transactions for an address at a height.
func assertTransactionCount(
	t *testing.T,
	store storage.AccountTransactionsReader,
	height uint64,
	addr flow.Address,
	expectedCount int,
) {
	t.Helper()
	page, err := store.TransactionsByAddress(addr, 1000, nil, nil)
	require.NoError(t, err)

	var count int
	for _, r := range page.Transactions {
		if r.BlockHeight == height {
			count++
		}
	}
	require.Equal(t, expectedCount, count,
		"expected %d transactions for address %s at height %d", expectedCount, addr, height)
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

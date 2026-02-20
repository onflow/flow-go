package transfers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers/testutil"
	"github.com/onflow/flow-go/utils/unittest"
)

// ==========================================================================
// NFT Transfer Tests
// ==========================================================================

func TestParseNFTTransfers_PairedTransfer(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftUUID := uint64(100)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, nftUUID, nftID),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 1, nftUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, recipient, nftID, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_UnpairedDeposit(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftID := uint64(7)
	events := []flow.Event{
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 0, 999, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, nftID, 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_UnpairedWithdrawal(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftID := uint64(13)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 888, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, nftID, 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_NilOptionalAddresses(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	uuid := uint64(50)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 0, uuid, 1),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, nil, txID, 0, 1, uuid, 1),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, flow.Address{}, 1, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_MultiplePairsInSameTx(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender1, txID, 0, 0, 10, 1),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient1, txID, 0, 1, 10, 1),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender2, txID, 0, 2, 20, 2),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient2, txID, 0, 3, 20, 2),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender1, recipient1, 1, 0, 1),
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender2, recipient2, 2, 2, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_PairedUsesDepositEventIndex verifies that paired NFT transfers
// include both the Withdrawn and Deposited event indices.
func TestParseNFTTransfers_PairedUsesDepositEventIndex(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(1)
	nftID := uint64(111)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, uuid, nftID),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 5, uuid, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, recipient, nftID, 0, 5),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_MalformedPayload(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)

	t.Run("malformed withdrawn event", func(t *testing.T) {
		events := []flow.Event{
			{
				Type:    testutil.NFTWithdrawnEventType(flow.Testnet),
				Payload: []byte("not valid ccf"),
			},
		}
		transfers, err := parser.Parse(events, testBlockHeight)
		require.Error(t, err)
		assert.Nil(t, transfers)
	})

	t.Run("malformed deposited event", func(t *testing.T) {
		events := []flow.Event{
			{
				Type:    testutil.NFTDepositedEventType(flow.Testnet),
				Payload: []byte("not valid ccf"),
			},
		}
		transfers, err := parser.Parse(events, testBlockHeight)
		require.Error(t, err)
		assert.Nil(t, transfers)
	})
}

// TestParseNFTTransfers_EventsAcrossTransactionsDoNotPair verifies that events
// with the same UUID but in different transactions are NOT paired together.
func TestParseNFTTransfers_EventsAcrossTransactionsDoNotPair(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sharedUUID := uint64(42)
	nftID := uint64(7)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID1, 0, 0, sharedUUID, nftID),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID2, 1, 0, sharedUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID1, 0, sender, flow.Address{}, nftID, 0),
		testutil.MakeNFTTransfer(testBlockHeight, txID2, 1, flow.Address{}, recipient, nftID, 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_DepositBeforeWithdrawal verifies that when a deposit is processed
// before a withdrawal with the same UUID, they are NOT paired. The deposit is treated as
// a mint and the withdrawal as a burn.
func TestParseNFTTransfers_DepositBeforeWithdrawal(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(55)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 0, uuid, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 1, uuid, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, nftID, 0),
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, nftID, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseNFTTransfers_SkipsIrrelevantEvents(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)

	events := []flow.Event{
		{
			Type:             "A.f233dcee88fe0abe.FlowToken.TokensMinted",
			TransactionID:    unittest.IdentifierFixture(),
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          []byte("irrelevant"),
		},
		{
			Type:             "A.1234567890abcdef.SomeContract.SomeEvent",
			TransactionID:    unittest.IdentifierFixture(),
			TransactionIndex: 0,
			EventIndex:       1,
			Payload:          []byte("also irrelevant"),
		},
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.Empty(t, transfers)
}

func TestParseNFTTransfers_MixedPairedAndUnpaired(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	mintRecipient := unittest.RandomAddressFixture()
	burnSender := unittest.RandomAddressFixture()

	events := []flow.Event{
		// Paired transfer
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 100, 1),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 1, 100, 1),
		// Unpaired deposit (mint)
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &mintRecipient, txID, 0, 2, 200, 2),
		// Unpaired withdrawal (burn)
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &burnSender, txID, 0, 3, 300, 3),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, recipient, 1, 0, 1),
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, mintRecipient, 2, 2),
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, burnSender, flow.Address{}, 3, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_MultipleWithdrawalsBeforeDeposit verifies that multiple Withdrawn events
// for the same NFT UUID within a transaction are valid and produce a single transfer. All
// withdrawal source events are included in the EventIndices of the resulting transfer.
func TestParseNFTTransfers_MultipleWithdrawalsBeforeDeposit(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftUUID := uint64(100)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 1, nftUUID, nftID),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 2, nftUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, sender, recipient, nftID, 0, 1, 2),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_MultiHopTransfer verifies that an NFT transferred through multiple
// addresses within a single transaction (A → B → C) produces one transfer per hop.
func TestParseNFTTransfers_MultiHopTransfer(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftUUID := uint64(100)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, nftUUID, nftID), // A → out
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 1, nftUUID, nftID),   // → B
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &bob, txID, 0, 2, nftUUID, nftID),   // B → out
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &carol, txID, 0, 3, nftUUID, nftID), // → C
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, alice, bob, nftID, 0, 1),
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, bob, carol, nftID, 2, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_MultiLayerCollectionTransfer verifies that when an NFT passes through
// multiple layers of collections owned by different accounts within a single transaction, an
// intermediate transfer is produced for each ownership boundary.
//
// Scenario (5 events, addresses labelled by number):
//
//	W(addr1, idx=0), W(addr1, idx=1), W(addr2, idx=2), W(addr3, idx=3), D(addr4, idx=4)
//
// Expected transfers:
//
//	addr3 → addr2  events [2, 3]   (G2 → G1 boundary)
//	addr2 → addr1  events [1, 2]   (G1 → G0 boundary)
//	addr1 → addr4  events [0, 1, 4] (innermost run → deposit)
func TestParseNFTTransfers_MultiLayerCollectionTransfer(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	addr3 := unittest.RandomAddressFixture()
	addr4 := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftUUID := uint64(100)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr1, txID, 0, 0, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr1, txID, 0, 1, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr2, txID, 0, 2, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr3, txID, 0, 3, nftUUID, nftID),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &addr4, txID, 0, 4, nftUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr1, addr4, nftID, 0, 1, 4), // innermost → deposit
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr2, addr1, nftID, 1, 2),    // G1 → G0
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr3, addr2, nftID, 2, 3),    // G2 → G1
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_MultiLayerBurn verifies that when a multi-layer NFT withdrawal has no
// matching deposit, intermediate transfers are still produced for each ownership boundary and
// the innermost run produces a burn record.
//
// Scenario:
//
//	W(addr1, idx=0), W(addr2, idx=1), W(addr3, idx=2), no deposit
//
// Expected transfers:
//
//	addr3 → addr2  events [1, 2]
//	addr2 → addr1  events [0, 1]
//	addr1 → burn   events [0]
func TestParseNFTTransfers_MultiLayerBurn(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	addr1 := unittest.RandomAddressFixture()
	addr2 := unittest.RandomAddressFixture()
	addr3 := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	nftUUID := uint64(100)
	nftID := uint64(42)
	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr1, txID, 0, 0, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr2, txID, 0, 1, nftUUID, nftID),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &addr3, txID, 0, 2, nftUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr1, flow.Address{}, nftID, 0), // burn
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr2, addr1, nftID, 0, 1),       // G1 → G0
		testutil.MakeNFTTransfer(testBlockHeight, txID, 0, addr3, addr2, nftID, 1, 2),       // G2 → G1
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_MultipleTransactionsInBlock verifies that events from
// different transactions in the same block are grouped and paired independently.
func TestParseNFTTransfers_MultipleTransactionsInBlock(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	events := []flow.Event{
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender1, txID1, 0, 0, 10, 1),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient1, txID1, 0, 1, 10, 1),
		testutil.MakeNFTWithdrawnEvent(t, flow.Testnet, &sender2, txID2, 1, 0, 20, 2),
		testutil.MakeNFTDepositedEvent(t, flow.Testnet, &recipient2, txID2, 1, 1, 20, 2),
	}

	expected := []access.NonFungibleTokenTransfer{
		testutil.MakeNFTTransfer(testBlockHeight, txID1, 0, sender1, recipient1, 1, 0, 1),
		testutil.MakeNFTTransfer(testBlockHeight, txID2, 1, sender2, recipient2, 2, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

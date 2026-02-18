package transfers

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const testBlockHeight = uint64(100)

// Event type strings derived from the test chain so they always match
// the addresses resolved by the parsers.
var (
	testFTDepositedType  flow.EventType
	testFTWithdrawnType  flow.EventType
	testNFTDepositedType flow.EventType
	testNFTWithdrawnType flow.EventType
)

// Default token type identifiers used by the test event helpers.
const (
	testFTTokenType  = "A.0000000000000001.FlowToken.Vault"
	testNFTTokenType = "A.0000000000000002.TopShot.NFT"
)

func init() {
	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	testFTDepositedType = flow.EventType(fmt.Sprintf("A.%s.%s", sc.FungibleToken.Address, ftDepositedSuffix))
	testFTWithdrawnType = flow.EventType(fmt.Sprintf("A.%s.%s", sc.FungibleToken.Address, ftWithdrawnSuffix))
	testNFTDepositedType = flow.EventType(fmt.Sprintf("A.%s.%s", sc.NonFungibleToken.Address, nftDepositedSuffix))
	testNFTWithdrawnType = flow.EventType(fmt.Sprintf("A.%s.%s", sc.NonFungibleToken.Address, nftWithdrawnSuffix))
}

// ==========================================================================
// FT Transfer Tests
// ==========================================================================

func TestParseFTTransfers_EmptyEvents(t *testing.T) {
	parser := NewFTParser(flow.Testnet)

	t.Run("nil input", func(t *testing.T) {
		transfers, err := parser.Parse(nil, testBlockHeight)
		require.NoError(t, err)
		assert.Empty(t, transfers)
	})

	t.Run("empty slice", func(t *testing.T) {
		transfers, err := parser.Parse([]flow.Event{}, testBlockHeight)
		require.NoError(t, err)
		assert.Empty(t, transfers)
	})
}

func TestParseFTTransfers_PairedTransfer(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(42)
	amount := cadence.UFix64(50_00000000)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, uuid, amount),
		makeFTDepositedEvent(t, &recipient, txID, 0, 1, uuid, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, sender, recipient, amount, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_AmountMismatch verifies that a deposit paired by UUID with a withdrawal
// returns an error when their amounts don't match. Direct 1:N pairing by UUID is not supported;
// vault splits across multiple recipients use withdrawal chains instead (see
// TestParseFTTransfers_WithdrawalChainResolution).
func TestParseFTTransfers_AmountMismatch(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(42)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, uuid, cadence.UFix64(100_00000000)),
		makeFTDepositedEvent(t, &recipient, txID, 0, 1, uuid, cadence.UFix64(40_00000000)),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not equal to the deposit amount")
}

func getTransfer(blockHeight uint64, txID flow.Identifier, txIndex uint32, source, recipient flow.Address, amount cadence.UFix64, eventIndices ...uint32) access.FungibleTokenTransfer {
	return access.FungibleTokenTransfer{
		TransactionID:    txID,
		BlockHeight:      blockHeight,
		TransactionIndex: txIndex,
		EventIndices:     eventIndices,
		TokenType:        testFTTokenType,
		Amount:           new(big.Int).SetUint64(uint64(amount)),
		SourceAddress:    source,
		RecipientAddress: recipient,
	}
}

func getNFTTransfer(blockHeight uint64, txID flow.Identifier, txIndex uint32, source, recipient flow.Address, nftID uint64, eventIndices ...uint32) access.NonFungibleTokenTransfer {
	return access.NonFungibleTokenTransfer{
		TransactionID:    txID,
		BlockHeight:      blockHeight,
		TransactionIndex: txIndex,
		EventIndices:     eventIndices,
		TokenType:        testNFTTokenType,
		ID:               nftID,
		SourceAddress:    source,
		RecipientAddress: recipient,
	}
}

// TestParseFTTransfers_WithdrawalChainResolution verifies that when a vault is
// withdrawn and then sub-withdrawn into intermediate vaults (which have from=nil),
// the intermediate withdrawals inherit the source address from the original.
//
// Scenario:
//
//	Withdraw 100 from Alice's stored vault (UUID=1) → temp vault UUID=50 (from=Alice)
//	Withdraw 40 from temp vault 50 → temp vault UUID=51 (from=nil, fromUUID=50)
//	Withdraw 25 from temp vault 50 → temp vault UUID=52 (from=nil, fromUUID=50)
//	Deposit temp vault 51 into Bob
//	Deposit temp vault 52 into Carol
//	Deposit temp vault 50 (remaining 35) into Dave
func TestParseFTTransfers_WithdrawalChainResolution(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	dave := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		// Original withdrawal from Alice's stored vault (UUID=1) → temp vault UUID=50
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		// Sub-withdraw from temp vault 50 → temp vault UUID=51
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(40_00000000)),
		// Sub-withdraw from temp vault 50 → temp vault UUID=52
		makeFTWithdrawnEvent(t, nil, txID, 0, 2, 50, 52, cadence.UFix64(25_00000000)),
		// Deposits
		makeFTDepositedEvent(t, &bob, txID, 0, 3, 51, cadence.UFix64(40_00000000)),
		makeFTDepositedEvent(t, &carol, txID, 0, 4, 52, cadence.UFix64(25_00000000)),
		makeFTDepositedEvent(t, &dave, txID, 0, 5, 50, cadence.UFix64(35_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(40_00000000), 0, 1, 3),
		getTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(25_00000000), 0, 2, 4),
		getTransfer(testBlockHeight, txID, 0, alice, dave, cadence.UFix64(35_00000000), 0, 5),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_DeepWithdrawalChain verifies chain resolution works for
// chains deeper than one level (A → B → C).
func TestParseFTTransfers_DeepWithdrawalChain(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	alice := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		// Alice's vault (UUID=1) → temp vault 50
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		// temp vault 50 → temp vault 51
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
		// temp vault 51 → temp vault 52
		makeFTWithdrawnEvent(t, nil, txID, 0, 2, 51, 52, cadence.UFix64(30_00000000)),
		// Deposit the deepest vault
		makeFTDepositedEvent(t, &recipient, txID, 0, 3, 52, cadence.UFix64(30_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Paired: full chain from vault 1→50→51→52 + deposit
		getTransfer(testBlockHeight, txID, 0, alice, recipient, cadence.UFix64(30_00000000), 0, 1, 2, 3),
		// Burn: vault 50 remainder (100 - 60 sub-withdrawn to vault 51)
		getTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(40_00000000), 0),
		// Burn: vault 51 remainder (60 - 30 sub-withdrawn to vault 52)
		getTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(30_00000000), 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_FullyConsumedParent verifies behavior when a temp vault's
// entire content is re-withdrawn into a child vault. The parent withdrawal produces
// a 0-amount burn record since it has no matching deposit.
//
// Scenario:
//
//	Withdraw 100 from Alice's stored vault (UUID=1) → temp vault UUID=50 (from=Alice)
//	Withdraw 100 from temp vault 50 → temp vault UUID=51 (from=nil, fromUUID=50) [full amount]
//	Deposit temp vault 51 into Bob
func TestParseFTTransfers_FullyConsumedParent(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(100_00000000)),
		makeFTDepositedEvent(t, &bob, txID, 0, 2, 51, cadence.UFix64(100_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Paired: Alice → Bob via chain 1→50→51
		getTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(100_00000000), 0, 1, 2),
		// Burn: parent vault 50 fully consumed (0 remaining), appears as 0-amount burn
		getTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(0), 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_FullyConsumedByMultipleChildren verifies behavior when
// multiple children collectively consume a parent's full amount. The parent
// produces a 0-amount burn record since it has no matching deposit.
//
// Scenario:
//
//	Withdraw 100 from Alice's stored vault → temp vault UUID=50 (from=Alice)
//	Withdraw 60 from temp vault 50 → UUID=51 (from=nil)
//	Withdraw 40 from temp vault 50 → UUID=52 (from=nil)
//	Deposit 51 into Bob, Deposit 52 into Carol
func TestParseFTTransfers_FullyConsumedByMultipleChildren(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 2, 50, 52, cadence.UFix64(40_00000000)),
		makeFTDepositedEvent(t, &bob, txID, 0, 3, 51, cadence.UFix64(60_00000000)),
		makeFTDepositedEvent(t, &carol, txID, 0, 4, 52, cadence.UFix64(40_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(60_00000000), 0, 1, 3),
		getTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(40_00000000), 0, 2, 4),
		// Parent vault 50 fully consumed (0 remaining), appears as 0-amount burn
		getTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(0), 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_UnpairedDeposit(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	amount := cadence.UFix64(100_00000000)
	events := []flow.Event{
		makeFTDepositedEvent(t, &recipient, txID, 0, 3, 99, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, amount, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_UnpairedWithdrawal(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	amount := cadence.UFix64(25_00000000)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 5, 1, 77, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, amount, 5),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_NilOptionalAddresses(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	uuid := uint64(10)
	amount := cadence.UFix64(1_00000000)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, nil, txID, 0, 0, 1, uuid, amount),
		makeFTDepositedEvent(t, nil, txID, 0, 1, uuid, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, flow.Address{}, flow.Address{}, amount, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_MultiplePairsInSameTx(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	amount1 := cadence.UFix64(10_00000000)
	amount2 := cadence.UFix64(20_00000000)

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender1, txID, 0, 0, 1, 10, amount1),
		makeFTDepositedEvent(t, &recipient1, txID, 0, 1, 10, amount1),
		makeFTWithdrawnEvent(t, &sender2, txID, 0, 2, 1, 20, amount2),
		makeFTDepositedEvent(t, &recipient2, txID, 0, 3, 20, amount2),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, sender1, recipient1, amount1, 0, 1),
		getTransfer(testBlockHeight, txID, 0, sender2, recipient2, amount2, 2, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_MixedPairedAndUnpaired(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	txID := unittest.IdentifierFixture()

	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	mintRecipient := unittest.RandomAddressFixture()
	burnSender := unittest.RandomAddressFixture()

	amount := cadence.UFix64(10_00000000)

	events := []flow.Event{
		// Paired transfer
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, 100, amount),
		makeFTDepositedEvent(t, &recipient, txID, 0, 1, 100, amount),
		// Unpaired deposit (mint)
		makeFTDepositedEvent(t, &mintRecipient, txID, 0, 2, 200, amount),
		// Unpaired withdrawal (burn)
		makeFTWithdrawnEvent(t, &burnSender, txID, 0, 3, 1, 300, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, sender, recipient, amount, 0, 1),
		getTransfer(testBlockHeight, txID, 0, flow.Address{}, mintRecipient, amount, 2),
		getTransfer(testBlockHeight, txID, 0, burnSender, flow.Address{}, amount, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_EventsAcrossTransactionsDoNotPair verifies that events
// with the same UUID but in different transactions are NOT paired together.
func TestParseFTTransfers_EventsAcrossTransactionsDoNotPair(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sharedUUID := uint64(42)
	amount := cadence.UFix64(10_00000000)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID1, 0, 0, 1, sharedUUID, amount),
		makeFTDepositedEvent(t, &recipient, txID2, 1, 0, sharedUUID, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID1, 0, sender, flow.Address{}, amount, 0),
		getTransfer(testBlockHeight, txID2, 1, flow.Address{}, recipient, amount, 0),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_DepositBeforeWithdrawal verifies that when a deposit is processed
// before a withdrawal with the same UUID, they are NOT paired. Pairing is order-dependent:
// deposits can only match already-seen withdrawals. The deposit is treated as a mint and
// the withdrawal as a burn.
func TestParseFTTransfers_DepositBeforeWithdrawal(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(55)
	amount := cadence.UFix64(30_00000000)

	// Deposit appears before withdrawal in the event list.
	events := []flow.Event{
		makeFTDepositedEvent(t, &recipient, txID, 0, 0, uuid, amount),
		makeFTWithdrawnEvent(t, &sender, txID, 0, 1, 1, uuid, amount),
	}

	// Deposit without matching withdrawal = mint; withdrawal without matching deposit = burn.
	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, amount, 0),
		getTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, amount, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_SkipsIrrelevantEvents(t *testing.T) {
	parser := NewFTParser(flow.Testnet)

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

func TestParseFTTransfers_MalformedPayload(t *testing.T) {
	parser := NewFTParser(flow.Testnet)

	t.Run("malformed withdrawn event", func(t *testing.T) {
		events := []flow.Event{
			{
				Type:    testFTWithdrawnType,
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
				Type:    testFTDepositedType,
				Payload: []byte("not valid ccf"),
			},
		}
		transfers, err := parser.Parse(events, testBlockHeight)
		require.Error(t, err)
		assert.Nil(t, transfers)
	})
}

// TestParseFTTransfers_DuplicateWithdrawalUUID verifies that two withdrawals with the same
// withdrawnUUID in the same transaction produce an error.
func TestParseFTTransfers_DuplicateWithdrawalUUID(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		makeFTWithdrawnEvent(t, &sender, txID, 0, 1, 1, 50, cadence.UFix64(50_00000000)),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate withdrawal resource UUID")
}

// TestParseFTTransfers_ChildExceedsParentAmount verifies that a child withdrawal with an
// amount larger than the parent's remaining balance produces an error.
func TestParseFTTransfers_ChildExceedsParentAmount(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, 50, cadence.UFix64(50_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greater than the remaining parent")
}

// TestParseFTTransfers_MultipleTransactionsInBlock verifies that events from
// different transactions in the same block are grouped and paired independently.
func TestParseFTTransfers_MultipleTransactionsInBlock(t *testing.T) {
	parser := NewFTParser(flow.Testnet)
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	amount1 := cadence.UFix64(10_00000000)
	amount2 := cadence.UFix64(20_00000000)

	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender1, txID1, 0, 0, 1, 10, amount1),
		makeFTDepositedEvent(t, &recipient1, txID1, 0, 1, 10, amount1),
		makeFTWithdrawnEvent(t, &sender2, txID2, 1, 0, 1, 20, amount2),
		makeFTDepositedEvent(t, &recipient2, txID2, 1, 1, 20, amount2),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID1, 0, sender1, recipient1, amount1, 0, 1),
		getTransfer(testBlockHeight, txID2, 1, sender2, recipient2, amount2, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

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
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 0, nftUUID, nftID),
		makeNFTDepositedEvent(t, &recipient, txID, 0, 1, nftUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, sender, recipient, nftID, 0, 1),
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
		makeNFTDepositedEvent(t, &recipient, txID, 0, 0, 999, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, nftID, 0),
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
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 0, 888, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, nftID, 0),
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
		makeNFTWithdrawnEvent(t, nil, txID, 0, 0, uuid, 1),
		makeNFTDepositedEvent(t, nil, txID, 0, 1, uuid, 1),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, flow.Address{}, 1, 0, 1),
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
		makeNFTWithdrawnEvent(t, &sender1, txID, 0, 0, 10, 1),
		makeNFTDepositedEvent(t, &recipient1, txID, 0, 1, 10, 1),
		makeNFTWithdrawnEvent(t, &sender2, txID, 0, 2, 20, 2),
		makeNFTDepositedEvent(t, &recipient2, txID, 0, 3, 20, 2),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, sender1, recipient1, 1, 0, 1),
		getNFTTransfer(testBlockHeight, txID, 0, sender2, recipient2, 2, 2, 3),
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
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 0, uuid, nftID),
		makeNFTDepositedEvent(t, &recipient, txID, 0, 5, uuid, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, sender, recipient, nftID, 0, 5),
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
				Type:    testNFTWithdrawnType,
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
				Type:    testNFTDepositedType,
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
		makeNFTWithdrawnEvent(t, &sender, txID1, 0, 0, sharedUUID, nftID),
		makeNFTDepositedEvent(t, &recipient, txID2, 1, 0, sharedUUID, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID1, 0, sender, flow.Address{}, nftID, 0),
		getNFTTransfer(testBlockHeight, txID2, 1, flow.Address{}, recipient, nftID, 0),
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
		makeNFTDepositedEvent(t, &recipient, txID, 0, 0, uuid, nftID),
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 1, uuid, nftID),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, nftID, 0),
		getNFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, nftID, 1),
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
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 0, 100, 1),
		makeNFTDepositedEvent(t, &recipient, txID, 0, 1, 100, 1),
		// Unpaired deposit (mint)
		makeNFTDepositedEvent(t, &mintRecipient, txID, 0, 2, 200, 2),
		// Unpaired withdrawal (burn)
		makeNFTWithdrawnEvent(t, &burnSender, txID, 0, 3, 300, 3),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID, 0, sender, recipient, 1, 0, 1),
		getNFTTransfer(testBlockHeight, txID, 0, flow.Address{}, mintRecipient, 2, 2),
		getNFTTransfer(testBlockHeight, txID, 0, burnSender, flow.Address{}, 3, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseNFTTransfers_DuplicateWithdrawalUUID verifies that two withdrawals with the same
// UUID in the same transaction produce an error.
func TestParseNFTTransfers_DuplicateWithdrawalUUID(t *testing.T) {
	parser := NewNFTParser(flow.Testnet)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 0, 50, 1),
		makeNFTWithdrawnEvent(t, &sender, txID, 0, 1, 50, 2),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate withdrawal resource UUID")
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
		makeNFTWithdrawnEvent(t, &sender1, txID1, 0, 0, 10, 1),
		makeNFTDepositedEvent(t, &recipient1, txID1, 0, 1, 10, 1),
		makeNFTWithdrawnEvent(t, &sender2, txID2, 1, 0, 20, 2),
		makeNFTDepositedEvent(t, &recipient2, txID2, 1, 1, 20, 2),
	}

	expected := []access.NonFungibleTokenTransfer{
		getNFTTransfer(testBlockHeight, txID1, 0, sender1, recipient1, 1, 0, 1),
		getNFTTransfer(testBlockHeight, txID2, 1, sender2, recipient2, 2, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// ==========================================================================
// addressFromOptional Tests
// ==========================================================================

func TestAddressFromOptional(t *testing.T) {
	t.Run("valid address", func(t *testing.T) {
		expected := unittest.RandomAddressFixture()
		opt := cadence.NewOptional(cadence.NewAddress(expected))
		addr, err := addressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, expected, addr)
	})

	t.Run("nil optional value", func(t *testing.T) {
		opt := cadence.NewOptional(nil)
		addr, err := addressFromOptional(opt)
		require.NoError(t, err)
		assert.Equal(t, flow.Address{}, addr)
	})

	t.Run("non-address value in optional returns error", func(t *testing.T) {
		opt := cadence.NewOptional(cadence.String("not an address"))
		_, err := addressFromOptional(opt)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected type")
	})
}

// ==========================================================================
// decodeEvent Tests
// ==========================================================================

func TestDecodeEvent_InvalidPayload(t *testing.T) {
	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: []byte("garbage"),
	}
	_, err := decodeEvent(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode CCF payload")
}

func TestDecodeEvent_NonEventPayload(t *testing.T) {
	// Encode a valid Cadence value that is not an Event.
	payload, err := ccf.Encode(cadence.String("not an event"))
	require.NoError(t, err)

	event := flow.Event{
		Type:    "A.1234.SomeContract.SomeEvent",
		Payload: payload,
	}
	_, err = decodeEvent(event)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not an event")
}

// ==========================================================================
// Test Helpers
// ==========================================================================

func makeFTDepositedEvent(
	t *testing.T,
	toAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	depositedUUID uint64,
	amount cadence.UFix64,
) flow.Event {
	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventType := cadence.NewEventType(
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
		cadence.String(testFTTokenType),
		amount,
		toValue,
		cadence.UInt64(1),
		cadence.UInt64(depositedUUID),
		cadence.UFix64(200_00000000),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             testFTDepositedType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

func makeFTWithdrawnEvent(
	t *testing.T,
	fromAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	fromUUID, withdrawnUUID uint64,
	amount cadence.UFix64,
) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "FungibleToken")
	eventType := cadence.NewEventType(
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
		cadence.String(testFTTokenType),
		amount,
		fromValue,
		cadence.UInt64(fromUUID),
		cadence.UInt64(withdrawnUUID),
		cadence.UFix64(150_00000000),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             testFTWithdrawnType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

func makeNFTDepositedEvent(
	t *testing.T,
	toAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	uuid, nftID uint64,
) flow.Event {
	var toValue cadence.Value
	if toAddr != nil {
		toValue = cadence.NewOptional(cadence.NewAddress(*toAddr))
	} else {
		toValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventType := cadence.NewEventType(
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
		cadence.String(testNFTTokenType),
		cadence.UInt64(nftID),
		cadence.UInt64(uuid),
		toValue,
		cadence.UInt64(5),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             testNFTDepositedType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

func makeNFTWithdrawnEvent(
	t *testing.T,
	fromAddr *flow.Address,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	uuid, nftID uint64,
) flow.Event {
	var fromValue cadence.Value
	if fromAddr != nil {
		fromValue = cadence.NewOptional(cadence.NewAddress(*fromAddr))
	} else {
		fromValue = cadence.NewOptional(nil)
	}

	location := common.NewAddressLocation(nil, common.Address{}, "NonFungibleToken")
	eventType := cadence.NewEventType(
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
		cadence.String(testNFTTokenType),
		cadence.UInt64(nftID),
		cadence.UInt64(uuid),
		fromValue,
		cadence.UInt64(5),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             testNFTWithdrawnType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

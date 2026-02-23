package transfers

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/transfers/testutil"
	"github.com/onflow/flow-go/utils/unittest"
)

const testBlockHeight = uint64(100)

// ==========================================================================
// FT Transfer Tests
// ==========================================================================

func TestParseFTTransfers_EmptyEvents(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)

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
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(42)
	amount := cadence.UFix64(50_00000000)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 1, uuid, amount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 1, 1, uuid, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender, recipient, amount, 0, 1),
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
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(42)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 1, uuid, cadence.UFix64(100_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 1, 1, uuid, cadence.UFix64(40_00000000)),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	require.Empty(t, transfers)
	assert.Contains(t, err.Error(), "not equal to the deposit amount")
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
	parser := NewFTParser(flow.Testnet, false)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	dave := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		// Original withdrawal from Alice's stored vault (UUID=1) → temp vault UUID=50
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		// Sub-withdraw from temp vault 50 → temp vault UUID=51
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(40_00000000)),
		// Sub-withdraw from temp vault 50 → temp vault UUID=52
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 2, 50, 52, cadence.UFix64(25_00000000)),
		// Deposits
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 3, 1, 51, cadence.UFix64(40_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &carol, txID, 0, 4, 1, 52, cadence.UFix64(25_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &dave, txID, 0, 5, 1, 50, cadence.UFix64(35_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(40_00000000), 0, 1, 3),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(25_00000000), 0, 2, 4),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, dave, cadence.UFix64(35_00000000), 0, 5),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_DeepWithdrawalChain verifies chain resolution works for
// chains deeper than one level (A → B → C).
func TestParseFTTransfers_DeepWithdrawalChain(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	alice := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		// Alice's vault (UUID=1) → temp vault 50
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		// temp vault 50 → temp vault 51
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
		// temp vault 51 → temp vault 52
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 2, 51, 52, cadence.UFix64(30_00000000)),
		// Deposit the deepest vault
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 3, 1, 52, cadence.UFix64(30_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Paired: full chain from vault 1→50→51→52 + deposit
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, recipient, cadence.UFix64(30_00000000), 0, 1, 2, 3),
		// Burn: vault 50 remainder (100 - 60 sub-withdrawn to vault 51)
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(40_00000000), 0),
		// Burn: vault 51 remainder (60 - 30 sub-withdrawn to vault 52)
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, flow.Address{}, cadence.UFix64(30_00000000), 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_FullyConsumedParent verifies behavior when a temp vault's
// entire content is re-withdrawn into a child vault. The parent withdrawal is fully
// consumed and produces no burn record, since its remaining amount is 0.
//
// Scenario:
//
//	Withdraw 100 from Alice's stored vault (UUID=1) → temp vault UUID=50 (from=Alice)
//	Withdraw 100 from temp vault 50 → temp vault UUID=51 (from=nil, fromUUID=50) [full amount]
//	Deposit temp vault 51 into Bob
func TestParseFTTransfers_FullyConsumedParent(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(100_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 2, 1, 51, cadence.UFix64(100_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Paired: Alice → Bob via chain 1→50→51; parent vault 50 fully consumed, no burn record.
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(100_00000000), 0, 1, 2),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_FullyConsumedByMultipleChildren verifies behavior when
// multiple children collectively consume a parent's full amount. The parent
// produces no burn record since its remaining amount is 0.
//
// Scenario:
//
//	Withdraw 100 from Alice's stored vault → temp vault UUID=50 (from=Alice)
//	Withdraw 60 from temp vault 50 → UUID=51 (from=nil)
//	Withdraw 40 from temp vault 50 → UUID=52 (from=nil)
//	Deposit 51 into Bob, Deposit 52 into Carol
func TestParseFTTransfers_FullyConsumedByMultipleChildren(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 2, 50, 52, cadence.UFix64(40_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 3, 1, 51, cadence.UFix64(60_00000000)),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &carol, txID, 0, 4, 1, 52, cadence.UFix64(40_00000000)),
	}

	// Parent vault 50 is fully consumed by children 51 and 52, so no burn record is produced.
	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(60_00000000), 0, 1, 3),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(40_00000000), 0, 2, 4),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_UnpairedDeposit verifies that an unpaired deposit is treated as a mint.
func TestParseFTTransfers_UnpairedDeposit(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	amount := cadence.UFix64(100_00000000)
	events := []flow.Event{
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 3, 1, 99, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, amount, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_UnpairedWithdrawal verifies that an unpaired withdrawal is treated as a burn.
func TestParseFTTransfers_UnpairedWithdrawal(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	amount := cadence.UFix64(25_00000000)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 5, 1, 77, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, amount, 5),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_NilOptionalAddresses(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	txID := unittest.IdentifierFixture()

	uuid := uint64(10)
	amount := cadence.UFix64(1_00000000)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 0, 1, uuid, amount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, nil, txID, 0, 1, 1, uuid, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, flow.Address{}, flow.Address{}, amount, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_MultiplePairsInSameTx(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	txID := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	amount1 := cadence.UFix64(10_00000000)
	amount2 := cadence.UFix64(20_00000000)

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender1, txID, 0, 0, 1, 10, amount1),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient1, txID, 0, 1, 1, 10, amount1),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender2, txID, 0, 2, 1, 20, amount2),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient2, txID, 0, 3, 1, 20, amount2),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender1, recipient1, amount1, 0, 1),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender2, recipient2, amount2, 2, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_MixedPairedAndUnpaired(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	txID := unittest.IdentifierFixture()

	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	mintRecipient := unittest.RandomAddressFixture()
	burnSender := unittest.RandomAddressFixture()

	amount := cadence.UFix64(10_00000000)

	events := []flow.Event{
		// Paired transfer
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 1, 100, amount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 1, 1, 100, amount),
		// Unpaired deposit (mint)
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &mintRecipient, txID, 0, 2, 1, 200, amount),
		// Unpaired withdrawal (burn)
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &burnSender, txID, 0, 3, 1, 300, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender, recipient, amount, 0, 1),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, flow.Address{}, mintRecipient, amount, 2),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, burnSender, flow.Address{}, amount, 3),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_EventsAcrossTransactionsDoNotPair verifies that events
// with the same UUID but in different transactions are NOT paired together.
func TestParseFTTransfers_EventsAcrossTransactionsDoNotPair(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sharedUUID := uint64(42)
	amount := cadence.UFix64(10_00000000)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID1, 0, 0, 1, sharedUUID, amount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID2, 1, 0, 1, sharedUUID, amount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID1, 0, sender, flow.Address{}, amount, 0),
		testutil.MakeFTTransfer(testBlockHeight, txID2, 1, flow.Address{}, recipient, amount, 0),
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
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(55)
	amount := cadence.UFix64(30_00000000)

	// Deposit appears before withdrawal in the event list.
	events := []flow.Event{
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient, txID, 0, 0, 1, uuid, amount),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 1, 1, uuid, amount),
	}

	// Deposit without matching withdrawal = mint; withdrawal without matching deposit = burn.
	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, amount, 0),
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, sender, flow.Address{}, amount, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

func TestParseFTTransfers_SkipsIrrelevantEvents(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)

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
	parser := NewFTParser(flow.Testnet, false)

	t.Run("malformed withdrawn event", func(t *testing.T) {
		events := []flow.Event{
			{
				Type:    testutil.FTWithdrawnEventType(flow.Testnet),
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
				Type:    testutil.FTDepositedEventType(flow.Testnet),
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
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 1, 1, 50, cadence.UFix64(50_00000000)),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate withdrawal resource UUID")
}

// TestParseFTTransfers_MintDepositedIntoWithdrawalVault verifies that when a minted vault
// is deposited into an intermediate vault that was itself created by a withdrawal, the
// intermediate vault's tracked amount is updated to include the minted tokens. This allows
// subsequent child withdrawals from the intermediate vault to exceed its original withdrawal
// amount without error, as long as they don't exceed the updated tracked amount.
//
// This models the Flow staking rewards distribution pattern, where a small fee withdrawal
// creates an intermediate vault, a large mint is deposited into it, and rewards are then
// distributed to many stakers via child withdrawals from the same vault.
//
// Scenario:
//
//	Withdraw 40 from Alice's stored vault → intermediate vault UUID=50 (tracked amount=40)
//	Mint 100 tokens → new vault UUID=51 (no prior withdrawal)
//	Deposit vault 51 (mint, depositedUUID=51) into vault 50 (toUUID=50) → vault 50 tracked amount=140
//	Withdraw 80 from vault 50 → vault UUID=52; 80 ≤ 140, vault 50 remaining=60
//	Deposit vault 52 into Bob
//	Deposit vault 50 (remaining 60) into Carol
func TestParseFTTransfers_MintDepositedIntoWithdrawalVault(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	carol := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		// Withdraw 40 from Alice's stored vault (fromUUID=1) → temp vault UUID=50
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 50, cadence.UFix64(40_00000000)),
		// Mint 100 tokens (vault UUID=51) and deposit into vault 50 (toUUID=50).
		// depositedUUID=51 has no prior withdrawal → treated as mint, but toUUID=50
		// is a tracked withdrawal vault so vault 50's tracked amount increases to 140.
		testutil.MakeFTDepositedEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(100_00000000)),
		// Withdraw 80 from vault 50 (fromUUID=50) → vault UUID=52; 80 ≤ 140 ✓, vault 50 remaining=60
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 2, 50, 52, cadence.UFix64(80_00000000)),
		// Deposit vault 52 into Bob
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 3, 1, 52, cadence.UFix64(80_00000000)),
		// Deposit vault 50 (remaining 60) into Carol
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &carol, txID, 0, 4, 1, 50, cadence.UFix64(60_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Mint: vault 51 deposited into vault 50 (no sender, no recipient address for temp vault)
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, flow.Address{}, flow.Address{}, cadence.UFix64(100_00000000), 1),
		// Alice → Bob via vault 50→52 chain
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(80_00000000), 0, 2, 3),
		// Alice → Carol: vault 50 remainder
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(60_00000000), 0, 4),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_ChildExceedsParentAmount verifies that a child withdrawal with an
// amount larger than the parent's remaining balance produces an error.
func TestParseFTTransfers_ChildExceedsParentAmount(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender, txID, 0, 0, 1, 50, cadence.UFix64(50_00000000)),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
	}

	_, err := parser.Parse(events, testBlockHeight)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "greater than the remaining parent")
}

// TestParseFTTransfers_MultipleTransactionsInBlock verifies that events from
// different transactions in the same block are grouped and paired independently.
func TestParseFTTransfers_MultipleTransactionsInBlock(t *testing.T) {
	parser := NewFTParser(flow.Testnet, false)
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()

	sender1 := unittest.RandomAddressFixture()
	recipient1 := unittest.RandomAddressFixture()
	sender2 := unittest.RandomAddressFixture()
	recipient2 := unittest.RandomAddressFixture()

	amount1 := cadence.UFix64(10_00000000)
	amount2 := cadence.UFix64(20_00000000)

	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender1, txID1, 0, 0, 1, 10, amount1),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient1, txID1, 0, 1, 1, 10, amount1),
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &sender2, txID2, 1, 0, 1, 20, amount2),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &recipient2, txID2, 1, 1, 1, 20, amount2),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID1, 0, sender1, recipient1, amount1, 0, 1),
		testutil.MakeFTTransfer(testBlockHeight, txID2, 1, sender2, recipient2, amount2, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// TestParseFTTransfers_FlowFees verifies that when omitFlowFees=false, a deposit to
// the FlowFees contract address that is paired with a FlowFees.FeesDeducted event is included
// in the parser output as a regular transfer. When omitFlowFees=true, the flow fees transfer is excluded.
func TestParseFTTransfers_FlowFees(t *testing.T) {
	payer := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()
	flowFeesAddress := testutil.FlowFeesAddress(flow.Testnet)

	feeAmount := cadence.UFix64(1_00000000)
	events := []flow.Event{
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &payer, txID, 0, 2, 1, 50, feeAmount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &flowFeesAddress, txID, 0, 3, 1, 50, feeAmount),
		testutil.MakeFlowFeesEvent(t, flow.Testnet, txID, 0, 2, feeAmount),
	}

	t.Run("flow fees included", func(t *testing.T) {
		parser := NewFTParser(flow.Testnet, false) // omitFlowFees = false

		expected := []access.FungibleTokenTransfer{
			testutil.MakeFTTransfer(testBlockHeight, txID, 0, payer, flowFeesAddress, feeAmount, 2, 3),
		}

		transfers, err := parser.Parse(events, testBlockHeight)
		require.NoError(t, err)
		assert.ElementsMatch(t, expected, transfers)
	})

	t.Run("flow fees omitted", func(t *testing.T) {
		parser := NewFTParser(flow.Testnet, true) // omitFlowFees = true

		transfers, err := parser.Parse(events, testBlockHeight)
		require.NoError(t, err)
		assert.Empty(t, transfers)
	})

	t.Run("flow fees omitted but transfer to flow fees address is included", func(t *testing.T) {
		parser := NewFTParser(flow.Testnet, true) // omitFlowFees = true

		// prepend a transfer to the flow fees account in the same amount as the fees deducted.
		// this should be treated as a regular transfer, and the correct events should be ignored.
		events := append([]flow.Event{
			testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &payer, txID, 0, 0, 1, 49, feeAmount),
			testutil.MakeFTDepositedEvent(t, flow.Testnet, &flowFeesAddress, txID, 0, 1, 1, 49, feeAmount),
		}, events...)

		expected := []access.FungibleTokenTransfer{
			testutil.MakeFTTransfer(testBlockHeight, txID, 0, payer, flowFeesAddress, feeAmount, 0, 1),
		}

		transfers, err := parser.Parse(events, testBlockHeight)
		require.NoError(t, err)
		assert.ElementsMatch(t, expected, transfers)
	})
}

// TestParseFTTransfers_FlowFees_MixedTransfers verifies that when omitFlowFees=true, the
// flow fees transfer is excluded but regular transfers in the same transaction are preserved.
func TestParseFTTransfers_FlowFees_MixedTransfers(t *testing.T) {
	parser := NewFTParser(flow.Testnet, true)
	alice := unittest.RandomAddressFixture()
	bob := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()
	flowFeesAddress := testutil.FlowFeesAddress(flow.Testnet)

	transferAmount := cadence.UFix64(50_00000000)
	feeAmount := cadence.UFix64(1_00000000)

	events := []flow.Event{
		// Regular transfer: alice → bob (fromUUID=1, withdrawnUUID=10)
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 0, 1, 10, transferAmount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &bob, txID, 0, 1, 1, 10, transferAmount),
		// Fee payment: alice → FlowFees contract (fromUUID=1, withdrawnUUID=50)
		testutil.MakeFTWithdrawnEvent(t, flow.Testnet, &alice, txID, 0, 2, 1, 50, feeAmount),
		testutil.MakeFTDepositedEvent(t, flow.Testnet, &flowFeesAddress, txID, 0, 3, 1, 50, feeAmount),
		testutil.MakeFlowFeesEvent(t, flow.Testnet, txID, 0, 4, feeAmount),
	}

	expected := []access.FungibleTokenTransfer{
		testutil.MakeFTTransfer(testBlockHeight, txID, 0, alice, bob, transferAmount, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

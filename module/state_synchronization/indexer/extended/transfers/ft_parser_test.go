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

const testFTTokenType = "A.0000000000000001.FlowToken.Vault"

var (
	testFTDepositedType flow.EventType
	testFTWithdrawnType flow.EventType
	testFlowFeesType    flow.EventType
	testFlowFeesAddress flow.Address
)

func init() {
	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	testFTDepositedType = flow.EventType(fmt.Sprintf(ftDepositedFormat, sc.FungibleToken.Address))
	testFTWithdrawnType = flow.EventType(fmt.Sprintf(ftWithdrawnFormat, sc.FungibleToken.Address))
	testFlowFeesType = flow.EventType(fmt.Sprintf(flowFeesFormat, sc.FlowFees.Address))
	testFlowFeesAddress = sc.FlowFees.Address
}

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
	parser := NewFTParser(flow.Testnet, false)
	sender := unittest.RandomAddressFixture()
	recipient := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	uuid := uint64(42)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &sender, txID, 0, 0, 1, uuid, cadence.UFix64(100_00000000)),
		makeFTDepositedEvent(t, &recipient, txID, 0, 1, uuid, cadence.UFix64(40_00000000)),
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
	parser := NewFTParser(flow.Testnet, false)
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
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(100_00000000)),
		makeFTDepositedEvent(t, &bob, txID, 0, 2, 51, cadence.UFix64(100_00000000)),
	}

	expected := []access.FungibleTokenTransfer{
		// Paired: Alice → Bob via chain 1→50→51; parent vault 50 fully consumed, no burn record.
		getTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(100_00000000), 0, 1, 2),
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
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 50, cadence.UFix64(100_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 1, 50, 51, cadence.UFix64(60_00000000)),
		makeFTWithdrawnEvent(t, nil, txID, 0, 2, 50, 52, cadence.UFix64(40_00000000)),
		makeFTDepositedEvent(t, &bob, txID, 0, 3, 51, cadence.UFix64(60_00000000)),
		makeFTDepositedEvent(t, &carol, txID, 0, 4, 52, cadence.UFix64(40_00000000)),
	}

	// Parent vault 50 is fully consumed by children 51 and 52, so no burn record is produced.
	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, alice, bob, cadence.UFix64(60_00000000), 0, 1, 3),
		getTransfer(testBlockHeight, txID, 0, alice, carol, cadence.UFix64(40_00000000), 0, 2, 4),
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
		makeFTDepositedEvent(t, &recipient, txID, 0, 3, 99, amount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, flow.Address{}, recipient, amount, 3),
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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
	parser := NewFTParser(flow.Testnet, false)
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

// TestParseFTTransfers_FlowFees verifies that when omitFlowFees=false, a deposit to
// the FlowFees contract address that is paired with a FlowFees.FeesDeducted event is included
// in the parser output as a regular transfer. When omitFlowFees=true, the flow fees transfer is excluded.
func TestParseFTTransfers_FlowFees(t *testing.T) {
	payer := unittest.RandomAddressFixture()
	txID := unittest.IdentifierFixture()

	feeAmount := cadence.UFix64(1_00000000)
	events := []flow.Event{
		makeFTWithdrawnEvent(t, &payer, txID, 0, 2, 1, 50, feeAmount),
		makeFTDepositedEvent(t, &testFlowFeesAddress, txID, 0, 3, 50, feeAmount),
		makeFlowFeesEvent(t, txID, 0, 2, feeAmount),
	}

	t.Run("flow fees included", func(t *testing.T) {
		parser := NewFTParser(flow.Testnet, false) // omitFlowFees = false

		expected := []access.FungibleTokenTransfer{
			getTransfer(testBlockHeight, txID, 0, payer, testFlowFeesAddress, feeAmount, 2, 3),
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
			makeFTWithdrawnEvent(t, &payer, txID, 0, 0, 1, 49, feeAmount),
			makeFTDepositedEvent(t, &testFlowFeesAddress, txID, 0, 1, 49, feeAmount),
		}, events...)

		expected := []access.FungibleTokenTransfer{
			getTransfer(testBlockHeight, txID, 0, payer, testFlowFeesAddress, feeAmount, 0, 1),
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

	transferAmount := cadence.UFix64(50_00000000)
	feeAmount := cadence.UFix64(1_00000000)

	events := []flow.Event{
		// Regular transfer: alice → bob (fromUUID=1, withdrawnUUID=10)
		makeFTWithdrawnEvent(t, &alice, txID, 0, 0, 1, 10, transferAmount),
		makeFTDepositedEvent(t, &bob, txID, 0, 1, 10, transferAmount),
		// Fee payment: alice → FlowFees contract (fromUUID=1, withdrawnUUID=50)
		makeFTWithdrawnEvent(t, &alice, txID, 0, 2, 1, 50, feeAmount),
		makeFTDepositedEvent(t, &testFlowFeesAddress, txID, 0, 3, 50, feeAmount),
		makeFlowFeesEvent(t, txID, 0, 4, feeAmount),
	}

	expected := []access.FungibleTokenTransfer{
		getTransfer(testBlockHeight, txID, 0, alice, bob, transferAmount, 0, 1),
	}

	transfers, err := parser.Parse(events, testBlockHeight)
	require.NoError(t, err)
	assert.ElementsMatch(t, expected, transfers)
}

// ==========================================================================
// FT Test Helpers
// ==========================================================================

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

func makeFlowFeesEvent(
	t *testing.T,
	txID flow.Identifier,
	txIndex, eventIndex uint32,
	amount cadence.UFix64,
) flow.Event {
	location := common.NewAddressLocation(nil, common.Address{}, "FlowFees")
	eventType := cadence.NewEventType(
		location,
		"FlowFees.FeesDeducted",
		[]cadence.Field{
			{Identifier: "amount", Type: cadence.UFix64Type},
			{Identifier: "executionEffort", Type: cadence.UInt64Type},
			{Identifier: "inclusionEffort", Type: cadence.UInt64Type},
		},
		nil,
	)

	event := cadence.NewEvent([]cadence.Value{
		amount,
		cadence.UInt64(500),
		cadence.UInt64(1000),
	}).WithType(eventType)

	payload, err := ccf.Encode(event)
	require.NoError(t, err)

	return flow.Event{
		Type:             testFlowFeesType,
		TransactionID:    txID,
		TransactionIndex: txIndex,
		EventIndex:       eventIndex,
		Payload:          payload,
	}
}

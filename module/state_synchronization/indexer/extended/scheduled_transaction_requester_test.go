package extended

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	executionmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/state_synchronization/indexer/extended/events"
	"github.com/onflow/flow-go/utils/unittest"
)

const requesterTestHeight = uint64(200)

// TestScheduledTransactionRequester_ExecutedEntry verifies that Fetch correctly applies
// Executed status and transaction ID to a fetched scheduled transaction.
func TestScheduledTransactionRequester_ExecutedEntry(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()
	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	executedTxID := unittest.IdentifierFixture()
	comp := MakeTransactionDataComposite(sc, 5, 1, 1000, 300, 100, owner, "A.abc.Contract.Handler", 99)
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 5),
		requesterTestHeight,
	).Return(MakeJITScriptResponse(t, comp), nil).Once()

	data := &scheduledTransactionData{
		executedEntries: []executedEntry{
			{
				event:         &events.TransactionSchedulerExecutedEvent{ID: 5},
				transactionID: executedTxID,
			},
		},
	}
	txs, err := requester.Fetch(context.Background(), []uint64{5}, requesterTestHeight, data)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, uint64(5), txs[0].ID)
	assert.Equal(t, access.ScheduledTxStatusExecuted, txs[0].Status)
	assert.Equal(t, executedTxID, txs[0].ExecutedTransactionID)
	assert.Equal(t, uint64(99), txs[0].TransactionHandlerUUID)
}

// TestScheduledTransactionRequester_CancelledEntry verifies that Fetch correctly applies
// Cancelled status, transaction ID, and fee fields to a fetched scheduled transaction.
func TestScheduledTransactionRequester_CancelledEntry(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()
	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	cancelTxID := unittest.IdentifierFixture()
	comp := MakeTransactionDataComposite(sc, 7, 2, 2000, 400, 150, owner, "A.def.Contract.Handler", 77)
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 7),
		requesterTestHeight,
	).Return(MakeJITScriptResponse(t, comp), nil).Once()

	data := &scheduledTransactionData{
		canceledEntries: []canceledEntry{
			{
				event:         &events.TransactionSchedulerCanceledEvent{ID: 7, FeesReturned: 50, FeesDeducted: 25},
				transactionID: cancelTxID,
			},
		},
	}
	txs, err := requester.Fetch(context.Background(), []uint64{7}, requesterTestHeight, data)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, uint64(7), txs[0].ID)
	assert.Equal(t, access.ScheduledTxStatusCancelled, txs[0].Status)
	assert.Equal(t, cancelTxID, txs[0].CancelledTransactionID)
	assert.Equal(t, uint64(50), txs[0].FeesReturned)
	assert.Equal(t, uint64(25), txs[0].FeesDeducted)
}

// TestScheduledTransactionRequester_FailedEntry verifies that Fetch correctly applies
// Failed status and transaction ID to a fetched scheduled transaction.
func TestScheduledTransactionRequester_FailedEntry(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()
	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	executorTxID := unittest.IdentifierFixture()
	comp := MakeTransactionDataComposite(sc, 42, 1, 3000, 200, 80, owner, "A.xyz.Contract.Handler", 15)
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 42),
		requesterTestHeight,
	).Return(MakeJITScriptResponse(t, comp), nil).Once()

	data := &scheduledTransactionData{
		failedEntries: []failedEntry{
			{scheduledTxID: 42, transactionID: executorTxID},
		},
	}
	txs, err := requester.Fetch(context.Background(), []uint64{42}, requesterTestHeight, data)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, uint64(42), txs[0].ID)
	assert.Equal(t, access.ScheduledTxStatusFailed, txs[0].Status)
	assert.Equal(t, executorTxID, txs[0].ExecutedTransactionID)
}

// TestScheduledTransactionRequester_NilOptional verifies that when the script returns a nil
// optional for an ID (transaction not found on-chain), Fetch returns an error.
func TestScheduledTransactionRequester_NilOptional(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()
	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	// ID 10 exists on-chain; ID 11 does not (nil optional).
	comp := MakeTransactionDataComposite(sc, 10, 1, 1000, 300, 100, owner, "A.abc.Contract.Handler", 10)
	response := MakeJITScriptResponseWithNils(
		t,
		[]cadence.Composite{comp, comp}, // second entry is a nil optional; value is ignored
		[]bool{false, true},
	)
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 10, 11),
		requesterTestHeight,
	).Return(response, nil).Once()

	data := &scheduledTransactionData{
		executedEntries: []executedEntry{
			{event: &events.TransactionSchedulerExecutedEvent{ID: 10}, transactionID: unittest.IdentifierFixture()},
			{event: &events.TransactionSchedulerExecutedEvent{ID: 11}, transactionID: unittest.IdentifierFixture()},
		},
	}
	_, err := requester.Fetch(context.Background(), []uint64{10, 11}, requesterTestHeight, data)
	require.Error(t, err)
	require.ErrorContains(t, err, "is not found on-chain")
}

// TestScheduledTransactionRequester_ScriptError verifies that an error from the script
// executor is propagated from Fetch.
func TestScheduledTransactionRequester_ScriptError(t *testing.T) {
	t.Parallel()

	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	scriptErr := fmt.Errorf("script execution failed")
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 9),
		requesterTestHeight,
	).Return([]byte(nil), scriptErr).Once()

	data := &scheduledTransactionData{
		canceledEntries: []canceledEntry{
			{event: &events.TransactionSchedulerCanceledEvent{ID: 9}},
		},
	}
	_, err := requester.Fetch(context.Background(), []uint64{9}, requesterTestHeight, data)
	require.Error(t, err)
	require.ErrorIs(t, err, scriptErr)
}

// TestScheduledTransactionRequester_Batching verifies that when more than maxLookupBatchSize
// IDs are requested, multiple script calls are made in batches.
func TestScheduledTransactionRequester_Batching(t *testing.T) {
	t.Parallel()

	sc := systemcontracts.SystemContractsForChain(flow.Testnet)
	owner := unittest.RandomAddressFixture()
	executorMock := executionmock.NewScriptExecutor(t)
	requester := NewScheduledTransactionRequester(executorMock, flow.Testnet)

	// maxLookupBatchSize is 50; use 51 IDs to force 2 batches.
	const totalIDs = 51

	var batch1Composites []cadence.Composite
	for i := range 50 {
		batch1Composites = append(batch1Composites, MakeTransactionDataComposite(sc, uint64(i+1), 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", uint64(i+1)))
	}
	batch2Composite := MakeTransactionDataComposite(sc, 51, 1, 1000, 100, 50, owner, "A.abc.Contract.Handler", 51)

	batch1IDs := make([]uint64, 50)
	for i := range 50 {
		batch1IDs[i] = uint64(i + 1)
	}
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, batch1IDs...),
		requesterTestHeight,
	).Return(MakeJITScriptResponse(t, batch1Composites...), nil).Once()
	executorMock.On("ExecuteAtBlockHeight",
		mock.Anything,
		GetTransactionDataScript(flow.Testnet),
		encodeUInt64Args(t, 51),
		requesterTestHeight,
	).Return(MakeJITScriptResponse(t, batch2Composite), nil).Once()

	lookupIDs := make([]uint64, totalIDs)
	canceledEntries := make([]canceledEntry, totalIDs)
	for i := range totalIDs {
		id := uint64(i + 1)
		lookupIDs[i] = id
		canceledEntries[i] = canceledEntry{
			event:         &events.TransactionSchedulerCanceledEvent{ID: id},
			transactionID: unittest.IdentifierFixture(),
		}
	}

	data := &scheduledTransactionData{canceledEntries: canceledEntries}
	txs, err := requester.Fetch(context.Background(), lookupIDs, requesterTestHeight, data)
	require.NoError(t, err)
	assert.Len(t, txs, totalIDs)
}

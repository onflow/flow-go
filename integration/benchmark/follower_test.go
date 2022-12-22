package benchmark

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	flowsdk "github.com/onflow/flow-go-sdk"
	mockClient "github.com/onflow/flow-go/integration/benchmark/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTxFollower creates new follower and stops it.
func TestTxFollower(t *testing.T) {
	t.Parallel()

	client := mockClient.NewClient(t)

	blockHeight := uint64(2)
	blockID := flowsdk.Identifier{0x1}
	client.On("GetLatestBlockHeader", mock.Anything, mock.Anything).Return(&flowsdk.BlockHeader{ID: blockID, Height: blockHeight}, nil).Once()

	nextBlockHeight := blockHeight + 1
	nextBlockID := flowsdk.Identifier{0x6}
	client.On("GetBlockHeaderByHeight", mock.Anything, mock.Anything).Return(&flowsdk.BlockHeader{ID: nextBlockID, Height: nextBlockHeight}, nil).Once()
	client.On("GetBlockHeaderByHeight", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))

	transactionID := flowsdk.Identifier{0x2}
	client.On("GetTransactionResultsByBlockID", mock.Anything, nextBlockID).Return(
		[]*flowsdk.TransactionResult{
			{
				TransactionID: transactionID,
				Status:        flowsdk.TransactionStatusSealed,
				BlockID:       nextBlockID,
				BlockHeight:   nextBlockHeight,
			},
		}, nil).Once()

	f, err := NewTxFollower(
		context.Background(),
		client,
		WithInteval(100*time.Millisecond),
	)
	require.NoError(t, err)

	// test that follower fetched the latest block header
	require.EqualValues(t, blockHeight, f.Height())
	require.Equal(t, blockID, f.BlockID())

	// test that transactionID is eventually discovered
	unittest.AssertReturnsBefore(t, func() {
		result := <-f.Follow(transactionID)
		require.Equal(t, transactionID, result.TransactionID)
		require.Equal(t, flowsdk.TransactionStatusSealed, result.Status)
	}, 1*time.Second)

	// wait for a transaction that is not in the block should not return
	select {
	case <-f.Follow(flowsdk.Identifier{0x7}):
		require.Fail(t, "should not have received a result")
	default:
	}

	// once transactionID is discovered, block height should be updated
	require.Eventually(t, func() bool {
		return nextBlockHeight == f.Height() && nextBlockID == f.BlockID()
	}, 10*time.Second, 10*time.Millisecond)

	// test that multiple Stops are safe
	f.Stop()
	f.Stop()

	// test that all further Follow calls return immediately
	unittest.AssertReturnsBefore(t, func() { <-f.Follow(transactionID) }, 100*time.Millisecond)
	unittest.AssertReturnsBefore(t, func() { <-f.Follow(flowsdk.Identifier{}) }, 100*time.Millisecond)
}

// TestNopTxFollower creates a new follower and verifies that it does not block.
func TestNopTxFollower(t *testing.T) {
	t.Parallel()

	client := mockClient.NewClient(t)
	client.On("GetLatestBlockHeader", mock.Anything, mock.Anything).Return(&flowsdk.BlockHeader{}, nil).Once()
	client.On("GetBlockByHeight", mock.Anything, mock.Anything).Return(nil, errors.New("not found")).Maybe()

	f, err := NewNopTxFollower(
		context.Background(),
		client,
		WithInteval(1*time.Hour),
	)
	require.NoError(t, err)
	unittest.AssertReturnsBefore(t, func() { <-f.Follow(flowsdk.Identifier{}) }, 1*time.Second)

	// test that multiple Stops are safe
	f.Stop()
	f.Stop()

	// test that all further Follow calls return immediately
	unittest.AssertReturnsBefore(t, func() { <-f.Follow(flowsdk.Identifier{}) }, 1*time.Second)
}

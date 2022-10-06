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

	nextBlockID := flowsdk.Identifier{0x6}
	nextBlockHeight := blockHeight + 1
	client.On("GetLatestBlockHeader", mock.Anything, mock.Anything).Return(&flowsdk.BlockHeader{ID: nextBlockID, Height: nextBlockHeight}, nil)

	collectionID := flowsdk.Identifier{0x3}
	client.On("GetBlockByHeight", mock.Anything, nextBlockHeight).Return(
		&flowsdk.Block{
			BlockHeader: flowsdk.BlockHeader{ID: nextBlockID, Height: nextBlockHeight},
			BlockPayload: flowsdk.BlockPayload{
				CollectionGuarantees: []*flowsdk.CollectionGuarantee{{CollectionID: collectionID}},
			},
		}, nil).Once()

	transactionID := flowsdk.Identifier{0x2}
	client.On("GetCollection", mock.Anything, collectionID).Return(
		&flowsdk.Collection{
			TransactionIDs: []flowsdk.Identifier{{0x98}, transactionID, {0x99}},
		}, nil).Once()

	client.On("GetBlockByHeight", mock.Anything, mock.Anything).Return(nil, errors.New("not found"))

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
	unittest.AssertClosesBefore(t, f.Follow(transactionID), 1*time.Second)
	// test that random transaction is not found
	unittest.AssertNotClosesBefore(t, f.Follow(flowsdk.Identifier{0x7}), 200*time.Millisecond)

	// once transactionID is discovered, block height should be updated
	require.EqualValues(t, nextBlockHeight, f.Height())
	require.Equal(t, nextBlockID, f.BlockID())

	// test that multiple Stops are safe
	f.Stop()
	f.Stop()

	// test that all further Follow calls return immediately
	unittest.AssertClosesBefore(t, f.Follow(transactionID), 1*time.Second)
	unittest.AssertClosesBefore(t, f.Follow(flowsdk.Identifier{}), 1*time.Second)
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
	unittest.AssertClosesBefore(t, f.Follow(flowsdk.Identifier{}), 1*time.Second)

	// test that multiple Stops are safe
	f.Stop()
	f.Stop()

	// test that all further Follow calls return immediately
	unittest.AssertClosesBefore(t, f.Follow(flowsdk.Identifier{}), 1*time.Second)
}

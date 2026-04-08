package verifier

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
	unittestMocks "github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestVerifyConcurrently(t *testing.T) {

	errMock := errors.New("mock error")
	errTwo := errors.New("error 2")
	errFour := errors.New("error 4")

	tests := []struct {
		name        string
		from        uint64
		to          uint64
		nWorker     uint
		errors      map[uint64]error // Map of heights to errors
		expectedErr error
	}{
		{
			name:        "All heights verified successfully",
			from:        1,
			to:          5,
			nWorker:     3,
			errors:      nil,
			expectedErr: nil,
		},
		{
			name:    "Single error at a height",
			from:    1,
			to:      5,
			nWorker: 3,
			errors: map[uint64]error{
				3: errMock,
			},
			expectedErr: errMock,
		},
		{
			name:    "Multiple errors, lowest height returned",
			from:    1,
			to:      5,
			nWorker: 3,
			errors: map[uint64]error{
				2: errTwo,
				4: errFour,
			},
			expectedErr: errTwo,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset mockVerifyHeight for each test
			mockVerifyHeight := func(
				height uint64,
				headers storage.Headers,
				chunkDataPacks storage.ChunkDataPacks,
				results storage.ExecutionResults,
				state protocol.State,
				verifier module.ChunkVerifier,
				stopOnMismatch bool,
			) (BlockVerificationStats, error) {
				if err, ok := tt.errors[height]; ok {
					return BlockVerificationStats{}, err
				}
				return BlockVerificationStats{}, nil
			}

			mockHeaders := storagemock.NewHeaders(t)
			mockChunkDataPacks := storagemock.NewChunkDataPacks(t)
			mockResults := storagemock.NewExecutionResults(t)
			mockState := unittestMocks.NewProtocolState()
			mockVerifier := mockmodule.NewChunkVerifier(t)

			_, err := verifyConcurrently(
				tt.from,
				tt.to,
				tt.nWorker,
				true,
				mockHeaders,
				mockChunkDataPacks,
				mockResults,
				mockState,
				mockVerifier,
				mockVerifyHeight,
			)
			if tt.expectedErr != nil {
				if err == nil || !errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

func TestVerifyConcurrentlyAggregatesStats(t *testing.T) {
	// each height returns different stats, verify they are summed correctly
	statsPerHeight := map[uint64]BlockVerificationStats{
		1: {MatchedChunkCount: 2, MismatchedChunkCount: 0, MatchedTransactionCount: 10, MismatchedTransactionCount: 0},
		2: {MatchedChunkCount: 1, MismatchedChunkCount: 1, MatchedTransactionCount: 5, MismatchedTransactionCount: 3},
		3: {MatchedChunkCount: 0, MismatchedChunkCount: 2, MatchedTransactionCount: 0, MismatchedTransactionCount: 8},
	}

	mockVerifyHeight := func(
		height uint64,
		headers storage.Headers,
		chunkDataPacks storage.ChunkDataPacks,
		results storage.ExecutionResults,
		state protocol.State,
		verifier module.ChunkVerifier,
		stopOnMismatch bool,
	) (BlockVerificationStats, error) {
		return statsPerHeight[height], nil
	}

	totalStats, err := verifyConcurrently(
		1, 3, 2, false,
		storagemock.NewHeaders(t),
		storagemock.NewChunkDataPacks(t),
		storagemock.NewExecutionResults(t),
		unittestMocks.NewProtocolState(),
		mockmodule.NewChunkVerifier(t),
		mockVerifyHeight,
	)
	require.NoError(t, err)

	assert.Equal(t, uint64(3), totalStats.MatchedChunkCount)
	assert.Equal(t, uint64(3), totalStats.MismatchedChunkCount)
	assert.Equal(t, uint64(15), totalStats.MatchedTransactionCount)
	assert.Equal(t, uint64(11), totalStats.MismatchedTransactionCount)
}

// setupVerifyHeightMocks creates mocks for verifyHeight with the given number of chunks.
// Each chunk gets NumberOfTransactions set to txPerChunk.
// The verifier mock is returned unconfigured so the caller can set up per-chunk behavior.
func setupVerifyHeightMocks(
	t *testing.T,
	height uint64,
	numChunks int,
	txPerChunk uint64,
) (
	*storagemock.Headers,
	*storagemock.ChunkDataPacks,
	*storagemock.ExecutionResults,
	*protocolmock.State,
	*mockmodule.ChunkVerifier,
	*flow.ExecutionResult,
) {
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
	blockID := header.ID()

	startState := unittest.StateCommitmentFixture()
	chunks := unittest.ChunkListFixture(uint(numChunks), blockID, startState)
	for _, chunk := range chunks {
		chunk.NumberOfTransactions = txPerChunk
	}

	result := unittest.ExecutionResultFixture()
	result.BlockID = blockID
	result.Chunks = chunks

	headers := storagemock.NewHeaders(t)
	headers.On("ByHeight", height).Return(header, nil)

	results := storagemock.NewExecutionResults(t)
	results.On("ByBlockID", blockID).Return(result, nil)

	chunkDataPacks := storagemock.NewChunkDataPacks(t)
	for _, chunk := range chunks {
		coll := unittest.CollectionFixture(1)
		cdp := unittest.ChunkDataPackFixture(chunk.ID(), unittest.WithChunkDataPackCollection(&coll))
		chunkDataPacks.On("ByChunkID", chunk.ID()).Return(cdp, nil).Maybe()
	}

	mockState := protocolmock.NewState(t)
	snapshot := protocolmock.NewSnapshot(t)
	mockState.On("AtBlockID", blockID).Return(snapshot)

	chunkVerifier := mockmodule.NewChunkVerifier(t)

	return headers, chunkDataPacks, results, mockState, chunkVerifier, result
}

func TestVerifyHeight_AllChunksMatch(t *testing.T) {
	height := uint64(100)
	txPerChunk := uint64(10)
	numChunks := 3

	headers, chunkDataPacks, results, state, chunkVerifier, _ := setupVerifyHeightMocks(t, height, numChunks, txPerChunk)
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, nil)

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, false)
	require.NoError(t, err)

	assert.Equal(t, uint64(numChunks), stats.MatchedChunkCount)
	assert.Equal(t, uint64(0), stats.MismatchedChunkCount)
	assert.Equal(t, uint64(numChunks)*txPerChunk, stats.MatchedTransactionCount)
	assert.Equal(t, uint64(0), stats.MismatchedTransactionCount)
}

func TestVerifyHeight_MismatchWithoutStop(t *testing.T) {
	height := uint64(100)
	txPerChunk := uint64(5)
	numChunks := 3

	headers, chunkDataPacks, results, state, chunkVerifier, _ := setupVerifyHeightMocks(t, height, numChunks, txPerChunk)

	verifyErr := errors.New("chunk mismatch")
	// first call fails, subsequent calls succeed
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, verifyErr).Once()
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, nil)

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, false)
	require.NoError(t, err)

	assert.Equal(t, uint64(2), stats.MatchedChunkCount)
	assert.Equal(t, uint64(1), stats.MismatchedChunkCount)
	assert.Equal(t, uint64(2)*txPerChunk, stats.MatchedTransactionCount)
	assert.Equal(t, txPerChunk, stats.MismatchedTransactionCount)
}

func TestVerifyHeight_MismatchWithStop(t *testing.T) {
	height := uint64(100)
	txPerChunk := uint64(5)
	numChunks := 3

	headers, chunkDataPacks, results, state, chunkVerifier, _ := setupVerifyHeightMocks(t, height, numChunks, txPerChunk)

	verifyErr := errors.New("chunk mismatch")
	// first chunk fails - with stopOnMismatch=true, should return error immediately
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, verifyErr).Once()

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, true)
	require.Error(t, err)
	assert.ErrorIs(t, err, verifyErr)
	assert.Equal(t, BlockVerificationStats{}, stats)
}

func TestVerifyHeight_BlockNotExecuted(t *testing.T) {
	height := uint64(100)
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
	blockID := header.ID()

	headers := storagemock.NewHeaders(t)
	headers.On("ByHeight", height).Return(header, nil)

	results := storagemock.NewExecutionResults(t)
	results.On("ByBlockID", blockID).Return(nil, storage.ErrNotFound)

	state := protocolmock.NewState(t)
	chunkVerifier := mockmodule.NewChunkVerifier(t)
	chunkDataPacks := storagemock.NewChunkDataPacks(t)

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, false)
	require.NoError(t, err)
	assert.Equal(t, BlockVerificationStats{}, stats)
}

func TestVerifyHeight_AllChunksMismatchWithoutStop(t *testing.T) {
	height := uint64(100)
	txPerChunk := uint64(7)
	numChunks := 3

	headers, chunkDataPacks, results, state, chunkVerifier, _ := setupVerifyHeightMocks(t, height, numChunks, txPerChunk)

	verifyErr := errors.New("chunk mismatch")
	// all chunks fail, but stopOnMismatch=false so we continue and count all mismatches
	// this also exercises the system chunk path (last chunk)
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, verifyErr)

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, false)
	require.NoError(t, err)

	assert.Equal(t, uint64(0), stats.MatchedChunkCount)
	assert.Equal(t, uint64(numChunks), stats.MismatchedChunkCount)
	assert.Equal(t, uint64(0), stats.MatchedTransactionCount)
	assert.Equal(t, uint64(numChunks)*txPerChunk, stats.MismatchedTransactionCount)
}

func TestVerifyHeight_VaryingTransactionCounts(t *testing.T) {
	height := uint64(100)
	header := unittest.BlockHeaderFixture(unittest.WithHeaderHeight(height))
	blockID := header.ID()

	startState := unittest.StateCommitmentFixture()
	chunks := unittest.ChunkListFixture(3, blockID, startState)
	// set different tx counts per chunk
	chunks[0].NumberOfTransactions = 10
	chunks[1].NumberOfTransactions = 20
	chunks[2].NumberOfTransactions = 30

	result := unittest.ExecutionResultFixture()
	result.BlockID = blockID
	result.Chunks = chunks

	headers := storagemock.NewHeaders(t)
	headers.On("ByHeight", height).Return(header, nil)

	results := storagemock.NewExecutionResults(t)
	results.On("ByBlockID", blockID).Return(result, nil)

	chunkDataPacks := storagemock.NewChunkDataPacks(t)
	for _, chunk := range chunks {
		coll := unittest.CollectionFixture(1)
		cdp := unittest.ChunkDataPackFixture(chunk.ID(), unittest.WithChunkDataPackCollection(&coll))
		chunkDataPacks.On("ByChunkID", chunk.ID()).Return(cdp, nil).Maybe()
	}

	state := protocolmock.NewState(t)
	snapshot := protocolmock.NewSnapshot(t)
	state.On("AtBlockID", blockID).Return(snapshot)

	verifyErr := errors.New("chunk mismatch")
	chunkVerifier := mockmodule.NewChunkVerifier(t)
	// chunk 0 (10 tx) matches, chunk 1 (20 tx) mismatches, chunk 2 (30 tx) matches
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, nil).Once()
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, verifyErr).Once()
	chunkVerifier.On("Verify", testifymock.Anything).Return(nil, nil).Once()

	stats, err := verifyHeight(height, headers, chunkDataPacks, results, state, chunkVerifier, false)
	require.NoError(t, err)

	assert.Equal(t, uint64(2), stats.MatchedChunkCount)
	assert.Equal(t, uint64(1), stats.MismatchedChunkCount)
	assert.Equal(t, uint64(40), stats.MatchedTransactionCount)    // 10 + 30
	assert.Equal(t, uint64(20), stats.MismatchedTransactionCount) // 20
}

func TestVerifyHeight_HeaderNotFound(t *testing.T) {
	height := uint64(100)

	headers := storagemock.NewHeaders(t)
	headers.On("ByHeight", height).Return(nil, storage.ErrNotFound)

	stats, err := verifyHeight(
		height,
		headers,
		storagemock.NewChunkDataPacks(t),
		storagemock.NewExecutionResults(t),
		protocolmock.NewState(t),
		mockmodule.NewChunkVerifier(t),
		false,
	)
	require.Error(t, err)
	assert.Equal(t, BlockVerificationStats{}, stats)
}

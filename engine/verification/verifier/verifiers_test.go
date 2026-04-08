package verifier

import (
	"errors"
	"testing"

	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/mock"
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

			mockHeaders := mock.NewHeaders(t)
			mockChunkDataPacks := mock.NewChunkDataPacks(t)
			mockResults := mock.NewExecutionResults(t)
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

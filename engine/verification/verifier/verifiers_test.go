package verifier

import (
	"errors"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/module"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/mock"
	unittestMocks "github.com/onflow/flow-go/utils/unittest/mocks"
)

func TestVerifyConcurrently(t *testing.T) {

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
			name:        "Single error at a height",
			from:        1,
			to:          5,
			nWorker:     3,
			errors:      map[uint64]error{3: errors.New("mock error")},
			expectedErr: fmt.Errorf("mock error"),
		},
		{
			name:        "Multiple errors, lowest height returned",
			from:        1,
			to:          5,
			nWorker:     3,
			errors:      map[uint64]error{2: errors.New("error 2"), 4: errors.New("error 4")},
			expectedErr: fmt.Errorf("error 2"),
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
			) error {
				if err, ok := tt.errors[height]; ok {
					return err
				}
				return nil
			}

			mockHeaders := mock.NewHeaders(t)
			mockChunkDataPacks := mock.NewChunkDataPacks(t)
			mockResults := mock.NewExecutionResults(t)
			mockState := unittestMocks.NewProtocolState()
			mockVerifier := mockmodule.NewChunkVerifier(t)

			err := verifyConcurrently(tt.from, tt.to, tt.nWorker, true, mockHeaders, mockChunkDataPacks, mockResults, mockState, mockVerifier, mockVerifyHeight)
			if tt.expectedErr != nil {
				if err == nil || errors.Is(err, tt.expectedErr) {
					t.Fatalf("expected error: %v, got: %v", tt.expectedErr, err)
				}
			} else if err != nil {
				t.Fatalf("expected no error, got: %v", err)
			}
		})
	}
}

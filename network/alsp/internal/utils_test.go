package internal_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/network/alsp/internal"
)

var (
	errExpected   = errors.New("expected error")   // only used for testing
	errUnexpected = errors.New("unexpected error") // only used for testing
)

// TestTryWithRecoveryIfHitError tests TryWithRecoveryIfHitError function.
// It tests the following cases:
// 1. successful execution: no error returned and no recovery needed.
// 2. unexpected error: no recovery needed, but error returned.
// 3. successful execution after recovery: recovery needed and successful execution after recovery.
// 4. unexpected error after recovery: recovery needed, but unexpected error returned.
func TestTryWithRecoveryIfHitError(t *testing.T) {
	tests := []struct {
		name string
		// f returns a function that returns a float64 and an error.
		// For this test, we need f itself to be a function so that it contains closure variables for testing.
		f       func() func() (float64, error)
		r       func()
		want    float64
		wantErr error
	}{
		{
			name: "successful execution",
			f: func() func() (float64, error) {
				return func() (float64, error) {
					return 42, nil
				}
			},
			r:       func() {},
			want:    42,
			wantErr: nil,
		},
		{
			name: "unexpected error",
			f: func() func() (float64, error) {
				return func() (float64, error) {
					return 0, errUnexpected
				}
			},
			r:       func() {},
			want:    0,
			wantErr: fmt.Errorf("failed to run f, unexpected error: %w", errUnexpected),
		},
		{
			name: "successful recovery",
			f: func() func() (float64, error) {
				staticCounter := 0
				return func() (float64, error) {
					if staticCounter == 0 {
						staticCounter++
						return 0, errExpected
					}
					return 42, nil
				}
			},
			r:       func() {},
			want:    42,
			wantErr: nil,
		},
		{
			name: "failed recovery",
			f: func() func() (float64, error) {
				return func() (float64, error) {
					return 0, errExpected
				}
			},
			r:       func() {},
			want:    0,
			wantErr: fmt.Errorf("failed to run f even when try recovery: %w", errExpected),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := internal.TryWithRecoveryIfHitError(errExpected, tt.f(), tt.r)
			if got != tt.want {
				t.Errorf("TryWithRecoveryIfHitError() got = %v, want %v", got, tt.want)
			}
			if (err != nil && tt.wantErr == nil) || // we expect error but got nil
				(err == nil && tt.wantErr != nil) || // or we expect no error but got error
				// or we expect error and got error but the error message is not the same
				(err != nil && tt.wantErr != nil && err.Error() != tt.wantErr.Error()) {
				t.Errorf("TryWithRecoveryIfHitError() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

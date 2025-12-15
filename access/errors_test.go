package access_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/module/irrecoverable"
)

func TestRequireNoError(t *testing.T) {
	t.Parallel()

	t.Run("no error", func(t *testing.T) {
		t.Parallel()

		signalerCtx := irrecoverable.NewMockSignalerContext(t, context.Background())
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireNoError(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("with error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("expected error")

		signalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireNoError(ctx, expectedErr)
		require.NotErrorIs(t, err, expectedErr, "expected error should be overridden and explicitly not wrapped")
		require.Containsf(t, err.Error(), expectedErr.Error(), "expected returned error message to contain original error message")
	})
}

func TestRequireErrorIs(t *testing.T) {
	t.Parallel()

	targetErr := fmt.Errorf("target error")

	t.Run("no error", func(t *testing.T) {
		t.Parallel()

		signalerCtx := irrecoverable.NewMockSignalerContext(t, context.Background())
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireErrorIs(ctx, nil, targetErr)
		require.NoError(t, err)
	})

	t.Run("with expected error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("got err: %w", targetErr)

		signalerCtx := irrecoverable.NewMockSignalerContext(t, context.Background())
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireErrorIs(ctx, expectedErr, targetErr)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("with multiple expected error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("got err: %w", targetErr)

		signalerCtx := irrecoverable.NewMockSignalerContext(t, context.Background())
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireErrorIs(ctx, expectedErr, fmt.Errorf("target error2"), targetErr)
		require.ErrorIs(t, err, expectedErr)
	})

	t.Run("with unexpected error", func(t *testing.T) {
		t.Parallel()

		expectedErr := fmt.Errorf("expected error")

		signalerCtx := irrecoverable.NewMockSignalerContextExpectError(t, context.Background(), expectedErr)
		ctx := irrecoverable.WithSignalerContext(context.Background(), signalerCtx)

		err := access.RequireErrorIs(ctx, expectedErr, targetErr)
		require.NotErrorIs(t, err, expectedErr, "expected error should be overridden and explicitly not wrapped")
		require.Containsf(t, err.Error(), expectedErr.Error(), "expected returned error message to contain original error message")
	})
}

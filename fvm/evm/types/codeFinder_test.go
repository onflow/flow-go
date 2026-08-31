package types_test

import (
	"fmt"
	"testing"

	gethVM "github.com/ethereum/go-ethereum/core/vm"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/stretchr/testify/require"
)

func Test_ExecutionErrorCode(t *testing.T) {

	t.Run("should handle unwrapped errors", func(t *testing.T) {
		// plain unwrapped error
		err := gethVM.ErrMaxCodeSizeExceeded

		errorCode := types.ExecutionErrorCode(err)
		require.Equal(
			t,
			types.ExecutionErrCodeMaxCodeSizeExceeded,
			errorCode,
		)
	})

	t.Run("should handle wrapped errors", func(t *testing.T) {
		// wrapped error as returned by Geth
		err := fmt.Errorf(
			"%w: code size %v limit %v",
			gethVM.ErrMaxCodeSizeExceeded,
			15_000,
			gethParams.MaxCodeSizeAmsterdam,
		)

		errorCode := types.ExecutionErrorCode(err)
		require.Equal(
			t,
			types.ExecutionErrCodeMaxCodeSizeExceeded,
			errorCode,
		)
	})
}

package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/admin"
)

func TestCommandParsing(t *testing.T) {
	cmd := StopAtHeightCommand{}

	t.Run("happy path", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": float64(21), // raw json parses to float64
				"crash":  true,
			},
		}

		err := cmd.Validator(req)
		require.NoError(t, err)

		require.IsType(t, StopAtHeightReq{}, req.ValidatorData)

		parsedReq := req.ValidatorData.(StopAtHeightReq)

		require.Equal(t, uint64(21), parsedReq.height)
		require.Equal(t, true, parsedReq.crash)
	})

	t.Run("empty", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{},
		}

		err := cmd.Validator(req)
		require.Error(t, err)
	})

	t.Run("wrong height type", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": "abc",
			},
		}

		err := cmd.Validator(req)
		require.Error(t, err)
	})

	t.Run("wrong height type", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": "abc",
			},
		}

		err := cmd.Validator(req)
		require.Error(t, err)
	})

	t.Run("negative", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": -12,
			},
		}

		err := cmd.Validator(req)
		require.Error(t, err)
	})

}

func TestCommandsSetsValues(t *testing.T) {

	height := atomic.NewUint64(0)
	crash := atomic.NewBool(false)

	cmd := StopAtHeightCommand{
		height: height,
		crash:  crash,
	}

	req := &admin.CommandRequest{
		ValidatorData: StopAtHeightReq{
			height: 37,
			crash:  true,
		},
	}

	_, err := cmd.Handler(context.TODO(), req)
	require.NoError(t, err)

	require.Equal(t, uint64(37), height.Load())
	require.Equal(t, true, crash.Load())

}

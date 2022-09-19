package execution

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/engine/execution/ingestion"
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

	stopAtHeight := ingestion.NewStopAtHeight()

	cmd := NewStopAtHeightCommand(stopAtHeight)

	req := &admin.CommandRequest{
		ValidatorData: StopAtHeightReq{
			height: 37,
			crash:  true,
		},
	}

	_, err := cmd.Handler(context.TODO(), req)
	require.NoError(t, err)

	set, height, crash := stopAtHeight.Get()

	require.True(t, set)
	require.Equal(t, uint64(37), height)
	require.Equal(t, true, crash)

}

package execution

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
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

		require.True(t, admin.IsInvalidAdminParameterError(err))
	})

	t.Run("wrong height type", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": "abc",
			},
		}

		err := cmd.Validator(req)

		require.True(t, admin.IsInvalidAdminParameterError(err))
	})

	t.Run("wrong height type", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": "abc",
			},
		}

		err := cmd.Validator(req)

		require.True(t, admin.IsInvalidAdminParameterError(err))
	})

	t.Run("negative", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": -12,
			},
		}

		err := cmd.Validator(req)

		require.True(t, admin.IsInvalidAdminParameterError(err))
	})

}

func TestCommandsSetsValues(t *testing.T) {

	stopControl := ingestion.NewStopControl(zerolog.Nop(), false, 0)

	cmd := NewStopAtHeightCommand(stopControl)

	req := &admin.CommandRequest{
		ValidatorData: StopAtHeightReq{
			height: 37,
			crash:  true,
		},
	}

	_, err := cmd.Handler(context.TODO(), req)
	require.NoError(t, err)

	height, crash := stopControl.GetStopHeight()

	require.Equal(t, stopControl.GetState(), ingestion.StopControlSet)
	require.Equal(t, uint64(37), height)
	require.Equal(t, true, crash)
}

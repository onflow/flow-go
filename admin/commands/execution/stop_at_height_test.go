package execution

import (
	"context"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/model/flow"
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

	stopControl := stop.NewStopControl(
		engine.NewUnit(),
		time.Second,
		zerolog.Nop(),
		nil,
		nil,
		nil,
		nil,
		&flow.Header{Height: 1},
		false,
		false,
	)

	cmd := NewStopAtHeightCommand(stopControl)

	req := &admin.CommandRequest{
		ValidatorData: StopAtHeightReq{
			height: 37,
			crash:  true,
		},
	}

	_, err := cmd.Handler(context.TODO(), req)
	require.NoError(t, err)

	s := stopControl.GetStopParameters()

	require.NotNil(t, s)
	require.Equal(t, uint64(37), s.StopBeforeHeight)
	require.Equal(t, true, s.ShouldCrash)
}

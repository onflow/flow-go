package verification

import (
	"context"
	"github.com/onflow/flow-go/engine/verification"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage/mock"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/admin"
)

func TestCommandParsing(t *testing.T) {
	cmd := StopVNAtHeightCommand{}

	t.Run("happy path", func(t *testing.T) {

		req := &admin.CommandRequest{
			Data: map[string]interface{}{
				"height": float64(21), // raw json parses to float64
				"crash":  true,
			},
		}

		err := cmd.Validator(req)
		require.NoError(t, err)

		require.IsType(t, StopVNAtHeightReq{}, req.ValidatorData)

		parsedReq := req.ValidatorData.(StopVNAtHeightReq)

		require.Equal(t, uint64(21), parsedReq.height)
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

	state := new(protocol.State)
	cp := new(mock.ConsumerProgress)
	cp.On("ProcessedIndex").Return(uint64(0), nil)

	stopControl, err := verification.NewStopControl(zerolog.Nop(), state, cp)

	cmd := NewStopVNAtHeightCommand(stopControl)

	req := &admin.CommandRequest{
		ValidatorData: StopVNAtHeightReq{
			height: 37,
		},
	}

	_, err = cmd.Handler(context.TODO(), req)
	require.NoError(t, err)

	height := stopControl.GetStopHeight()
	require.Equal(t, uint64(37), height)
}

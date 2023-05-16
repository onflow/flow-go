package cruisectl

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/irrecoverable"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestStartStop tests that the component can be started and stopped gracefully.
func TestStartStop(t *testing.T) {
	state := mockprotocol.NewState(t)
	ctl, err := NewBlockRateController(unittest.Logger(), DefaultConfig(), state)
	require.NoError(t, err)

	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	ctl.Start(ctx)
	unittest.RequireCloseBefore(t, ctl.Ready(), time.Second, "component did not start")
	cancel()
	unittest.RequireCloseBefore(t, ctl.Done(), time.Second, "component did not stop")
}

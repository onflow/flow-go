package adversary

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

const attackerAddress = "localhost:0"

func TestAttackerStart(t *testing.T) {

	codec := cbor.NewCodec()
	orchestrator := &mockinsecure.AttackOrchestrator{}
	// mocks start up of orchestrator
	orchestrator.On("Start", mock.AnythingOfType("*irrecoverable.signalerCtx")).Return().Once()

	attacker, err := NewAttacker(unittest.Logger(), attackerAddress, codec, orchestrator)
	require.NoError(t, err)

	ctx := context.Background()
	attackCtx, errChan := irrecoverable.WithSignaler(ctx)

	go func() {
		select {
		case err := <-errChan:
			t.Error("attacker startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	attacker.Start(attackCtx)
	<-attacker.Ready()
}

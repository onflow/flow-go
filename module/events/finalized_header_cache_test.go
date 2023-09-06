package events

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	protocolmock "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestFinalizedHeaderCache validates that the FinalizedHeaderCache can be constructed
// with an initial value, and updated with events through the FinalizationActor.
func TestFinalizedHeaderCache(t *testing.T) {
	final := unittest.BlockHeaderFixture()

	state := protocolmock.NewState(t)
	snap := protocolmock.NewSnapshot(t)
	state.On("Final").Return(snap)
	snap.On("Head").Return(
		func() *flow.Header { return final },
		func() error { return nil })

	cache, worker, err := NewFinalizedHeaderCache(state)
	require.NoError(t, err)

	// cache should be initialized
	assert.Equal(t, final, cache.Get())

	ctx, cancel := irrecoverable.NewMockSignalerContextWithCancel(t, context.Background())
	defer cancel()
	go worker(ctx, func() {})

	// change the latest finalized block and mock a BlockFinalized event
	final = unittest.BlockHeaderFixture(
		unittest.HeaderWithView(final.View+1),
		unittest.WithHeaderHeight(final.Height+1))
	cache.OnFinalizedBlock(model.BlockFromFlow(final))

	// the cache should be updated
	assert.Eventually(t, func() bool {
		return final.ID() == cache.Get().ID()
	}, time.Second, time.Millisecond*10)
}

package stop

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// RunWithStopControl is a helper function that creates a StopControl instance and runs the provided test function with it.
//
// Parameters:
//   - t: The testing context.
//   - f: A function that takes a MockSignalerContext and a StopControl, used to run the test logic.
func RunWithStopControl(t *testing.T, f func(ctx *irrecoverable.MockSignalerContext, sc *StopControl)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signalerContext := irrecoverable.NewMockSignalerContext(t, ctx)

	f(signalerContext, createStopControl(t, signalerContext))
}

// createStopControl creates and starts a new StopControl instance.
//
// Parameters:
//   - t: The testing context.
//   - signalerContext: The mock context used to simulate signaler behavior.
//
// Returns:
//   - A pointer to the newly created and started StopControl instance.
func createStopControl(t *testing.T, signalerContext *irrecoverable.MockSignalerContext) *StopControl {
	sc := NewStopControl(zerolog.Nop())
	assert.NotNil(t, sc)

	// Start the StopControl component.
	sc.Start(signalerContext)

	return sc
}

// TestNewStopControl verifies that a new StopControl instance is created correctly and its components are ready.
//
// This test ensures that the StopControl can be initialized and started properly, and that all components are ready
// within a specified time frame.
func TestNewStopControl(t *testing.T) {
	RunWithStopControl(t, func(_ *irrecoverable.MockSignalerContext, sc *StopControl) {
		unittest.RequireComponentsReadyBefore(t, 2*time.Second, sc)
	})
}

// TestStopControl_OnVersionUpdate tests the OnVersionUpdate method of the StopControl.
//
// This test covers two scenarios:
// 1. When a valid version update is received, it checks that the version data is stored correctly.
// 2. When a nil version is provided, it checks that the version data is cleared.
func TestStopControl_OnVersionUpdate(t *testing.T) {
	RunWithStopControl(t, func(_ *irrecoverable.MockSignalerContext, sc *StopControl) {

		// Case 1: Version is updated
		height := uint64(100)
		version := semver.New("1.0.0")

		sc.OnVersionUpdate(height, version)

		// Verify that the version data is correctly stored.
		versionData := sc.versionData.Load()
		assert.NotNil(t, versionData)
		assert.Equal(t, height, versionData.incompatibleBlockHeight)
		assert.Equal(t, "1.0.0", versionData.updatedVersion)

		// Case 2: Version update is deleted (nil version)
		sc.OnVersionUpdate(0, nil)

		// Verify that the version data is cleared.
		versionData = sc.versionData.Load()
		assert.Nil(t, versionData)
	})
}

// TestStopControl_OnProcessedBlock tests the onProcessedBlock method of the StopControl.
//
// This test covers multiple scenarios related to processing block heights:
// 1. Verifying that the processed height is updated correctly.
// 2. Ensuring that a lower processed height cannot overwrite a higher one.
// 3. Testing that the StopControl correctly triggers an irrecoverable error (via Throw) when the incompatible block height is reached.
func TestStopControl_OnProcessedBlock(t *testing.T) {
	RunWithStopControl(t, func(ctx *irrecoverable.MockSignalerContext, sc *StopControl) {
		// Initial block height
		height := uint64(10)

		// Update processed height and verify it's stored correctly.
		sc.updateProcessedHeight(height)
		assert.Equal(t, height, sc.lastProcessedHeight.Value())

		// Attempt to set a lower processed height, which should not be allowed.
		sc.updateProcessedHeight(height - 1)
		assert.Equal(t, height, sc.lastProcessedHeight.Value())

		// Set version metadata with an incompatible height and verify the processed height behavior.
		incompatibleHeight := uint64(13)
		version := semver.New("1.0.0")

		sc.OnVersionUpdate(incompatibleHeight, version)
		height = incompatibleHeight - 2
		sc.updateProcessedHeight(height)
		assert.Equal(t, height, sc.lastProcessedHeight.Value())

		// Prepare to trigger the Throw method when the incompatible block height is processed.
		height = incompatibleHeight - 1

		var wg sync.WaitGroup
		wg.Add(1)

		// Expected error message when the incompatible block height is processed.
		expectedError := fmt.Errorf("processed block at height %d is incompatible with the current node version, please upgrade to version %s starting from block height %d", height, version.String(), incompatibleHeight)

		// Set expectation that the Throw method will be called with the expected error.
		ctx.On("Throw", expectedError).Run(func(args mock.Arguments) { wg.Done() }).Return().Once()

		// Update the processed height to the incompatible height and wait for Throw to be called.
		sc.updateProcessedHeight(height)
		unittest.RequireReturnsBefore(t, wg.Wait, 100*time.Millisecond, "expect for ctx.Throw before timeout")

		// Verify that the processed height and the Throw method call are correct.
		assert.Equal(t, height, sc.lastProcessedHeight.Value())
		ctx.AssertCalled(t, "Throw", expectedError)
	})
}

package corruptible

// This test file covers corruptible network tests that are not ingress or egress specific, including error conditions.

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestEngineClosingChannel evaluates that corruptible network closes the channel whenever the corresponding
// engine of that channel attempts on closing it.
func TestEngineClosingChannel(t *testing.T) {
	corruptibleNetwork, adapter := corruptibleNetworkFixture(t, unittest.Logger())
	channel := channels.TestNetworkChannel

	// on invoking adapter.UnRegisterChannel(channel), it must return a nil, which means
	// that the channel has been unregistered by the adapter successfully.
	adapter.On("UnRegisterChannel", channel).Return(nil).Once()

	err := corruptibleNetwork.EngineClosingChannel(channel)
	require.NoError(t, err)

	// adapter's UnRegisterChannel method must be called once.
	mock.AssertExpectationsForObjects(t, adapter)
}

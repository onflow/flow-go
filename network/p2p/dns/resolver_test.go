package dns

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/mocknetwork"
)

func TestResolver(t *testing.T) {
	basicResolver := mocknetwork.BasicResolver{}
	_, err := NewResolver(metrics.NewNoopCollector(), WithBasicResolver(&basicResolver))
	require.NoError(t, err)

}

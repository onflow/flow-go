package testnet_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

func TestFilter(t *testing.T) {
	t.Run("filters by role", func(t *testing.T) {
		configs := testnet.NewNodeConfigSet(5, flow.RoleAccess)

		// add another role to the set to ensure it is filtered out
		configs = append(configs, testnet.NewNodeConfig(flow.RoleExecution))
		filters := configs.Filter(func(n testnet.NodeConfig) bool { return n.Role == flow.RoleAccess })
		assert.Len(t, filters, 5) // should exclude execution node
		for _, config := range filters {
			assert.Equal(t, flow.RoleAccess, config.Role)
		}
	})

	t.Run("filters by multiple conditions", func(t *testing.T) {
		configs := testnet.NodeConfigs{
			testnet.NewNodeConfig(flow.RoleAccess, testnet.WithDebugImage(true)),
			testnet.NewNodeConfig(flow.RoleExecution, testnet.WithDebugImage(true)),
		}

		filters := configs.Filter(
			func(n testnet.NodeConfig) bool { return n.Role == flow.RoleAccess },
			func(n testnet.NodeConfig) bool {
				return n.Debug
			},
		)

		assert.Len(t, filters, 1) // should exclude execution node
		assert.True(t, filters[0].Debug)
	})

	t.Run("no matching filters", func(t *testing.T) {
		configs := testnet.NewNodeConfigSet(5, flow.RoleConsensus)

		filters := configs.Filter(func(n testnet.NodeConfig) bool { return n.Role == flow.RoleAccess })

		assert.Len(t, filters, 0)
	})
}

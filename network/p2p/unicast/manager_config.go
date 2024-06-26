package unicast

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p"
)

type ManagerConfig struct {
	Logger        zerolog.Logger               `validate:"required"`
	StreamFactory p2p.StreamFactory            `validate:"required"`
	SporkId       flow.Identifier              `validate:"required"`
	Metrics       module.UnicastManagerMetrics `validate:"required"`

	Parameters *netconf.UnicastManager `validate:"required"`

	// UnicastConfigCacheFactory is a factory function to create a new dial config cache.
	UnicastConfigCacheFactory DialConfigCacheFactory `validate:"required"`
}

package factories

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/metrics"
)

type SyncCoreFactory struct {
	log     zerolog.Logger
	conf    chainsync.Config
	metrics module.ChainSyncMetrics
}

func NewSyncCoreFactory(
	log zerolog.Logger,
	conf chainsync.Config,
) (*SyncCoreFactory, error) {
	factory := &SyncCoreFactory{
		log:     log,
		conf:    conf,
		metrics: metrics.NewChainSyncCollector(),
	}
	return factory, nil
}

func (f *SyncCoreFactory) Create() (*chainsync.Core, error) {
	core, err := chainsync.New(f.log, f.conf, f.metrics)
	if err != nil {
		return nil, err
	}
	return core, nil
}

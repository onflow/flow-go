package factories

import (
	"github.com/rs/zerolog"

	syncengine "github.com/onflow/flow-go/engine/collection/synchronization"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	chainsync "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
)

type SyncEngineFactory struct {
	log     zerolog.Logger
	net     network.Network
	me      module.Local
	metrics module.EngineMetrics
	conf    chainsync.Config
}

func NewSyncEngineFactory(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.Network,
	me module.Local,
	conf chainsync.Config,
) (*SyncEngineFactory, error) {

	factory := &SyncEngineFactory{
		log:     log,
		me:      me,
		net:     net,
		metrics: metrics,
		conf:    conf,
	}
	return factory, nil
}

func (f *SyncEngineFactory) Create(
	participants flow.IdentityList,
	state cluster.State,
	blocks storage.ClusterBlocks,
	comp network.Engine,
) (*chainsync.Core, *syncengine.Engine, error) {

	core, err := chainsync.New(f.log, f.conf)
	if err != nil {
		return nil, nil, err
	}
	engine, err := syncengine.New(
		f.log,
		f.metrics,
		f.net,
		f.me,
		participants,
		state,
		blocks,
		comp,
		core,
	)
	return core, engine, err
}

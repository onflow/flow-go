package factories

import (
	"github.com/rs/zerolog"

	syncengine "github.com/dapperlabs/flow-go/engine/collection/synchronization"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	chainsync "github.com/dapperlabs/flow-go/module/synchronization"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/storage"
)

type SyncEngine struct {
	log     zerolog.Logger
	net     module.Network
	me      module.Local
	metrics module.EngineMetrics
	conf    chainsync.Config
}

func NewSyncEngineFactory(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net module.Network,
	me module.Local,
	conf chainsync.Config,
) (*SyncEngine, error) {

	factory := &SyncEngine{
		log:     log,
		me:      me,
		net:     net,
		metrics: metrics,
		conf:    conf,
	}
	return factory, nil
}

func (f *SyncEngine) Create(
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

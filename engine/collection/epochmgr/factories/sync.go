package factories

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/collection"
	syncengine "github.com/onflow/flow-go/engine/collection/synchronization"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/storage"
)

type SyncEngineFactory struct {
	log     zerolog.Logger
	net     network.EngineRegistry
	me      module.Local
	metrics module.EngineMetrics
}

func NewSyncEngineFactory(
	log zerolog.Logger,
	metrics module.EngineMetrics,
	net network.EngineRegistry,
	me module.Local,
) (*SyncEngineFactory, error) {

	factory := &SyncEngineFactory{
		log:     log,
		me:      me,
		net:     net,
		metrics: metrics,
	}
	return factory, nil
}

func (f *SyncEngineFactory) Create(
	participants flow.IdentitySkeletonList,
	state cluster.State,
	blocks storage.ClusterBlocks,
	core *chainsync.Core,
	comp collection.Compliance,
) (*syncengine.Engine, error) {

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
	return engine, err
}

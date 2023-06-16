package access

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/access/rpc"
	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

func NewBackend(
	log zerolog.Logger,
	state protocol.State,
	config rpc.Config,
	collectionRPC accessproto.AccessAPIClient,
	historicalAccessNodes []accessproto.AccessAPIClient,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	executionReceipts storage.ExecutionReceipts,
	executionResults storage.ExecutionResults,
	chainID flow.ChainID,
	accessMetrics module.AccessMetrics,
	collectionGRPCPort uint,
	executionGRPCPort uint,
	retryEnabled bool) (*backend.Backend, error) {

	var cache *lru.Cache
	cacheSize := config.ConnectionPoolSize
	if cacheSize > 0 {
		// TODO: remove this fallback after fixing issues with evictions
		// It was observed that evictions cause connection errors for in flight requests. This works around
		// the issue by forcing hte pool size to be greater than the number of ENs + LNs
		if cacheSize < backend.DefaultConnectionPoolSize {
			log.Warn().Msg("connection pool size below threshold, setting pool size to default value ")
			cacheSize = backend.DefaultConnectionPoolSize
		}
		var err error
		cache, err = lru.NewWithEvict(int(cacheSize), func(_, evictedValue interface{}) {
			store := evictedValue.(*backend.CachedClient)
			store.Close()
			log.Debug().Str("grpc_conn_evicted", store.Address).Msg("closing grpc connection evicted from pool")
			if accessMetrics != nil {
				accessMetrics.ConnectionFromPoolEvicted()
			}
		})
		if err != nil {
			return nil, fmt.Errorf("could not initialize connection pool cache: %w", err)
		}
	}

	connectionFactory := &backend.ConnectionFactoryImpl{
		CollectionGRPCPort:        collectionGRPCPort,
		ExecutionGRPCPort:         executionGRPCPort,
		CollectionNodeGRPCTimeout: config.CollectionClientTimeout,
		ExecutionNodeGRPCTimeout:  config.ExecutionClientTimeout,
		ConnectionsCache:          cache,
		CacheSize:                 cacheSize,
		MaxMsgSize:                config.MaxMsgSize,
		AccessMetrics:             accessMetrics,
		Log:                       log,
	}

	return backend.New(state,
		collectionRPC,
		historicalAccessNodes,
		blocks,
		headers,
		collections,
		transactions,
		executionReceipts,
		executionResults,
		chainID,
		accessMetrics,
		connectionFactory,
		retryEnabled,
		config.MaxHeightRange,
		config.PreferredExecutionNodeIDs,
		config.FixedExecutionNodeIDs,
		log,
		backend.DefaultSnapshotHistoryLimit,
		config.ArchiveAddressList,
	), nil
}

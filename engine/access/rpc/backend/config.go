package backend

import (
	"time"

	"github.com/onflow/flow-go/engine/access/rpc/connection"
)

// Config defines the configurable options for creating Backend
type Config struct {
	ExecutionClientTimeout    time.Duration                   // execution API GRPC client timeout
	CollectionClientTimeout   time.Duration                   // collection API GRPC client timeout
	ConnectionPoolSize        uint                            // size of the cache for storing collection and execution connections
	MaxHeightRange            uint                            // max size of height range requests
	PreferredExecutionNodeIDs []string                        // preferred list of upstream execution node IDs
	FixedExecutionNodeIDs     []string                        // fixed list of execution node IDs to choose from if no node ID can be chosen from the PreferredExecutionNodeIDs
	CircuitBreakerConfig      connection.CircuitBreakerConfig // the configuration for circuit breaker
	ScriptExecutionMode       string                          // the mode in which scripts are executed
	EventQueryMode            string                          // the mode in which events are queried
	TxResultQueryMode         string                          // the mode in which tx results are queried
}

package backend

import (
	"github.com/onflow/flow-go/engine/access/rpc/connection"
)

// Config defines the configurable options for creating Backend
type Config struct {
	AccessConfig              connection.Config               // access API GRPC client config
	ExecutionConfig           connection.Config               // execution API GRPC client config
	CollectionConfig          connection.Config               // collection API GRPC client config
	ConnectionPoolSize        uint                            // size of the cache for storing collection and execution connections
	MaxHeightRange            uint                            // max size of height range requests
	PreferredExecutionNodeIDs []string                        // preferred list of upstream execution node IDs
	FixedExecutionNodeIDs     []string                        // fixed list of execution node IDs to choose from if no node ID can be chosen from the PreferredExecutionNodeIDs
	CircuitBreakerConfig      connection.CircuitBreakerConfig // the configuration for circuit breaker
	ScriptExecutionMode       string                          // the mode in which scripts are executed
	EventQueryMode            string                          // the mode in which events are queried
	TxResultQueryMode         string                          // the mode in which tx results are queried
}

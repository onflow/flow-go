package backend

import (
	"errors"
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
}

type ScriptExecutionMode int

const (
	// ScriptExecutionModeLocalOnly executes scripts and gets accounts using only local storage
	ScriptExecutionModeLocalOnly ScriptExecutionMode = iota + 1

	// ScriptExecutionModeExecutionNodesOnly executes scripts and gets accounts using only
	// execution nodes
	ScriptExecutionModeExecutionNodesOnly

	// ScriptExecutionModeFailover executes scripts and gets accounts using local storage first,
	// then falls back to execution nodes if data is not available for the height or if request
	// failed due to a non-user error.
	ScriptExecutionModeFailover

	// ScriptExecutionModeCompare executes scripts and gets accounts using both local storage and
	// execution nodes and compares the results. The execution node result is always returned.
	ScriptExecutionModeCompare
)

func ParseScriptExecutionMode(s string) (ScriptExecutionMode, error) {
	switch s {
	case ScriptExecutionModeLocalOnly.String():
		return ScriptExecutionModeLocalOnly, nil
	case ScriptExecutionModeExecutionNodesOnly.String():
		return ScriptExecutionModeExecutionNodesOnly, nil
	case ScriptExecutionModeFailover.String():
		return ScriptExecutionModeFailover, nil
	case ScriptExecutionModeCompare.String():
		return ScriptExecutionModeCompare, nil
	default:
		return 0, errors.New("invalid script execution mode")
	}
}

func (m ScriptExecutionMode) String() string {
	switch m {
	case ScriptExecutionModeLocalOnly:
		return "local-only"
	case ScriptExecutionModeExecutionNodesOnly:
		return "execution-nodes-only"
	case ScriptExecutionModeFailover:
		return "failover"
	case ScriptExecutionModeCompare:
		return "compare"
	default:
		return ""
	}
}

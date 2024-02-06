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
	EventQueryMode            string                          // the mode in which events are queried
	TxResultQueryMode         string                          // the mode in which events are queried
}

type IndexQueryMode int

const (
	// IndexQueryModeLocalOnly executes scripts and gets accounts using only local storage
	IndexQueryModeLocalOnly IndexQueryMode = iota + 1

	// IndexQueryModeExecutionNodesOnly executes scripts and gets accounts using only
	// execution nodes
	IndexQueryModeExecutionNodesOnly

	// IndexQueryModeFailover executes scripts and gets accounts using local storage first,
	// then falls back to execution nodes if data is not available for the height or if request
	// failed due to a non-user error.
	IndexQueryModeFailover

	// IndexQueryModeCompare executes scripts and gets accounts using both local storage and
	// execution nodes and compares the results. The execution node result is always returned.
	IndexQueryModeCompare
)

func ParseIndexQueryMode(s string) (IndexQueryMode, error) {
	switch s {
	case IndexQueryModeLocalOnly.String():
		return IndexQueryModeLocalOnly, nil
	case IndexQueryModeExecutionNodesOnly.String():
		return IndexQueryModeExecutionNodesOnly, nil
	case IndexQueryModeFailover.String():
		return IndexQueryModeFailover, nil
	case IndexQueryModeCompare.String():
		return IndexQueryModeCompare, nil
	default:
		return 0, errors.New("invalid script execution mode")
	}
}

func (m IndexQueryMode) String() string {
	switch m {
	case IndexQueryModeLocalOnly:
		return "local-only"
	case IndexQueryModeExecutionNodesOnly:
		return "execution-nodes-only"
	case IndexQueryModeFailover:
		return "failover"
	case IndexQueryModeCompare:
		return "compare"
	default:
		return ""
	}
}

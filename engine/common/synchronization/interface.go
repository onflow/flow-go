package synchronization

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// SyncEngineMetrics is a helper interface which is built around module.EngineMetrics.
// It's useful for RequestHandlerEngine since it can be used to report metrics for different engine names.
type SyncEngineMetrics interface {
	MessageSent(message string)
	MessageReceived(message string)
	MessageHandled(messages string)
}

// RequestHandlerCore is a helper interface which makes possible to implement request handling core for collection and consensus
// clusters using same codebase for message routing using RequestHandlerEngine.
// This interface implements bare minimum functions to handle 3 types of requests.
type RequestHandlerCore interface {
	// HandleSyncRequest handles sync request and returns response, response can be nil, means we should ignore it
	// and omit sending
	HandleSyncRequest(req *messages.SyncRequest, finalized *flow.Header) (interface{}, error)
	// HandleRangeRequest handles range request and returns response, response can be nil, means we should ignore it
	// and omit sending
	HandleRangeRequest(req *messages.RangeRequest, finalized *flow.Header) (interface{}, error)
	// HandleBatchRequest handles batch request and returns response, response can be nil, means we should ignore it
	// and omit sending
	HandleBatchRequest(req *messages.BatchRequest) (interface{}, error)
}

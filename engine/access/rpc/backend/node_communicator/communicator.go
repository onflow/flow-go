package node_communicator

import (
	"github.com/sony/gobreaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
)

// maxFailedRequestCount represents the maximum number of failed requests before returning errors.
const maxFailedRequestCount = 3

type Communicator interface {
	CallAvailableNode(
		// List of node identifiers to execute callback on
		nodes flow.IdentitySkeletonList,

		// Callback function that represents an action to be performed on a node.
		// It takes a node as input and returns an error indicating the result of the action.
		call func(node *flow.IdentitySkeleton) error,

		// Callback function that determines whether an error should terminate further execution.
		// It takes an error as input and returns a boolean value indicating whether the error should be considered terminal.
		shouldTerminateOnError func(node *flow.IdentitySkeleton, err error) bool,
	) (*flow.IdentitySkeleton, error)
}

var _ Communicator = (*NodeCommunicator)(nil)

// NodeCommunicator is responsible for calling available nodes in the backend.
type NodeCommunicator struct {
	nodeSelectorFactory NodeSelectorFactory
}

// NewNodeCommunicator creates a new instance of NodeCommunicator.
func NewNodeCommunicator(circuitBreakerEnabled bool) *NodeCommunicator {
	return &NodeCommunicator{
		nodeSelectorFactory: NodeSelectorFactory{circuitBreakerEnabled: circuitBreakerEnabled},
	}
}

// CallAvailableNode calls the provided `callback` function passing nodes from the provided `nodes`
// list until the function returns without error.
// `nodes` is iterated in order.
// If an error occurs, the provided `shouldTerminateOnError` function is called passing the error. If
// the function returnes true, CallAvailableNode returns immediately without querying any other nodes.
// If `maxFailedRequestCount` is reached, CallAvailableNode returns the accumulated errors.
// Returns the last node passed to `callback`. If the no error is returned, this is the node that
// served the successful request.
func (b *NodeCommunicator) CallAvailableNode(
	nodes flow.IdentitySkeletonList,
	callback func(id *flow.IdentitySkeleton) error,
	shouldTerminateOnError func(node *flow.IdentitySkeleton, err error) bool,
) (*flow.IdentitySkeleton, error) {
	nodeSelector, err := b.nodeSelectorFactory.SelectNodes(nodes)
	if err != nil {
		return nil, err
	}

	var lastNode *flow.IdentitySkeleton

	errs := newMultiStatusError()
	for node := nodeSelector.Next(); node != nil; node = nodeSelector.Next() {
		lastNode = node

		err := callback(node)
		if err == nil {
			return lastNode, nil
		}

		if shouldTerminateOnError != nil && shouldTerminateOnError(node, err) {
			return lastNode, err
		}

		if err == gobreaker.ErrOpenState {
			if !nodeSelector.HasNext() && errs == nil {
				errs.Add(status.Error(codes.Unavailable, "there are no available nodes"))
			}
			continue
		}

		errs.Add(newNodeGrpcError(err, node.Address))
		if len(errs.Errors) >= maxFailedRequestCount {
			return lastNode, errs.ErrorOrNil()
		}
	}

	return lastNode, errs.ErrorOrNil()
}

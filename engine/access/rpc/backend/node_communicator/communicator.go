package node_communicator

import (
	"fmt"
	"maps"

	"github.com/hashicorp/go-multierror"
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

// CallAvailableNode calls the provided function on the available nodes.
// It iterates through the nodes and executes the function.
// If an error occurs, it applies the custom error terminator (if provided) and keeps track of the errors.
// If the error occurs in circuit breaker, it continues to the next node.
// If the maximum failed request count is reached, it returns the accumulated errors.
// Returns the last node used to execute the function. If the no error is returned, this is the node
// that served the successful request.
func (b *NodeCommunicator) CallAvailableNode(
	// List of node identifiers to execute callback on
	nodes flow.IdentitySkeletonList,
	// Callback function that determines whether an error should terminate further execution.
	// It takes an error as input and returns a boolean value indicating whether the error should be considered terminal.
	call func(id *flow.IdentitySkeleton) error,
	// Callback function that determines whether an error should terminate further execution.
	// It takes an error as input and returns a boolean value indicating whether the error should be considered terminal.
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

		err := call(node)
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

type MultiStatusError struct {
	Errors []error
	codes  map[codes.Code]struct{}
}

func newMultiStatusError() *MultiStatusError {
	return &MultiStatusError{
		codes: make(map[codes.Code]struct{}),
	}
}

func (e *MultiStatusError) Add(err error) {
	e.Errors = append(e.Errors, err)

	if nodeErr, ok := err.(nodeGrpcError); ok {
		err = nodeErr.err
	}

	e.codes[status.Code(err)] = struct{}{}
}

func (e *MultiStatusError) Code() codes.Code {
	switch len(e.codes) {
	case 0:
		return codes.OK

	case 1:
		// return the first (only) code
		for code := range e.codes {
			return code
		}

	default:
		codesCopy := make(map[codes.Code]struct{})
		maps.Copy(codesCopy, e.codes)

		remainder := make(map[codes.Code]struct{})
		for code := range e.codes {
			switch code {
			case codes.Unavailable, codes.DeadlineExceeded:
			default:
				remainder[code] = struct{}{}
			}
		}
		if len(remainder) == 1 {
			// return the first (only) code
			for code := range remainder {
				return code
			}
		}
		return codes.Internal
	}

	return codes.Internal
}

func (e *MultiStatusError) Unwrap() []error {
	return e.Errors
}

func (e *MultiStatusError) Error() string {
	return status.Error(e.Code(), multierror.ListFormatFunc(e.Errors)).Error()
}

func (e *MultiStatusError) ErrorOrNil() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e
}

type nodeGrpcError struct {
	err         error
	nodeAddress string
}

func newNodeGrpcError(err error, nodeAddress string) nodeGrpcError {
	return nodeGrpcError{
		err:         err,
		nodeAddress: nodeAddress,
	}
}

func (e nodeGrpcError) Error() string {
	return fmt.Sprintf("%s: %v", e.nodeAddress, e.err)
}

func (e nodeGrpcError) Unwrap() error {
	return e.err
}

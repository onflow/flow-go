package backend

import (
	"github.com/hashicorp/go-multierror"
	"github.com/sony/gobreaker"

	"github.com/onflow/flow-go/model/flow"
)

// maxFailedRequestCount represents the maximum number of failed requests before returning errors.
const maxFailedRequestCount = 3

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

// CallAvailableExecutionNode calls the provided function on the available execution nodes.
// It iterates through the execution nodes and executes the function.
// If an error occurs, it applies the custom error handler (if provided) and keeps track of the errors.
// If the error occurs in circuit breaker, it continues to the next execution node.
// If the maximum failed request count is reached, it returns the accumulated errors.
func (b *NodeCommunicator) CallAvailableExecutionNode(
	nodes flow.IdentityList,
	call func(node *flow.Identity) error,
	customErrorHandler func(err error) bool,
) error {
	var errs *multierror.Error
	execNodeSelector := b.nodeSelectorFactory.SelectExecutionNodes(nodes)

	for execNode := execNodeSelector.Next(); execNode != nil; execNode = execNodeSelector.Next() {
		err := call(execNode)
		if err == nil {
			return nil
		}

		if customErrorHandler != nil && customErrorHandler(err) {
			return err
		}

		if err == gobreaker.ErrOpenState {
			continue
		}

		errs = multierror.Append(errs, err)
		if len(errs.Errors) >= maxFailedRequestCount {
			return errs.ErrorOrNil()
		}
	}

	return errs.ErrorOrNil()
}

// CallAvailableCollectionNode calls the provided function on the available collection nodes.
// It iterates through the collection nodes and executes the function.
// If an error occurs, it keeps track of the errors.
// If the error occurs in circuit breaker, it continues to the next collection node.
// If the maximum failed request count is reached, it returns the accumulated errors.
func (b *NodeCommunicator) CallAvailableCollectionNode(nodes flow.IdentityList, call func(node *flow.Identity) error) error {
	var errs *multierror.Error

	collNodeSelector := b.nodeSelectorFactory.SelectCollectionNodes(nodes)

	for colNode := collNodeSelector.Next(); colNode != nil; colNode = collNodeSelector.Next() {
		err := call(colNode)
		if err == nil {
			return nil
		}

		if err == gobreaker.ErrOpenState {
			continue
		}

		errs = multierror.Append(errs, err)
		if len(errs.Errors) >= maxFailedRequestCount {
			return errs.ErrorOrNil()
		}
	}

	return errs.ErrorOrNil()
}

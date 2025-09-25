package node_communicator

import (
	"fmt"
	"maps"

	"github.com/hashicorp/go-multierror"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MultiStatusError represents a set or gRPC status.Error errors returned by multiple nodes.
// This is produced by the NodeCommunicator.CallAvailableNode function when errors are returned by
// one or more of the nodes queried to complete the request.
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

// nodeGrpcError represents a gRPC error returned by a specific node.
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

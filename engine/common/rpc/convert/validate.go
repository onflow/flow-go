package convert

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
)

// Common validation and conversion for incoming Access API requests

// Address validates the input address and returns a Flow address if valid and error otherwise
func Address(rawAddress []byte, chain flow.Chain) (flow.Address, error) {
	if len(rawAddress) == 0 {
		return flow.EmptyAddress, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	address := flow.BytesToAddress(rawAddress)

	if !chain.IsValid(address) {
		return flow.EmptyAddress, status.Errorf(codes.InvalidArgument, "address %s is invalid for chain %s", address, chain)
	}

	return address, nil
}

func HexToAddress(hexAddress string, chain flow.Chain) (flow.Address, error) {
	if len(hexAddress) == 0 {
		return flow.EmptyAddress, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	address := flow.HexToAddress(hexAddress)

	if !chain.IsValid(address) {
		return flow.EmptyAddress, status.Errorf(codes.InvalidArgument, "address %s is invalid for chain %s", address, chain)
	}

	return address, nil
}

func BlockID(blockID []byte) (flow.Identifier, error) {
	if len(blockID) != flow.IdentifierLen {
		return flow.ZeroID, status.Error(codes.InvalidArgument, "invalid block id")
	}
	return flow.HashToID(blockID), nil
}

func BlockIDs(blockIDs [][]byte) ([]flow.Identifier, error) {
	if len(blockIDs) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty block ids")
	}
	flowBlockIDs := make([]flow.Identifier, len(blockIDs))
	for i, b := range blockIDs {
		flowBlockID, err := BlockID(b)
		if err != nil {
			return nil, err
		}
		flowBlockIDs[i] = flowBlockID
	}
	return flowBlockIDs, nil
}

func CollectionID(collectionID []byte) (flow.Identifier, error) {
	if len(collectionID) == 0 {
		return flow.ZeroID, status.Error(codes.InvalidArgument, "invalid collection id")
	}
	return flow.HashToID(collectionID), nil
}

func EventType(eventType string) (string, error) {
	if len(strings.TrimSpace(eventType)) == 0 {
		return "", status.Error(codes.InvalidArgument, "invalid event type")
	}
	return eventType, nil
}

func TransactionID(txID []byte) (flow.Identifier, error) {
	if len(txID) == 0 {
		return flow.ZeroID, status.Error(codes.InvalidArgument, "invalid transaction id")
	}
	return flow.HashToID(txID), nil
}

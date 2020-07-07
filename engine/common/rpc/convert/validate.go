package convert

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Common validation and conversion for incoming Access API requests

// Address validates the input address and returns a flow address if valid and error otherwise
func Address(rawAddress []byte, chain flow.Chain) (flow.Address, error) {
	if len(rawAddress) == 0 {
		return flow.EmptyAddress, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	address := flow.BytesToAddress(rawAddress)

	if !chain.IsValid(address) {
		return flow.EmptyAddress, status.Error(codes.InvalidArgument, "address is invalid")
	}

	return address, nil
}

func BlockHeight(start, end uint64) error {
	if end < start {
		return status.Error(codes.InvalidArgument, "invalid start or end height")
	}
	return nil
}

func BlockID(blockID []byte) (flow.Identifier, error) {
	if len(blockID) == 0 {
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

func EventType(eventType string) (flow.EventType, error) {
	if eventType == "" {
		return "", status.Error(codes.InvalidArgument, "invalid event type")
	}
	return flow.EventType(eventType), nil
}

func TransactionID(txID []byte) (flow.Identifier, error) {
	if len(txID) == 0 {
		return flow.ZeroID, status.Error(codes.InvalidArgument, "invalid transaction id")
	}
	return flow.HashToID(txID), nil
}

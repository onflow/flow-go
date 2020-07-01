package validate

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func Address(address []byte) error {
	if len(address) == 0 {
		return status.Error(codes.InvalidArgument, fmt.Sprintf("invalid address"))
	}
	return nil
}

func BlockHeight(start, end uint64) error {
	if end < start {
		return status.Error(codes.InvalidArgument, "invalid start or end height")
	}
	return nil
}

func BlockID(blockID []byte) error {
	if len(blockID) == 0 {
		return status.Error(codes.InvalidArgument, "invalid block id")
	}
	return nil
}

func BlockIDs(blockIDs [][]byte) error {
	if len(blockIDs) == 0 {
		return status.Error(codes.InvalidArgument, "empty block ids")
	}
	for _, b := range blockIDs {
		if err := BlockID(b); err != nil {
			return err
		}
	}
	return nil
}

func CollectionID(collectionID []byte) error {
	if len(collectionID) == 0 {
		return status.Error(codes.InvalidArgument, "invalid collection id")
	}
	return nil
}

func EventType(eventType string) error {
	if eventType == "" {
		return status.Error(codes.InvalidArgument, "invalid event type")
	}
	return nil
}

func TransactionID(txID []byte) error {
	if len(txID) == 0 {
		return status.Error(codes.InvalidArgument, "invalid transaction id")
	}
	return nil
}

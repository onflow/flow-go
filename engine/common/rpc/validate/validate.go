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

func BlockIDs(blockIDs [][]byte) error {
	if len(blockIDs) == 0 {
		return status.Error(codes.InvalidArgument, "empty block ids")
	}
	return nil
}

func EventType(eventType string) error {
	if eventType == "" {
		return status.Error(codes.InvalidArgument, "invalid event type")
	}
	return nil
}


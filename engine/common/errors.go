package common

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// InsufficientExecutionReceipts indicates that no execution receipt were found for a given block ID
type InsufficientExecutionReceipts struct {
	blockID      flow.Identifier
	receiptCount int
}

func (e InsufficientExecutionReceipts) Error() string {
	return fmt.Sprintf("insufficient execution receipts found (%d) for block ID: %s", e.receiptCount, e.blockID.String())
}

func ConvertStorageError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, storage.ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}

package routes

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/common"
	commonmodels "github.com/onflow/flow-go/engine/access/rest/common/models"
	"github.com/onflow/flow-go/engine/access/rest/http/request"
)

// GetExecutionReceiptsByBlockID returns all execution receipts for the given block ID.
// The execution_result field is included inline when the caller sets expand=execution_result.
func GetExecutionReceiptsByBlockID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetExecutionReceiptsByBlockIDRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	receipts, err := backend.GetExecutionReceiptsByBlockID(r.Context(), req.BlockID)
	if err != nil {
		return nil, err
	}

	expand := r.ExpandFields
	responses := make([]commonmodels.ExecutionReceipt, len(receipts))
	for i, receipt := range receipts {
		var response commonmodels.ExecutionReceipt
		if err := response.Build(receipt, link, expand); err != nil {
			return nil, err
		}
		responses[i] = response
	}

	return responses, nil
}

// GetExecutionReceiptsByResultID returns all execution receipts for the given execution result ID.
// The execution_result field is included inline when the caller sets expand=execution_result.
func GetExecutionReceiptsByResultID(r *common.Request, backend access.API, link commonmodels.LinkGenerator) (any, error) {
	req, err := request.GetExecutionReceiptsByResultIDRequest(r)
	if err != nil {
		return nil, common.NewBadRequestError(err)
	}

	receipts, err := backend.GetExecutionReceiptsByResultID(r.Context(), req.ID)
	if err != nil {
		return nil, err
	}

	expand := r.ExpandFields
	responses := make([]commonmodels.ExecutionReceipt, len(receipts))
	for i, receipt := range receipts {
		var response commonmodels.ExecutionReceipt
		if err := response.Build(receipt, link, expand); err != nil {
			return nil, err
		}
		responses[i] = response
	}

	return responses, nil
}

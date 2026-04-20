package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/common"
	"github.com/onflow/flow-go/engine/access/rest/common/parser"
	"github.com/onflow/flow-go/model/flow"
)

// GetExecutionReceiptsByBlockID holds the parsed block_id query parameter for
// retrieving execution receipts by block ID.
type GetExecutionReceiptsByBlockID struct {
	BlockID flow.Identifier
}

// GetExecutionReceiptsByBlockIDRequest extracts necessary variables from the provided request,
// builds a GetExecutionReceiptsByBlockID instance, and validates it.
//
// No errors are expected during normal operation.
func GetExecutionReceiptsByBlockIDRequest(r *common.Request) (GetExecutionReceiptsByBlockID, error) {
	var req GetExecutionReceiptsByBlockID
	err := req.Build(r)
	return req, err
}

func (g *GetExecutionReceiptsByBlockID) Build(r *common.Request) error {
	rawID := r.GetQueryParam(blockIDQuery)
	if rawID == "" {
		return fmt.Errorf("block_id query parameter is required")
	}
	var id parser.ID
	if err := id.Parse(rawID); err != nil {
		return err
	}
	g.BlockID = id.Flow()
	return nil
}

// GetExecutionReceiptsByResultID holds the parsed result ID path parameter for
// retrieving execution receipts by execution result ID.
type GetExecutionReceiptsByResultID struct {
	GetByIDRequest
}

// GetExecutionReceiptsByResultIDRequest extracts necessary variables from the provided request,
// builds a GetExecutionReceiptsByResultID instance, and validates it.
//
// No errors are expected during normal operation.
func GetExecutionReceiptsByResultIDRequest(r *common.Request) (GetExecutionReceiptsByResultID, error) {
	var req GetExecutionReceiptsByResultID
	err := req.Build(r)
	return req, err
}

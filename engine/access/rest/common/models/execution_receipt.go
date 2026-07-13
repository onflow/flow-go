package models

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

const ExpandableFieldExecutionResult = "execution_result"

// Build populates an ExecutionReceipt response model from the given domain receipt.
// If expand[ExpandableFieldExecutionResult] is true, the full execution result is inlined;
// otherwise only the link to the execution result is set in the expandable field.
//
// No error returns are expected during normal operation.
func (e *ExecutionReceipt) Build(
	receipt *flow.ExecutionReceipt,
	link LinkGenerator,
	expand map[string]bool,
) error {
	e.ExecutorId = receipt.ExecutorID.String()
	e.ResultId = receipt.ExecutionResult.ID().String()

	spocks := make([]string, len(receipt.Spocks))
	for i, spock := range receipt.Spocks {
		spocks[i] = util.ToBase64(spock)
	}
	e.Spocks = spocks
	e.ExecutorSignature = util.ToBase64(receipt.ExecutorSignature)

	e.Expandable = &ExecutionReceiptExpandable{}
	if expand[ExpandableFieldExecutionResult] {
		var exeResult ExecutionResult
		err := exeResult.Build(&receipt.ExecutionResult, link)
		if err != nil {
			return fmt.Errorf("failed to build execution result: %w", err)
		}
		e.ExecutionResult = &exeResult
	} else {
		resultLink, err := link.ExecutionResultLink(receipt.ExecutionResult.ID())
		if err != nil {
			return fmt.Errorf("failed to build execution result link: %w", err)
		}
		e.Expandable.ExecutionResult = resultLink
	}

	return nil
}

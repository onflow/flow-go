package types

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type ErrorFinalizationFatal struct {
	Msg string
}

func (e *ErrorFinalizationFatal) Error() string { return e.Msg }

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string { return e.Msg }

type ErrorConflictingQCs struct {
	View uint64
	Qcs  []*QuorumCertificate
}

func (e *ErrorConflictingQCs) Error() string {
	return fmt.Sprintf("%d conflicting QCs at view %d", len(e.Qcs), e.View)
}

type ErrorMissingBlock struct {
	View    uint64
	BlockID flow.Identifier
}

func (e *ErrorMissingBlock) Error() string {
	return fmt.Sprintf("missing Block at view %d with ID %v", e.View, e.BlockID)
}

type ErrorInvalidBlock struct {
	View    uint64
	BlockID flow.Identifier
	Msg     string
}

func (e *ErrorInvalidBlock) Error() string {
	return fmt.Sprintf("invalid block (view %d; ID %v): %s", e.View, e.BlockID, e.Msg)
}

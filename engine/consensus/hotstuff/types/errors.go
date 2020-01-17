package types

import "fmt"

type ErrorFinalizationFatal struct {
}

func (e *ErrorFinalizationFatal) Error() string {
	panic("implement me")
}

type ErrorConfiguration struct {
	Msg string
}

func (e *ErrorConfiguration) Error() string {
	return e.Msg
}

type ErrorConflictingQCs struct {
	View uint64
	Qcs []*QuorumCertificate
}
func (e *ErrorConflictingQCs) Error() string {
	return fmt.Sprintf("%d conflicting QCs at view %d", e.View, len(e.Qcs))
}

package irrecoverable

import "fmt"

// RecoverableError is an error that is fatal (i.e. the application should not continue) but is also
// temporary and may be recoverable by restarting a component or taking some other recovery action.
//
// This is intended to be use in conjunction with irrecoverable error handling to differentiate
// errors that can be recovered by an error handler, and those that cannot (e.g. configuration issues)
type RecoverableError struct {
	msg string
	err error
}

func WrapRecoverable(msg string, err error) *RecoverableError {
	return &RecoverableError{msg, err}
}

func (e *RecoverableError) Error() string {
	return fmt.Errorf(e.msg, e.err).Error()
}

func (e *RecoverableError) Temporary() bool {
	return true
}

func (e *RecoverableError) Unwrap() error {
	return e.err
}

package irrecoverable

// RecoverableError is an error that is fatal (i.e. the application should not continue) but is also
// temporary and may be recoverable by restarting a component or taking some other recovery action.
//
// This is intended to be use in in conjunction with irrecoverable error handling to differentiate
// errors that can be recovered by an error handler, and those that cannot (e.g. configuration issues)
type RecoverableError struct {
	err error
}

func NewRecoverableError(err error) *RecoverableError {
	return &RecoverableError{
		err: err,
	}
}

func (e *RecoverableError) Error() string {
	return e.err.Error()
}
func (e *RecoverableError) Unwrap() error {
	return e.err
}

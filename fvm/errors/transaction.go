package errors

import stdErrors "errors"

// TransactionError is an error for transactions
type TransactionError interface {
	// Code returns the code for this error
	Code() uint32
	// and anything else that is needed to be an error
	error
}

func Is(err, target error) bool {
	return stdErrors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return stdErrors.As(err, target)
}

func SplitErrorTypes(err error) (txError TransactionError, vmError VMError) {
	switch err.(type) {
	case VMError:
		return nil, err.(VMError)
	case TransactionError:
		return err.(TransactionError), nil
	default:
		// capture anything else as unknown failures
		return nil, &UnknownFailure{Err: err}
	}
}

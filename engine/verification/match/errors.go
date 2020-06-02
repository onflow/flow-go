package match

import "fmt"

// InvalidInput are the type for input errors. It's useful to distinguish
// errors from exceptions.
// By distinguishing errors from exception using different type, we can log them
// differently. For instance, log InvalidInput error as a warning log, and log
// other error as an error log.
// You can use this struct as an customized error type directly or
// create a function to reuse a certain error type, just like ErrorExecutionResultExist
type InvalidInput struct {
	Msg string
	Err error
}

func NewInvalidInput(msg string) error {
	return InvalidInput{
		Msg: msg,
	}
}

func (e InvalidInput) Unwrap() error {
	return e.Err
}

func (e InvalidInput) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%v, err: %v", e.Msg, e.Err)
	}
	return e.Msg
}

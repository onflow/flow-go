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

type ErrorInvalidTimeout struct {
	Timeout *Timeout
	CurrentView uint64
	CurrentMode TimeoutMode
}

func (e *ErrorInvalidTimeout) Error() string {
	return fmt.Sprintf(
		"received timeout (view, mode) (%d, %s) but current state is (%d, %s)",
		e.Timeout.View, e.Timeout.Mode.String(), e.CurrentView, e.CurrentMode.String(),
		)
}

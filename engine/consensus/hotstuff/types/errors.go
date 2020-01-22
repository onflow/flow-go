package types

type ErrorFinalizationFatal struct {
}

func (e *ErrorFinalizationFatal) Error() string {
	panic("implement me")
}

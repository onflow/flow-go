package types

type ErrorFinalizationFatal struct {
}

func (e *ErrorFinalizationFatal) Error() string {
	panic("implement me")
}

type HotStuffConfigurationError struct {
	msg string
}

func (e *HotStuffConfigurationError) Error() string {
	return e.msg
}

package types

type FinalizationFatalError struct {
}

func (e *FinalizationFatalError) FinalizationFatalError() string {
	panic("implement me")
}

type HotStuffConfigurationError struct {
	msg string
}

func (e *HotStuffConfigurationError) Error() string {
	return e.msg
}

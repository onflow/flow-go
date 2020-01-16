package types

type FinalizationFatalError struct {
}

func (e *FinalizationFatalError) FinalizationFatalError() string {
	panic("implement me")
}

package mocks

type MockCloser struct{}

func (mc *MockCloser) Close() error { return nil }

package errors

// UnreachableError

type UnreachableError struct{}

func (UnreachableError) Error() string {
	return "unreachable"
}

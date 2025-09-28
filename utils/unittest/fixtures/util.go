package fixtures

import "fmt"

// NoError panics if the error is not nil.
func NoError(err error) {
	if err != nil {
		panic(err)
	}
}

// Assert panics if the conditional is false, throwing the provided message.
func Assert(conditional bool, msg string) {
	if !conditional {
		panic(msg)
	}
}

// Assertf panics if the conditional is false, throwing the provided formatted message.
func Assertf(conditional bool, msg string, args ...any) {
	Assert(conditional, fmt.Sprintf(msg, args...))
}

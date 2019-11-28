package errors

import (
	"fmt"
	"runtime/debug"
	"strings"
)

// UnreachableError

// UnreachableError is an internal error in the runtime which should have never occurred
// due to a programming error in the runtime.
//
// NOTE: this error is not used for errors because of bugs in a user-provided program.
// For program errors, see interpreter/errors.go
//
type UnreachableError struct {
	Stack []byte
}

func (e UnreachableError) Error() string {
	return fmt.Sprintf("unreachable\n%s", e.Stack)
}

func NewUnreachableError() *UnreachableError {
	return &UnreachableError{Stack: debug.Stack()}
}

// SecondaryError

// SecondaryError is an interface for errors that provide a secondary error message
//
type SecondaryError interface {
	SecondaryError() string
}

// ErrorNotes is an interface for errors that provide notes
//
type ErrorNotes interface {
	ErrorNotes() []ErrorNote
}

type ErrorNote interface {
	Message() string
}

// ParentError is an error that contains one or more child errors.
type ParentError interface {
	error
	ChildErrors() []error
}

// UnrollChildErrors recursively combines all child errors into a single error message.
func UnrollChildErrors(err error) string {
	var sb strings.Builder
	unrollChildErrors(&sb, 0, err)
	return sb.String()
}

func unrollChildErrors(sb *strings.Builder, level int, err error) {
	var indent = strings.Repeat("    ", level)

	sb.WriteString(indent)
	sb.WriteString(err.Error())

	if err, ok := err.(SecondaryError); ok {
		sb.WriteString(". ")
		sb.WriteString(err.SecondaryError())
	}

	if err, ok := err.(ParentError); ok {
		childErrors := err.ChildErrors()
		if len(childErrors) > 0 {
			sb.WriteString(":")
		}

		for _, childErr := range childErrors {
			sb.WriteString("\n")
			unrollChildErrors(sb, level+1, childErr)
		}
	}
}

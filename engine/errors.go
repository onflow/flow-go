package engine

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
)

// InvalidInputError are errors for caused by invalid inputs.
// It's useful to distinguish these known errors from exceptions.
// By distinguishing errors from exceptions, we can log them
// differently.
// For instance, log InvalidInputError error as a warn log, and log
// other error as an error log.
type InvalidInputError struct {
	err error
}

func NewInvalidInputError(msg string) error {
	return NewInvalidInputErrorf(msg)
}

func NewInvalidInputErrorf(msg string, args ...interface{}) error {
	return InvalidInputError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e InvalidInputError) Unwrap() error {
	return e.err
}

func (e InvalidInputError) Error() string {
	return e.err.Error()
}

// IsInvalidInputError returns whether the given error is an InvalidInputError error
func IsInvalidInputError(err error) bool {
	var errInvalidInputError InvalidInputError
	return errors.As(err, &errInvalidInputError)
}

// OutdatedInputError are for inputs that are outdated. An outdated input doesn't mean
// whether the input was invalid or not, knowing that would take more computation that
// isn't necessary.
// An outdated input could also for a duplicated input: the duplication is outdated.
type OutdatedInputError struct {
	err error
}

func NewOutdatedInputErrorf(msg string, args ...interface{}) error {
	return OutdatedInputError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e OutdatedInputError) Unwrap() error {
	return e.err
}

func (e OutdatedInputError) Error() string {
	return e.err.Error()
}

func IsOutdatedInputError(err error) bool {
	var errOutdatedInputError OutdatedInputError
	return errors.As(err, &errOutdatedInputError)
}

// LogError logs the engine processing error
func LogError(log zerolog.Logger, err error) {
	if IsInvalidInputError(err) {
		// invalid input could be logged as warning.
		// but in non-BFT phase, there should not be any invalid input error.
		// if there is, it must be an exception error, which makes sense
		// to log as error level
		log.Error().
			Str("error_type", "invalid_input").
			Err(err).
			Msg("received invalid input")
	} else if IsOutdatedInputError(err) {
		// outdated input might happen often and not an expection error, so log
		// as warning.
		log.Warn().Err(err).Msg("received outdated input")
	}
}

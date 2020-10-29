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

	msg := "could not process message"

	// Invalid input errors could be logged as warning, because they can be
	// part of normal operations when the network is open and anyone can send
	// weird messages around. However, during the non-BFT phase where we
	// control the majority of the network, we should not be seeing any of
	// them. Logging them as error will help us to identify and address
	// issues with our application logic before going full BFT.
	if IsInvalidInputError(err) {
		log.Error().Str("error_type", "invalid_input").Err(err).Msg(msg)
		return
	}

	// Outdated input errors, on the other hand, can happen regularly, even
	// before opening the network up, as some messages might just be late
	// due to network delays or other infrastructure issues. They should
	// thus be logged as warnings.
	if IsOutdatedInputError(err) {
		log.Warn().Str("error_type", "outdated_input").Err(err).Msg(msg)
		return
	}

	// all other errors should just be logged as usual
	log.Error().Str("error_type", "generic_error").Err(err).Msg(msg)
}

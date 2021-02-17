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

// UnverifiableInputError are for inputs that cannot be verified at this moment.
// Usually it means that we don't have enough data to verify it. A good example is missing
// data in DB to process input.
type UnverifiableInputError struct {
	err error
}

func NewUnverifiableInputError(msg string, args ...interface{}) error {
	return UnverifiableInputError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e UnverifiableInputError) Unwrap() error {
	return e.err
}

func (e UnverifiableInputError) Error() string {
	return e.err.Error()
}

func IsUnverifiableInputError(err error) bool {
	var errUnverifiableInputError UnverifiableInputError
	return errors.As(err, &errUnverifiableInputError)
}

type DuplicatedEntryError struct {
	err error
}

func NewDuplicatedEntryErrorf(msg string, args ...interface{}) error {
	return DuplicatedEntryError{
		err: fmt.Errorf(msg, args...),
	}
}

func (e DuplicatedEntryError) Unwrap() error {
	return e.err
}

func (e DuplicatedEntryError) Error() string {
	return e.err.Error()
}

func IsDuplicatedEntryError(err error) bool {
	var errDuplicatedEntryError DuplicatedEntryError
	return errors.As(err, &errDuplicatedEntryError)
}

// LogError logs the engine processing error
func LogError(log zerolog.Logger, err error) {
	LogErrorWithMsg(log, "could not process message", err)
}

func LogErrorWithMsg(log zerolog.Logger, msg string, err error) {
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
	log.Error().Str("error_type", "internal_error").Err(err).Msg(msg)
}

// Return the error type and whether the error is internal error
func ErrorType(err error) (string, bool) {
	if err == nil {
		return "", false
	}

	if IsInvalidInputError(err) {
		return "invalid_input", false
	}

	if IsOutdatedInputError(err) {
		return "outdated_input", false
	}

	if IsUnverifiableInputError(err) {
		return "unverifiable_input", false
	}

	if IsDuplicatedEntryError(err) {
		return "duplicated_entry", false
	}

	return "internal_error", true
}

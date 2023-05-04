package internal

import (
	"errors"
	"fmt"
)

// TryWithRecoveryIfHitError executes function f and, if an error matching eErr is encountered,
// runs recovery function r and retries executing f. Returns f's result or an error if applicable.
// Args:
//
//	eErr: the expected error.
//	f: the function to execute.
//	r: the recovery function to execute (if running f fails with eErr).
//
// Returns:
//
//	the result of f or an error if applicable.
//
// Note that it returns error if f fails with an error other than eErr or if running f fails after running r with any
// error. Hence, any error returned by this function should be treated as an irrecoverable error.
func TryWithRecoveryIfHitError(eErr error, f func() (float64, error), r func()) (float64, error) {
	// attempts to execute function f
	v, err := f()

	switch {
	// if no error, return the value
	case err == nil:
		return v, nil
	// if error matches the expected error eErr
	case errors.Is(err, eErr):
		// execute the recovery function
		r()

		// retry executing function f
		v, err = f()

		// any error returned by f after running r should be treated as an irrecoverable error.
		if err != nil {
			return 0, fmt.Errorf("failed to run f even when try recovery: %w", err)
		}

		return v, nil
	// if error is unexpected, return the error directly. This should be treated as an irrecoverable error.
	default:
		return 0, fmt.Errorf("failed to run f, unexpected error: %w", err)
	}
}

package irrecoverable_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/module/irrecoverable"
)

// IsIrrecoverableException returns true if and only of the provided error is
// (a wrapped) irrecoverable exception. By protocol convention, irrecoverable
// errors conceptually handled the same way as other unexpected errors: any
// occurrence means that the software has left the pre-defined path of _safe_
// operations. Continuing despite an unexpected error or irrecoverable exception
// is impossible, because protocol-compliant operation of a node can no longer
// be expected. In the worst case the node might be slashed or the protocol as a
// hole compromised.
// For the reason mentioned above, protocol business logic should generally only
// check for sentinel errors expected exactly in the situation the business logic
// is in. Any error that does not match the sentinels known to be benign in that
// situation should be treated as a critical failure and the node must crash.
// Hence, PROTOCOL BUSINESS LOGIC should NEVER CHECK whether an error is an
// IRRECOVERABLE EXCEPTION. This function is for TESTING ONLY.
func IsIrrecoverableException(err error) bool {
	var e = irrecoverable.NewExceptionf("")
	return errors.As(err, &e)
}

var sentinelVar = errors.New("sentinelVar")

type sentinelType struct{}

func (err sentinelType) Error() string { return "sentinel" }

func TestWrapSentinelVar(t *testing.T) {
	// wrapping with Errorf should be unwrappable
	err := fmt.Errorf("some error: %w", sentinelVar)
	assert.ErrorIs(t, err, sentinelVar)

	// wrapping sentinel directly should not be unwrappable
	exception := irrecoverable.NewException(sentinelVar)
	assert.NotErrorIs(t, exception, sentinelVar)

	// wrapping wrapped sentinel should not be unwrappable
	exception = irrecoverable.NewException(err)
	assert.NotErrorIs(t, exception, sentinelVar)
}

func TestWrapSentinelType(t *testing.T) {
	// wrapping with Errorf should be unwrappable
	err := fmt.Errorf("some error: %w", sentinelType{})
	assert.ErrorAs(t, err, &sentinelType{})

	// wrapping sentinel directly should not be unwrappable
	exception := irrecoverable.NewException(sentinelType{})
	assert.False(t, errors.As(exception, &sentinelType{}))

	// wrapping wrapped sentinel should not be unwrappable
	exception = irrecoverable.NewException(err)
	assert.False(t, errors.As(exception, &sentinelType{}))
}

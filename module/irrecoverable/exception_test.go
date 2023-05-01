package irrecoverable

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

var sentinelVar = errors.New("sentinelVar")

type sentinelType struct{}

func (err sentinelType) Error() string { return "sentinel" }

func TestWrapSentinelVar(t *testing.T) {
	// wrapping with Errorf should be unwrappable
	err := fmt.Errorf("some error: %w", sentinelVar)
	assert.ErrorIs(t, err, sentinelVar)

	// wrapping sentinel directly should not be unwrappable
	exception := NewException(sentinelVar)
	assert.NotErrorIs(t, exception, sentinelVar)

	// wrapping wrapped sentinel should not be unwrappable
	exception = NewException(err)
	assert.NotErrorIs(t, exception, sentinelVar)
}

func TestWrapSentinelType(t *testing.T) {
	// wrapping with Errorf should be unwrappable
	err := fmt.Errorf("some error: %w", sentinelType{})
	assert.ErrorAs(t, err, &sentinelType{})

	// wrapping sentinel directly should not be unwrappable
	exception := NewException(sentinelType{})
	assert.False(t, errors.As(exception, &sentinelType{}))

	// wrapping wrapped sentinel should not be unwrappable
	exception = NewException(err)
	assert.False(t, errors.As(exception, &sentinelType{}))
}

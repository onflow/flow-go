package websockets

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestNewSubscriptionID(t *testing.T) {
	t.Run("should generate new ID when input ID is empty", func(t *testing.T) {
		subscriptionID, err := NewSubscriptionID("")

		assert.NoError(t, err)
		assert.NotEmpty(t, subscriptionID.id)
		assert.NoError(t, uuid.Validate(subscriptionID.id), "Generated ID should be a valid UUID")
	})

	t.Run("should return valid SubscriptionID when input ID is valid", func(t *testing.T) {
		validID := "subscription/blocks"
		subscriptionID, err := NewSubscriptionID(validID)

		assert.NoError(t, err)
		assert.Equal(t, validID, subscriptionID.id)
	})

	t.Run("should return an error for invalid input in ParseClientSubscriptionID", func(t *testing.T) {
		longID := fmt.Sprintf("%s%s", "id-", make([]byte, maxLen+1))
		_, err := NewSubscriptionID(longID)

		assert.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf("subscription ID provided by the client must not exceed %d characters", maxLen))
	})
}

func TestParseClientSubscriptionID(t *testing.T) {
	t.Run("should return error if input ID is empty", func(t *testing.T) {
		_, err := ParseClientSubscriptionID("")

		assert.Error(t, err)
		assert.EqualError(t, err, "subscription ID provided by the client must not be empty")
	})

	t.Run("should return error if input ID exceeds max length", func(t *testing.T) {
		longID := fmt.Sprintf("%s%s", "id-", make([]byte, maxLen+1))
		_, err := ParseClientSubscriptionID(longID)

		assert.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf("subscription ID provided by the client must not exceed %d characters", maxLen))
	})

	t.Run("should return valid SubscriptionID for valid input", func(t *testing.T) {
		validID := "subscription/blocks"
		subscription, err := ParseClientSubscriptionID(validID)

		assert.NoError(t, err)
		assert.Equal(t, validID, subscription.id)
	})
}

package validator

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

type TestCase struct {
	Identity   *flow.Identity
	Channel    network.Channel
	Message    interface{}
	MessageStr string
}

var (
	happyPathTestCases = make([]TestCase, 0)
	sadPathTestCases   = make([]TestCase, 0)
)

// TestIsAuthorizedSender_AuthorizedSender checks that IsAuthorizedSender does not return false positive
// validation errors for all possible valid combinations (authorized sender role, message type).
func TestIsAuthorizedSender_AuthorizedSender(t *testing.T) {
	for _, c := range happyPathTestCases {
		str := fmt.Sprintf("role (%s) should be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		t.Run(str, func(t *testing.T) {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			require.NoError(t, err)

			require.Equal(t, c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_UnAuthorizedSender checks that IsAuthorizedSender return's ErrUnauthorizedSender
// validation error for all possible invalid combinations (unauthorized sender role, message type).
func TestIsAuthorizedSender_UnAuthorizedSender(t *testing.T) {
	for _, c := range sadPathTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		t.Run(str, func(t *testing.T) {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			require.ErrorIs(t, err, ErrUnauthorizedSender)
			require.Equal(t, c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_ValidationFailure checks that IsAuthorizedSender returns the expected validation error.
func TestIsAuthorizedSender_ValidationFailure(t *testing.T) {
	t.Run("sender is ejected", func(t *testing.T) {
		identity := unittest.IdentityFixture()
		identity.Ejected = true
		msgType, err := IsAuthorizedSender(identity, network.Channel(""), nil)
		require.ErrorIs(t, err, ErrSenderEjected)
		require.Equal(t, "", msgType)
	})

	t.Run("unknown message type", func(t *testing.T) {
		identity := unittest.IdentityFixture()
		msgType, err := IsAuthorizedSender(identity, network.Channel(""), nil)
		require.ErrorIs(t, err, ErrUnknownMessageType)
		require.Equal(t, "", msgType)
	})
}

// initializeTestCases initializes happy and sad path test cases for checking authorized and unauthorized role message combinations.
func initializeTestCases() {
	for _, c := range network.GetAllMessageAuthConfigs() {
		for channel, authorizedRoles := range c.Config {
			for _, role := range flow.Roles() {
				identity := unittest.IdentityFixture(unittest.WithRole(role))
				if authorizedRoles.Contains(role) {
					// test cases for validation success happy path
					tc := TestCase{
						Identity:   identity,
						Channel:    channel,
						Message:    c.Interface,
						MessageStr: c.String,
					}
					happyPathTestCases = append(happyPathTestCases, tc)
				} else {
					// test cases for validation unsuccessful sad path
					tc := TestCase{
						Identity:   identity,
						Channel:    channel,
						Message:    c.Interface,
						MessageStr: c.String,
					}
					sadPathTestCases = append(sadPathTestCases, tc)
				}
			}
		}
	}
}

func init() {
	initializeTestCases()
}

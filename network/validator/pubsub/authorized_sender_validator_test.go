package validator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestCase struct {
	Identity   *flow.Identity
	Channel    channels.Channel
	Message    interface{}
	MessageStr string
}

func TestIsAuthorizedSender(t *testing.T) {
	suite.Run(t, new(TestIsAuthorizedSenderSuite))
}

type TestIsAuthorizedSenderSuite struct {
	suite.Suite
	authorizedSenderTestCases             []TestCase
	unauthorizedSenderTestCases           []TestCase
	unauthorizedMessageOnChannelTestCases []TestCase
}

func (s *TestIsAuthorizedSenderSuite) SetupTest() {
	s.initializeAuthorizationTestCases()
	s.initializeInvalidMessageOnChannelTestCases()
}

// TestIsAuthorizedSender_AuthorizedSender checks that IsAuthorizedSender does not return false positive
// validation errors for all possible valid combinations (authorized sender role, message type).
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_AuthorizedSender() {
	for _, c := range s.authorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			s.Require().NoError(err)
			s.Require().Equal(c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_UnAuthorizedSender checks that IsAuthorizedSender return's ErrUnauthorizedSender
// validation error for all possible invalid combinations (unauthorized sender role, message type).
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_UnAuthorizedSender() {
	for _, c := range s.unauthorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			s.Require().ErrorIs(err, message.ErrUnauthorizedRole)
			s.Require().Equal(c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_UnAuthorizedSender for each invalid combination of message type and channel
// an appropriate error message.ErrUnauthorizedMessageOnChannel is returned.
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_UnAuthorizedMessageOnChannel() {
	for _, c := range s.unauthorizedMessageOnChannelTestCases {
		str := fmt.Sprintf("message type (%s) should not be authorized to be sent on channel (%s)", c.MessageStr, c.Channel)
		s.Run(str, func() {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			s.Require().ErrorIs(err, message.ErrUnauthorizedMessageOnChannel)
			s.Require().Equal(c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_ClusterPrefixedChannels checks that IsAuthorizedSender correctly
// handles cluster prefixed channels during validation.
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_ClusterPrefixedChannels() {
	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	clusterID := flow.Localnet
	msgType, err := IsAuthorizedSender(identity, channels.ConsensusCluster(clusterID), &messages.ClusterBlockResponse{})
	s.Require().NoError(err)
	s.Require().Equal(message.ClusterBlockResponse, msgType)
}

// TestIsAuthorizedSender_ValidationFailure checks that IsAuthorizedSender returns the expected validation error.
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_ValidationFailure() {
	s.Run("sender is ejected", func() {
		identity := unittest.IdentityFixture()
		identity.Ejected = true
		msgType, err := IsAuthorizedSender(identity, channels.SyncCommittee, &messages.SyncRequest{})
		s.Require().ErrorIs(err, ErrSenderEjected)
		s.Require().Equal("", msgType)
	})

	s.Run("unknown message type", func() {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		type msg struct {
			*messages.BlockProposal
		}

		// *validator.msg is not a known message type, but embeds *messages.BlockProposal which is
		m := &msg{&messages.BlockProposal{
			Header:  nil,
			Payload: nil,
		}}

		msgType, err := IsAuthorizedSender(identity, channels.ConsensusCommittee, m)
		s.Require().ErrorIs(err, ErrUnknownMessageType)
		s.Require().Equal("", msgType)

		msgType, err = IsAuthorizedSender(identity, channels.ConsensusCommittee, nil)
		s.Require().ErrorIs(err, ErrUnknownMessageType)
		s.Require().Equal("", msgType)
	})
}

// initializeAuthorizationTestCases initializes happy and sad path test cases for checking authorized and unauthorized role message combinations.
func (s *TestIsAuthorizedSenderSuite) initializeAuthorizationTestCases() {
	for _, c := range message.AuthorizationConfigs {
		for channel, authorizedRoles := range c.Config {
			for _, role := range flow.Roles() {
				identity := unittest.IdentityFixture(unittest.WithRole(role))
				tc := TestCase{
					Identity:   identity,
					Channel:    channel,
					Message:    c.Type(),
					MessageStr: c.Name,
				}

				if authorizedRoles.Contains(role) {
					// test cases for validation success happy path
					s.authorizedSenderTestCases = append(s.authorizedSenderTestCases, tc)
				} else {
					// test cases for validation unsuccessful sad path
					s.unauthorizedSenderTestCases = append(s.unauthorizedSenderTestCases, tc)
				}
			}
		}
	}
}

// initializeInvalidMessageOnChannelTestCases initializes test cases for all possible combinations of invalid message types on channel.
// NOTE: the role in the test case does not matter since ErrUnauthorizedMessageOnChannel will be returned before the role is checked.
func (s *TestIsAuthorizedSenderSuite) initializeInvalidMessageOnChannelTestCases() {
	// iterate all channels
	for _, c := range message.AuthorizationConfigs {
		for channel, authorizedRoles := range c.Config {
			identity := unittest.IdentityFixture(unittest.WithRole(authorizedRoles[0]))

			// iterate all message types
			for _, config := range message.AuthorizationConfigs {

				// include test if message type is not authorized on channel
				_, ok := config.Config[channel]
				if config.Name != c.Name && !ok {
					tc := TestCase{
						Identity:   identity,
						Channel:    channel,
						Message:    config.Type(),
						MessageStr: config.Name,
					}
					s.unauthorizedMessageOnChannelTestCases = append(s.unauthorizedMessageOnChannelTestCases, tc)
				}
			}
		}
	}
}

package validator

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/messages"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestCase struct {
	Identity   *flow.Identity
	Channel    network.Channel
	Message    interface{}
	MessageStr string
}

func TestIsAuthorizedSender(t *testing.T) {
	suite.Run(t, new(TestIsAuthorizedSenderSuite))
}

type TestIsAuthorizedSenderSuite struct {
	suite.Suite
	happyPathTestCases []TestCase
	sadPathTestCases   []TestCase
}

func (s *TestIsAuthorizedSenderSuite) SetupTest() {
	s.initializeTestCases()
}

// TestIsAuthorizedSender_AuthorizedSender checks that IsAuthorizedSender does not return false positive
// validation errors for all possible valid combinations (authorized sender role, message type).
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_AuthorizedSender() {
	for _, c := range s.happyPathTestCases {
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
	for _, c := range s.sadPathTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			msgType, err := IsAuthorizedSender(c.Identity, c.Channel, c.Message)
			s.Require().ErrorIs(err, ErrUnauthorizedSender)
			s.Require().Equal(c.MessageStr, msgType)
		})
	}
}

// TestIsAuthorizedSender_ValidationFailure checks that IsAuthorizedSender returns the expected validation error.
func (s *TestIsAuthorizedSenderSuite) TestIsAuthorizedSender_ValidationFailure() {
	s.Run("sender is ejected", func() {
		identity := unittest.IdentityFixture()
		identity.Ejected = true
		msgType, err := IsAuthorizedSender(identity, network.Channel(""), nil)
		s.Require().ErrorIs(err, ErrSenderEjected)
		s.Require().Equal("", msgType)
	})

	s.Run("unknown message type", func() {
		identity := unittest.IdentityFixture()
		msgType, err := IsAuthorizedSender(identity, network.Channel(""), nil)
		s.Require().ErrorIs(err, ErrUnknownMessageType)
		s.Require().Equal("", msgType)
	})

	s.Run("unknown message type with message embedded", func() {
		identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
		type msg struct {
			*messages.BlockProposal
		}

		m := &msg{&messages.BlockProposal{
			Header:  nil,
			Payload: nil,
		}}

		msgType, err := IsAuthorizedSender(identity, network.ConsensusCommittee, m)
		s.Require().ErrorIs(err, ErrUnknownMessageType)
		s.Require().Equal("", msgType)
	})
}

// initializeTestCases initializes happy and sad path test cases for checking authorized and unauthorized role message combinations.
func (s *TestIsAuthorizedSenderSuite) initializeTestCases() {
	for _, c := range network.MessageAuthConfigs {
		for channel, authorizedRoles := range c.Config {
			for _, role := range flow.Roles() {
				identity := unittest.IdentityFixture(unittest.WithRole(role))
				tc := TestCase{
					Identity:   identity,
					Channel:    channel,
					Message:    c.Interface(),
					MessageStr: c.String,
				}

				if authorizedRoles.Contains(role) {
					// test cases for validation success happy path
					s.happyPathTestCases = append(s.happyPathTestCases, tc)
				} else {
					// test cases for validation unsuccessful sad path
					s.sadPathTestCases = append(s.sadPathTestCases, tc)
				}
			}
		}
	}
}

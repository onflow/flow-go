package validator

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestCase struct {
	Identity    *flow.Identity
	GetIdentity func(pid peer.ID) (*flow.Identity, bool)
	Channel     channels.Channel
	Message     interface{}
	MessageStr  string
}

func TestIsAuthorizedSender(t *testing.T) {
	suite.Run(t, new(TestAuthorizedSenderValidatorSuite))
}

type TestAuthorizedSenderValidatorSuite struct {
	suite.Suite
	authorizedSenderTestCases             []TestCase
	unauthorizedSenderTestCases           []TestCase
	unauthorizedMessageOnChannelTestCases []TestCase
	log                                   zerolog.Logger
	slashingViolationsConsumer            slashing.ViolationsConsumer
}

func (s *TestAuthorizedSenderValidatorSuite) SetupTest() {
	s.initializeAuthorizationTestCases()
	s.initializeInvalidMessageOnChannelTestCases()
	s.log = unittest.Logger()
	s.slashingViolationsConsumer = slashing.NewSlashingViolationsConsumer(s.log, metrics.NewNoopCollector())
}

// TestValidatorCallback_AuthorizedSender checks that AuthorizedSenderValidator.Validate does not return false positive
// validation errors for all possible valid combinations (authorized sender role, message type).
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_AuthorizedSender() {
	for _, c := range s.authorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			msgType, err := authorizedSenderValidator.Validate(pid, c.Message, c.Channel, true)
			require.NoError(s.T(), err)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)
		})
	}
}

// TestValidatorCallback_UnAuthorizedSender checks that AuthorizedSenderValidator.Validate return's ErrUnauthorizedSender
// validation error for all possible invalid combinations (unauthorized sender role, message type).
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedSender() {
	for _, c := range s.unauthorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			msgType, err := authorizedSenderValidator.Validate(pid, c.Message, c.Channel, true)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedRole)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
		})
	}
}

// TestValidatorCallback_UnAuthorizedMessageOnChannel checks that for each invalid combination of message type and channel
// AuthorizedSenderValidator.Validate returns the appropriate error message.ErrUnauthorizedMessageOnChannel.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedMessageOnChannel() {
	for _, c := range s.unauthorizedMessageOnChannelTestCases {
		str := fmt.Sprintf("message type (%s) should not be authorized to be sent on channel (%s)", c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			msgType, err := authorizedSenderValidator.Validate(pid, c.Message, c.Channel, true)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedMessageOnChannel)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
		})
	}
}

// TestValidatorCallback_ClusterPrefixedChannels checks that AuthorizedSenderValidator.Validate correctly
// handles cluster prefixed channels during validation.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_ClusterPrefixedChannels() {
	identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	clusterID := flow.Localnet

	getIdentityFunc := s.getIdentity(identity)
	pid, err := unittest.PeerIDFromFlowID(identity)
	require.NoError(s.T(), err)

	authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)

	// validate collection consensus cluster
	msgType, err := authorizedSenderValidator.Validate(pid, &messages.ClusterBlockProposal{}, channels.ConsensusCluster(clusterID), true)
	require.NoError(s.T(), err)
	require.Equal(s.T(), message.ClusterBlockProposal, msgType)

	validateCollConsensusPubsub := authorizedSenderValidator.PubSubMessageValidator(channels.ConsensusCluster(clusterID))
	pubsubResult := validateCollConsensusPubsub(pid, &messages.ClusterBlockProposal{})
	require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)

	// validate collection sync cluster
	msgType, err = authorizedSenderValidator.Validate(pid, &messages.SyncRequest{}, channels.SyncCluster(clusterID), true)
	require.NoError(s.T(), err)
	require.Equal(s.T(), message.SyncRequest, msgType)

	validateSyncClusterPubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCluster(clusterID))
	pubsubResult = validateSyncClusterPubsub(pid, &messages.SyncRequest{})
	require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)
}

// TestValidatorCallback_ValidationFailure checks that AuthorizedSenderValidator.Validate returns the expected validation error.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_ValidationFailure() {
	s.Run("sender is ejected", func() {
		identity, _ := unittest.IdentityWithNetworkingKeyFixture()
		identity.Ejected = true
		getIdentityFunc := s.getIdentity(identity)
		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)

		msgType, err := authorizedSenderValidator.Validate(pid, &messages.SyncRequest{}, channels.SyncCommittee, true)
		require.ErrorIs(s.T(), err, ErrSenderEjected)
		require.Equal(s.T(), "", msgType)

		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCommittee)
		pubsubResult := validatePubsub(pid, &messages.SyncRequest{})
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
	})

	s.Run("unknown message type", func() {
		identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))
		type msg struct {
			*messages.BlockProposal
		}

		// *validator.msg is not a known message type, but embeds *messages.BlockProposal which is
		m := &msg{&messages.BlockProposal{
			Header:  nil,
			Payload: nil,
		}}

		getIdentityFunc := s.getIdentity(identity)
		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)
		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.ConsensusCommittee)

		// unknown message types are rejected
		msgType, err := authorizedSenderValidator.Validate(pid, m, channels.ConsensusCommittee, true)
		require.True(s.T(), message.IsUnknownMsgTypeErr(err))
		require.Equal(s.T(), "", msgType)
		pubsubResult := validatePubsub(pid, m)
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)

		// nil messages are rejected
		msgType, err = authorizedSenderValidator.Validate(pid, nil, channels.ConsensusCommittee, true)
		require.True(s.T(), message.IsUnknownMsgTypeErr(err))
		require.Equal(s.T(), "", msgType)
		pubsubResult = validatePubsub(pid, nil)
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
	})

	s.Run("sender is not staked getIdentityFunc does not return identity ", func() {
		identity, _ := unittest.IdentityWithNetworkingKeyFixture()

		// getIdentityFunc simulates unstaked node not found in participant list
		getIdentityFunc := func(id peer.ID) (*flow.Identity, bool) { return nil, false }

		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)

		msgType, err := authorizedSenderValidator.Validate(pid, &messages.SyncRequest{}, channels.SyncCommittee, true)
		require.ErrorIs(s.T(), err, ErrIdentityUnverified)
		require.Equal(s.T(), "", msgType)

		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCommittee)
		pubsubResult := validatePubsub(pid, &messages.SyncRequest{})
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
	})
}

// initializeAuthorizationTestCases initializes happy and sad path test cases for checking authorized and unauthorized role message combinations.
func (s *TestAuthorizedSenderValidatorSuite) initializeAuthorizationTestCases() {
	for _, c := range message.GetAllMessageAuthConfigs() {
		for channel, authorizedRoles := range c.Config {
			for _, role := range flow.Roles() {
				identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(role))
				tc := TestCase{
					Identity:    identity,
					GetIdentity: s.getIdentity(identity),
					Channel:     channel,
					Message:     c.Type(),
					MessageStr:  c.Name,
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
func (s *TestAuthorizedSenderValidatorSuite) initializeInvalidMessageOnChannelTestCases() {
	configs := message.GetAllMessageAuthConfigs()

	// iterate all channels
	for _, c := range configs {
		for channel, authorizedRoles := range c.Config {
			identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(authorizedRoles[0]))

			// iterate all message types
			for _, config := range configs {

				// include test if message type is not authorized on channel
				_, ok := config.Config[channel]
				if config.Name != c.Name && !ok {
					tc := TestCase{
						Identity:    identity,
						GetIdentity: s.getIdentity(identity),
						Channel:     channel,
						Message:     config.Type(),
						MessageStr:  config.Name,
					}
					s.unauthorizedMessageOnChannelTestCases = append(s.unauthorizedMessageOnChannelTestCases, tc)
				}
			}
		}
	}
}

// getIdentity returns a callback that simply returns the provided identity.
func (s *TestAuthorizedSenderValidatorSuite) getIdentity(id *flow.Identity) func(pid peer.ID) (*flow.Identity, bool) {
	return func(pid peer.ID) (*flow.Identity, bool) {
		return id, true
	}
}

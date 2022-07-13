package validator

import (
	"context"
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
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
}

func (s *TestAuthorizedSenderValidatorSuite) SetupTest() {
	s.initializeAuthorizationTestCases()
	s.initializeInvalidMessageOnChannelTestCases()
}

// TestValidatorCallback_AuthorizedSender checks that the call back returned from AuthorizedSenderValidator does not return false positive
// validation errors for all possible valid combinations (authorized sender role, message type).
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_AuthorizedSender() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, c := range s.authorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			validate := AuthorizedSenderValidator(unittest.Logger(), c.Channel, c.GetIdentity)

			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			msgType, err := validate(ctx, pid, c.Message)
			require.NoError(s.T(), err)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), c.Channel, c.GetIdentity)
			pubsubResult := validatePubsub(ctx, pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)
		})
	}
}

// TestValidatorCallback_UnAuthorizedSender checks that the call back returned from AuthorizedSenderValidator return's ErrUnauthorizedSender
// validation error for all possible invalid combinations (unauthorized sender role, message type).
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedSender() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, c := range s.unauthorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			validate := AuthorizedSenderValidator(unittest.Logger(), c.Channel, c.GetIdentity)
			msgType, err := validate(ctx, pid, c.Message)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedRole)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), c.Channel, c.GetIdentity)
			pubsubResult := validatePubsub(ctx, pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
		})
	}
}

// TestValidatorCallback_UnAuthorizedMessageOnChannel for each invalid combination of message type and channel
// the call back returned from AuthorizedSenderValidator returns the appropriate error message.ErrUnauthorizedMessageOnChannel.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedMessageOnChannel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	for _, c := range s.unauthorizedMessageOnChannelTestCases {
		str := fmt.Sprintf("message type (%s) should not be authorized to be sent on channel (%s)", c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			validate := AuthorizedSenderValidator(unittest.Logger(), c.Channel, c.GetIdentity)

			msgType, err := validate(ctx, pid, c.Message)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedMessageOnChannel)
			require.Equal(s.T(), c.MessageStr, msgType)

			validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), c.Channel, c.GetIdentity)
			pubsubResult := validatePubsub(ctx, pid, c.Message)
			require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
		})
	}
}

// TestValidatorCallback_ClusterPrefixedChannels checks that the call back returned from AuthorizedSenderValidator correctly
// handles cluster prefixed channels during validation.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_ClusterPrefixedChannels() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleCollection))
	clusterID := flow.Localnet

	getIdentityFunc := s.getIdentity(identity)
	pid, err := unittest.PeerIDFromFlowID(identity)
	require.NoError(s.T(), err)

	// validate collection consensus cluster
	validateCollConsensus := AuthorizedSenderValidator(unittest.Logger(), channels.ConsensusCluster(clusterID), getIdentityFunc)
	msgType, err := validateCollConsensus(ctx, pid, &messages.ClusterBlockProposal{})
	require.NoError(s.T(), err)
	require.Equal(s.T(), message.ClusterBlockProposal, msgType)

	validateCollConsensusPubsub := AuthorizedSenderMessageValidator(unittest.Logger(), channels.ConsensusCluster(clusterID), getIdentityFunc)
	pubsubResult := validateCollConsensusPubsub(ctx, pid, &messages.ClusterBlockProposal{})
	require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)

	// validate collection sync cluster
	validateSyncCluster := AuthorizedSenderValidator(unittest.Logger(), channels.SyncCluster(clusterID), getIdentityFunc)
	msgType, err = validateSyncCluster(ctx, pid, &messages.SyncRequest{})
	require.NoError(s.T(), err)
	require.Equal(s.T(), message.SyncRequest, msgType)

	validateSyncClusterPubsub := AuthorizedSenderMessageValidator(unittest.Logger(), channels.SyncCluster(clusterID), getIdentityFunc)
	pubsubResult = validateSyncClusterPubsub(ctx, pid, &messages.SyncRequest{})
	require.Equal(s.T(), pubsub.ValidationAccept, pubsubResult)
}

// TestValidatorCallback_ValidationFailure checks that the call back returned from AuthorizedSenderValidator returns the expected validation error.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_ValidationFailure() {
	s.Run("sender is ejected", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		identity, _ := unittest.IdentityWithNetworkingKeyFixture()
		identity.Ejected = true
		getIdentityFunc := s.getIdentity(identity)
		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		validate := AuthorizedSenderValidator(unittest.Logger(), channels.SyncCommittee, getIdentityFunc)
		msgType, err := validate(ctx, pid, &messages.SyncRequest{})
		require.ErrorIs(s.T(), err, ErrSenderEjected)
		require.Equal(s.T(), "", msgType)

		validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), channels.SyncCommittee, getIdentityFunc)
		pubsubResult := validatePubsub(ctx, pid, &messages.SyncRequest{})
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
	})

	s.Run("unknown message type", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

		validate := AuthorizedSenderValidator(unittest.Logger(), channels.ConsensusCommittee, getIdentityFunc)
		validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), channels.ConsensusCommittee, getIdentityFunc)

		// unknown message types are rejected
		msgType, err := validate(ctx, pid, m)
		require.True(s.T(), message.IsUnknownMsgTypeErr(err))
		require.Equal(s.T(), "", msgType)
		pubsubResult := validatePubsub(ctx, pid, m)
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)

		// nil messages are rejected
		msgType, err = validate(ctx, pid, nil)
		require.True(s.T(), message.IsUnknownMsgTypeErr(err))
		require.Equal(s.T(), "", msgType)
		pubsubResult = validatePubsub(ctx, pid, nil)
		require.Equal(s.T(), pubsub.ValidationReject, pubsubResult)
	})

	s.Run("sender is not staked getIdentityFunc does not return identity ", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		identity, _ := unittest.IdentityWithNetworkingKeyFixture()

		// getIdentityFunc simulates unstaked node not found in participant list
		getIdentityFunc := func(id peer.ID) (*flow.Identity, bool) { return nil, false }

		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		validate := AuthorizedSenderValidator(unittest.Logger(), channels.SyncCommittee, getIdentityFunc)
		msgType, err := validate(ctx, pid, &messages.SyncRequest{})
		require.ErrorIs(s.T(), err, ErrIdentityUnverified)
		require.Equal(s.T(), "", msgType)

		validatePubsub := AuthorizedSenderMessageValidator(unittest.Logger(), channels.SyncCommittee, getIdentityFunc)
		pubsubResult := validatePubsub(ctx, pid, &messages.SyncRequest{})
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

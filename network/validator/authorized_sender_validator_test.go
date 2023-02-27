package validator

import (
	"fmt"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/utils/unittest"
)

type TestCase struct {
	Identity    *flow.Identity
	GetIdentity func(pid peer.ID) (*flow.Identity, bool)
	Channel     channels.Channel
	Message     interface{}
	MessageCode codec.MessageCode
	MessageStr  string
	Protocols   message.Protocols
}

func TestIsAuthorizedSender(t *testing.T) {
	suite.Run(t, new(TestAuthorizedSenderValidatorSuite))
}

type TestAuthorizedSenderValidatorSuite struct {
	suite.Suite
	authorizedSenderTestCases             []TestCase
	unauthorizedSenderTestCases           []TestCase
	unauthorizedMessageOnChannelTestCases []TestCase
	unauthorizedUnicastOnChannel          []TestCase
	authorizedUnicastOnChannel            []TestCase
	log                                   zerolog.Logger
	slashingViolationsConsumer            slashing.ViolationsConsumer
	allMsgConfigs                         []message.MsgAuthConfig
	codec                                 network.Codec
}

func (s *TestAuthorizedSenderValidatorSuite) SetupTest() {
	s.allMsgConfigs = message.GetAllMessageAuthConfigs()
	s.initializeAuthorizationTestCases()
	s.initializeInvalidMessageOnChannelTestCases()
	s.initializeUnicastOnChannelTestCases()
	s.log = unittest.Logger()
	s.slashingViolationsConsumer = slashing.NewSlashingViolationsConsumer(s.log, metrics.NewNoopCollector())
	s.codec = unittest.NetworkCodec()
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

			// ensure according to the message auth config, if a message is authorized to be sent via unicast it
			// is accepted or rejected.
			msgType, err := authorizedSenderValidator.Validate(pid, []byte{c.MessageCode.Uint8()}, c.Channel, message.ProtocolTypeUnicast)
			if c.Protocols.Contains(message.ProtocolTypeUnicast) {
				require.NoError(s.T(), err)
				require.Equal(s.T(), c.MessageStr, msgType)
			} else {
				require.ErrorIs(s.T(), err, message.ErrUnauthorizedUnicastOnChannel)
				require.Equal(s.T(), c.MessageStr, msgType)
			}

			payload, err := s.codec.Encode(c.Message)
			require.NoError(s.T(), err)
			m := &message.Message{
				ChannelID: c.Channel.String(),
				Payload:   payload,
			}
			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, m)
			if !c.Protocols.Contains(message.ProtocolTypePubSub) {
				require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
			} else {
				require.Equal(s.T(), p2p.ValidationAccept, pubsubResult)
			}
		})
	}
}

// TestValidatorCallback_UnAuthorizedSender checks that AuthorizedSenderValidator.Validate return's p2p.ValidationReject
// validation error for all possible invalid combinations (unauthorized sender role, message type).
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedSender() {
	for _, c := range s.unauthorizedSenderTestCases {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s)", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			payload, err := s.codec.Encode(c.Message)
			require.NoError(s.T(), err)
			m := &message.Message{
				ChannelID: c.Channel.String(),
				Payload:   payload,
			}
			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, m)
			require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
		})
	}
}

// TestValidatorCallback_AuthorizedUnicastOnChannel checks that AuthorizedSenderValidator.Validate does not return an error
// for messages sent via unicast that are authorized to be sent via unicast.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_AuthorizedUnicastOnChannel() {
	for _, c := range s.authorizedUnicastOnChannel {
		str := fmt.Sprintf("role (%s) should be authorized to send message type (%s) on channel (%s) via unicast", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			msgType, err := authorizedSenderValidator.Validate(pid, []byte{c.MessageCode.Uint8()}, c.Channel, message.ProtocolTypeUnicast)
			require.NoError(s.T(), err)
			require.Equal(s.T(), c.MessageStr, msgType)
		})
	}
}

// TestValidatorCallback_UnAuthorizedUnicastOnChannel checks that AuthorizedSenderValidator.Validate returns message.ErrUnauthorizedUnicastOnChannel
// when a message not authorized to be sent via unicast is sent via unicast.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnAuthorizedUnicastOnChannel() {
	for _, c := range s.unauthorizedUnicastOnChannel {
		str := fmt.Sprintf("role (%s) should not be authorized to send message type (%s) on channel (%s) via unicast", c.Identity.Role, c.MessageStr, c.Channel)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)

			msgType, err := authorizedSenderValidator.Validate(pid, []byte{c.MessageCode.Uint8()}, c.Channel, message.ProtocolTypeUnicast)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedUnicastOnChannel)
			require.Equal(s.T(), c.MessageStr, msgType)
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

			msgType, err := authorizedSenderValidator.Validate(pid, []byte{c.MessageCode.Uint8()}, c.Channel, message.ProtocolTypeUnicast)
			require.ErrorIs(s.T(), err, message.ErrUnauthorizedMessageOnChannel)
			require.Equal(s.T(), c.MessageStr, msgType)

			payload, err := s.codec.Encode(c.Message)
			require.NoError(s.T(), err)
			m := &message.Message{
				ChannelID: c.Channel.String(),
				Payload:   payload,
			}
			validatePubsub := authorizedSenderValidator.PubSubMessageValidator(c.Channel)
			pubsubResult := validatePubsub(pid, m)
			require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
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

	// ensure ClusterBlockProposal not allowed to be sent on channel via unicast
	msgType, err := authorizedSenderValidator.Validate(pid, []byte{codec.CodeClusterBlockProposal.Uint8()}, channels.ConsensusCluster(clusterID), message.ProtocolTypeUnicast)
	require.ErrorIs(s.T(), err, message.ErrUnauthorizedUnicastOnChannel)
	require.Equal(s.T(), "*messages.ClusterBlockProposal", msgType)

	// ensure ClusterBlockProposal is allowed to be sent via pubsub by authorized sender
	payload, err := s.codec.Encode(&messages.ClusterBlockProposal{})
	require.NoError(s.T(), err)
	m := &message.Message{
		ChannelID: channels.ConsensusCluster(clusterID).String(),
		Payload:   payload,
	}
	validateCollConsensusPubsub := authorizedSenderValidator.PubSubMessageValidator(channels.ConsensusCluster(clusterID))
	pubsubResult := validateCollConsensusPubsub(pid, m)
	require.Equal(s.T(), p2p.ValidationAccept, pubsubResult)

	// validate collection sync cluster SyncRequest is not allowed to be sent on channel via unicast
	msgType, err = authorizedSenderValidator.Validate(pid, []byte{codec.CodeSyncRequest.Uint8()}, channels.SyncCluster(clusterID), message.ProtocolTypeUnicast)
	require.ErrorIs(s.T(), err, message.ErrUnauthorizedUnicastOnChannel)
	require.Equal(s.T(), "*messages.SyncRequest", msgType)

	// ensure SyncRequest is allowed to be sent via pubsub by authorized sender
	payload, err = s.codec.Encode(&messages.SyncRequest{})
	require.NoError(s.T(), err)
	m = &message.Message{
		ChannelID: channels.SyncCluster(clusterID).String(),
		Payload:   payload,
	}
	validateSyncClusterPubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCluster(clusterID))
	pubsubResult = validateSyncClusterPubsub(pid, m)
	require.Equal(s.T(), p2p.ValidationAccept, pubsubResult)
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

		msgType, err := authorizedSenderValidator.Validate(pid, []byte{codec.CodeSyncRequest.Uint8()}, channels.SyncCommittee, message.ProtocolTypeUnicast)
		require.ErrorIs(s.T(), err, ErrSenderEjected)
		require.Equal(s.T(), "", msgType)

		payload, err := s.codec.Encode(&messages.SyncRequest{})
		require.NoError(s.T(), err)
		m := &message.Message{
			ChannelID: channels.SyncCommittee.String(),
			Payload:   payload,
		}
		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCommittee)
		pubsubResult := validatePubsub(pid, m)
		require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
	})

	s.Run("unknown message code", func() {
		identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(flow.RoleConsensus))

		getIdentityFunc := s.getIdentity(identity)
		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)
		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.ConsensusCommittee)

		// unknown message types are rejected
		msgType, err := authorizedSenderValidator.Validate(pid, []byte{'x'}, channels.ConsensusCommittee, message.ProtocolTypeUnicast)
		require.True(s.T(), codec.IsErrUnknownMsgCode(err))
		require.Equal(s.T(), "", msgType)

		payload, err := s.codec.Encode(&messages.BlockProposal{})
		require.NoError(s.T(), err)
		payload[0] = byte('x')
		netMsg := &message.Message{
			ChannelID: channels.ConsensusCommittee.String(),
			Payload:   payload,
		}
		pubsubResult := validatePubsub(pid, netMsg)
		require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
	})

	s.Run("sender is not staked getIdentityFunc does not return identity ", func() {
		identity, _ := unittest.IdentityWithNetworkingKeyFixture()

		// getIdentityFunc simulates unstaked node not found in participant list
		getIdentityFunc := func(id peer.ID) (*flow.Identity, bool) { return nil, false }

		pid, err := unittest.PeerIDFromFlowID(identity)
		require.NoError(s.T(), err)

		authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, getIdentityFunc)

		msgType, err := authorizedSenderValidator.Validate(pid, []byte{codec.CodeSyncRequest.Uint8()}, channels.SyncCommittee, message.ProtocolTypeUnicast)
		require.ErrorIs(s.T(), err, ErrIdentityUnverified)
		require.Equal(s.T(), "", msgType)

		payload, err := s.codec.Encode(&messages.SyncRequest{})
		require.NoError(s.T(), err)
		m := &message.Message{
			ChannelID: channels.SyncCommittee.String(),
			Payload:   payload,
		}
		validatePubsub := authorizedSenderValidator.PubSubMessageValidator(channels.SyncCommittee)
		pubsubResult := validatePubsub(pid, m)
		require.Equal(s.T(), p2p.ValidationReject, pubsubResult)
	})
}

// TestValidatorCallback_ValidationFailure checks that AuthorizedSenderValidator returns the expected validation error when a unicast-only message is published.
func (s *TestAuthorizedSenderValidatorSuite) TestValidatorCallback_UnauthorizedPublishOnChannel() {
	for _, c := range s.authorizedUnicastOnChannel {
		str := fmt.Sprintf("message type (%s) is not authorized to be sent via libp2p publish", c.MessageStr)
		s.Run(str, func() {
			pid, err := unittest.PeerIDFromFlowID(c.Identity)
			require.NoError(s.T(), err)

			authorizedSenderValidator := NewAuthorizedSenderValidator(s.log, s.slashingViolationsConsumer, c.GetIdentity)
			msgType, err := authorizedSenderValidator.Validate(pid, []byte{c.MessageCode.Uint8()}, c.Channel, message.ProtocolTypePubSub)
			if c.MessageStr == "*message.TestMessage" {
				require.NoError(s.T(), err)
			} else {
				require.ErrorIs(s.T(), err, message.ErrUnauthorizedPublishOnChannel)
				require.Equal(s.T(), c.MessageStr, msgType)
			}
		})
	}
}

// initializeAuthorizationTestCases initializes happy and sad path test cases for checking authorized and unauthorized role message combinations.
func (s *TestAuthorizedSenderValidatorSuite) initializeAuthorizationTestCases() {
	for _, c := range s.allMsgConfigs {
		for channel, channelAuthConfig := range c.Config {
			for _, role := range flow.Roles() {
				identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(role))
				code, what, err := codec.MessageCodeFromInterface(c.Type())
				require.NoError(s.T(), err)
				tc := TestCase{
					Identity:    identity,
					GetIdentity: s.getIdentity(identity),
					Channel:     channel,
					Message:     c.Type(),
					MessageCode: code,
					MessageStr:  what,
					Protocols:   channelAuthConfig.AllowedProtocols,
				}
				if channelAuthConfig.AuthorizedRoles.Contains(role) {
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
	// iterate all channels
	for _, c := range s.allMsgConfigs {
		for channel, channelAuthConfig := range c.Config {
			identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(channelAuthConfig.AuthorizedRoles[0]))

			// iterate all message types
			for _, config := range s.allMsgConfigs {
				// include test if message type is not authorized on channel
				_, ok := config.Config[channel]
				code, what, err := codec.MessageCodeFromInterface(config.Type())
				require.NoError(s.T(), err)
				if config.Name != c.Name && !ok {
					tc := TestCase{
						Identity:    identity,
						GetIdentity: s.getIdentity(identity),
						Channel:     channel,
						Message:     config.Type(),
						MessageCode: code,
						MessageStr:  what,
						Protocols:   channelAuthConfig.AllowedProtocols,
					}
					s.unauthorizedMessageOnChannelTestCases = append(s.unauthorizedMessageOnChannelTestCases, tc)
				}
			}
		}
	}
}

// initializeUnicastOnChannelTestCases initializes happy and sad path test cases for unicast on channel message combinations.
func (s *TestAuthorizedSenderValidatorSuite) initializeUnicastOnChannelTestCases() {
	for _, c := range s.allMsgConfigs {
		for channel, channelAuthConfig := range c.Config {
			identity, _ := unittest.IdentityWithNetworkingKeyFixture(unittest.WithRole(channelAuthConfig.AuthorizedRoles[0]))
			code, what, err := codec.MessageCodeFromInterface(c.Type())
			require.NoError(s.T(), err)
			tc := TestCase{
				Identity:    identity,
				GetIdentity: s.getIdentity(identity),
				Channel:     channel,
				Message:     c.Type(),
				MessageCode: code,
				MessageStr:  what,
				Protocols:   channelAuthConfig.AllowedProtocols,
			}
			if channelAuthConfig.AllowedProtocols.Contains(message.ProtocolTypeUnicast) {
				s.authorizedUnicastOnChannel = append(s.authorizedUnicastOnChannel, tc)
			} else {
				s.unauthorizedUnicastOnChannel = append(s.unauthorizedUnicastOnChannel, tc)
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

package underlay

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	mockmsg "github.com/onflow/flow-go/network/mock"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/validator"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsProtocolParticipant_UnknownPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	idProvider.On("ByPeerID", remotePeerID).Return(nil, false).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestIsProtocolParticipant_EjectedPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	ejectedIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleExecution),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(ejectedIdentity, true).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ejected")
}

func TestIsProtocolParticipant_ActivePeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	activeIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(activeIdentity, true).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.NoError(t, err)
}

func TestGetAuthorizedIdentity_UnknownPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	idProvider.On("ByPeerID", remotePeerID).Return(nil, false).Once()
	violationsConsumer.On("OnUnauthorizedSenderError", &network.Violation{
		PeerID:   p2plogging.PeerId(remotePeerID),
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrIdentityUnverified,
	}).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.False(t, ok)
	require.Nil(t, identity)
}

func TestGetAuthorizedIdentity_EjectedPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	ejectedIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleExecution),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(ejectedIdentity, true).Once()
	violationsConsumer.On("OnSenderEjectedError", &network.Violation{
		OriginID: ejectedIdentity.NodeID,
		Identity: ejectedIdentity,
		PeerID:   p2plogging.PeerId(remotePeerID),
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrIdentityUnverified,
	}).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.False(t, ok)
	require.Nil(t, identity)
}

func TestGetAuthorizedIdentity_ActivePeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	activeIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(activeIdentity, true).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.True(t, ok)
	require.Equal(t, activeIdentity, identity)
}

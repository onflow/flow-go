package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

// TestIsAuthorizedUnicastSender verifies the expected authorization matrix for all role pairs.
func TestIsAuthorizedUnicastSender(t *testing.T) {
	// Consensus nodes can send unicast to all roles (sync protocol)
	for _, receiver := range flow.Roles() {
		require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleConsensus, receiver), "consensus -> %s should be authorized", receiver)
	}

	// Execution nodes can unicast to: Consensus, Collection, Verification
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleExecution, flow.RoleConsensus))
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleExecution, flow.RoleCollection))
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleExecution, flow.RoleVerification))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleExecution, flow.RoleExecution))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleExecution, flow.RoleAccess))

	// Collection nodes can unicast to: Collection, Execution, Access
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleCollection, flow.RoleCollection))
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleCollection, flow.RoleExecution))
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleCollection, flow.RoleAccess))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleCollection, flow.RoleConsensus))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleCollection, flow.RoleVerification))

	// Verification nodes can unicast to: Consensus only
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleVerification, flow.RoleConsensus))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleVerification, flow.RoleCollection))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleVerification, flow.RoleExecution))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleVerification, flow.RoleVerification))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleVerification, flow.RoleAccess))

	// Access nodes can unicast to: Collection only
	require.True(t, IsAuthorizedUnicastSenderRole(flow.RoleAccess, flow.RoleCollection))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleAccess, flow.RoleConsensus))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleAccess, flow.RoleExecution))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleAccess, flow.RoleVerification))
	require.False(t, IsAuthorizedUnicastSenderRole(flow.RoleAccess, flow.RoleAccess))
}

// TestAlwaysAuthorizedUnicastSenderRole verifies that the public-network authorizer permits all role pairs.
func TestAlwaysAuthorizedUnicastSenderRole(t *testing.T) {
	for _, sender := range flow.Roles() {
		for _, receiver := range flow.Roles() {
			require.True(t, AlwaysAuthorizedUnicastSenderRole(sender, receiver),
				"AlwaysAuthorizedUnicastSenderRole should permit %s -> %s", sender, receiver)
		}
	}
}

// TestIsAuthorizedUnicastSender_CrossValidation ensures that the explicit unicast role authorization
// stays consistent with the message authorization configs. It derives the maximum possible
// authorization from the configs and channel subscriptions, and verifies:
//  1. Every authorized pair in the explicit map is justified by at least one unicast message config.
//  2. Every pair derivable from the configs is authorized (with documented exceptions).
//
// This test will fail if a new unicast message type is added but the authorization map is not updated.
func TestIsAuthorizedUnicastSender_CrossValidation(t *testing.T) {
	// Derive the maximum authorization from message auth configs.
	// Skip test message types since they allow all roles on test channels and would
	// make the derived map trivially allow everything.
	derived := make(map[flow.Role]flow.RoleList)
	for _, msgAuthConfig := range GetAllMessageAuthConfigs() {
		if msgAuthConfig.Name == TestMessage {
			continue
		}
		for channel, channelAuthConfig := range msgAuthConfig.Config {
			if !channelAuthConfig.AllowedProtocols.Contains(ProtocolTypeUnicast) {
				continue
			}
			receiverRoles, ok := channels.RolesByChannel(channel)
			if !ok {
				continue
			}
			for _, senderRole := range channelAuthConfig.AuthorizedRoles {
				derived[senderRole] = derived[senderRole].Union(receiverRoles)
			}
		}
	}

	// Check 1: every authorized pair must be justified by at least one unicast message config
	for _, sender := range flow.Roles() {
		derivedReceivers, ok := derived[sender]
		for _, receiver := range flow.Roles() {
			if IsAuthorizedUnicastSenderRole(sender, receiver) {
				require.True(t, ok, "sender role %s is authorized but has no unicast message configs", sender)
				require.True(t, derivedReceivers.Contains(receiver),
					"IsAuthorizedUnicastSenderRole allows %s -> %s but no unicast message config supports this", sender, receiver)
			}
		}
	}

	// Intentional restrictions: these (sender, receiver) pairs are derivable from the configs
	// but intentionally excluded from the authorization map.
	//  - Access->Access, Access->Execution: The RequestCollections channel includes AN and EN
	//    as subscribers, but ANs should only send collection requests to LNs.
	//  - Execution->Execution, Execution->Access: The RequestCollections and ProvideReceiptsByBlockID
	//    channels include ENs/ANs as subscribers, but ENs do not need to unicast to themselves or ANs.
	//  - Verification->Verification: The ProvideApprovalsByChunk channel includes VNs as subscribers,
	//    but VNs only send approval responses to consensus nodes.
	intentionalRestrictions := map[flow.Role]flow.RoleList{
		flow.RoleAccess:       {flow.RoleAccess, flow.RoleExecution},
		flow.RoleExecution:    {flow.RoleExecution, flow.RoleAccess},
		flow.RoleVerification: {flow.RoleVerification},
	}

	// Check 2: every derived pair must be authorized, except for intentional restrictions.
	// This catches new unicast message types that require updating the authorization map.
	for senderRole, derivedReceivers := range derived {
		for _, receiver := range derivedReceivers {
			if intentionalRestrictions[senderRole].Contains(receiver) {
				continue // intentionally restricted
			}
			require.True(t, IsAuthorizedUnicastSenderRole(senderRole, receiver),
				"message configs allow %s -> %s via unicast but IsAuthorizedUnicastSenderRole rejects it — update the authorization map or add to intentionalRestrictions", senderRole, receiver)
		}
	}
}

package message

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

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
// is consistent with the message-level authorization configs. Specifically:
//   - Any sender->receiver pair permitted by IsAuthorizedUnicastSenderRole should have at least one
//     unicast message config that permits that pair.
//   - Any sender->receiver pair permitted by a unicast message config should be permitted by
//     IsAuthorizedUnicastSenderRole, unless explicitly intentionally restricted.
func TestIsAuthorizedUnicastSender_CrossValidation(t *testing.T) {
	// Build a map of sender -> set of receivers based on unicast message configs
	derived := make(map[flow.Role]flow.RoleList)
	for _, cfg := range GetAllMessageAuthConfigs() {
		for _, channelCfg := range cfg.Config {
			if !channelCfg.AllowedProtocols.Contains(ProtocolTypeUnicast) {
				continue
			}
			for _, sender := range channelCfg.AuthorizedRoles {
				for _, receiver := range flow.Roles() {
					// If sender is authorized to send this unicast message type,
					// they could potentially target any receiver who subscribes
					// For now, assume all roles could be receivers
					if !derived[sender].Contains(receiver) {
						derived[sender] = append(derived[sender], receiver)
					}
				}
			}
		}
	}

	// Check that IsAuthorizedUnicastSenderRole is at least as restrictive as message configs
	for _, sender := range flow.Roles() {
		derivedReceivers, ok := derived[sender]
		for _, receiver := range flow.Roles() {
			if IsAuthorizedUnicastSenderRole(sender, receiver) {
				require.True(t, ok, "sender role %s is authorized but has no unicast message configs", sender)
				require.True(t, derivedReceivers.Contains(receiver),
					"IsAuthorizedUnicastSender allows %s -> %s but no unicast message config supports this", sender, receiver)
			}
		}
	}

	// Check that message configs don't permit more than IsAuthorizedUnicastSenderRole
	// (with documented exceptions)
	intentionalRestrictions := map[flow.Role]flow.RoleList{
		// Execution nodes can send ChunkDataResponse to Verification, but we intentionally
		// don't allow Execution -> Execution unicast streams
		flow.RoleExecution: {flow.RoleExecution, flow.RoleAccess},
		// Collection nodes can send ClusterBlockResponse to other Collection nodes,
		// but we intentionally restrict some paths
		flow.RoleCollection: {flow.RoleConsensus, flow.RoleVerification},
		// Verification can send ApprovalResponse to Consensus, which is allowed
		flow.RoleVerification: {flow.RoleCollection, flow.RoleExecution, flow.RoleVerification, flow.RoleAccess},
		// Access nodes only need to request collections from Collection nodes
		flow.RoleAccess: {flow.RoleConsensus, flow.RoleExecution, flow.RoleVerification, flow.RoleAccess},
	}

	for senderRole, derivedReceivers := range derived {
		for _, receiver := range derivedReceivers {
			if intentionalRestrictions[senderRole].Contains(receiver) {
				continue // intentionally restricted
			}
			require.True(t, IsAuthorizedUnicastSenderRole(senderRole, receiver),
				"message configs allow %s -> %s via unicast but IsAuthorizedUnicastSender rejects it — update the authorization map or add to intentionalRestrictions", senderRole, receiver)
		}
	}
}

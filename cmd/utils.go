package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/cache"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/p2p/translator"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

// loadRootProtocolSnapshot loads the root protocol snapshot from disk
func loadRootProtocolSnapshot(dir string) (*inmem.Snapshot, error) {
	path := filepath.Join(dir, bootstrap.PathRootProtocolStateSnapshot)
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read root snapshot (path=%s): %w", path, err)
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return inmem.SnapshotFromEncodable(snapshot), nil
}

// LoadPrivateNodeInfo the private info for this node from disk (e.g., private staking/network keys).
func LoadPrivateNodeInfo(dir string, myID flow.Identifier) (*bootstrap.NodeInfoPriv, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read private node info (path=%s): %w", path, err)
	}
	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	return &info, err
}

func LoadNetworkPrivateKey(dir string, myID flow.Identifier) (crypto.PrivateKey, error) {
	path := filepath.Join(dir, fmt.Sprintf(filepath.Join(bootstrap.DirPrivateRoot,
		"private-node-info_%v/network_private_key"), myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read private node info (path=%s): %w", path, err)
	}

	keyBytes, err := hex.DecodeString(strings.Trim(string(data), "\n "))
	if err != nil {
		return nil, fmt.Errorf("could not hex decode networking key (path=%s): %w", path, err)
	}

	networkingKey, err := crypto.DecodePrivateKey(crypto.ECDSASecp256k1, keyBytes)
	if err != nil {
		return nil, fmt.Errorf("could not decode networking key (path=%s): %w", path, err)
	}
	return networkingKey, nil
}

// loadSecretsEncryptionKey loads the encryption key for the secrets database.
// If the file does not exist, returns os.ErrNotExist.
func loadSecretsEncryptionKey(dir string, myID flow.Identifier) ([]byte, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read secrets db encryption key (path=%s): %w", path, err)
	}
	return data, nil
}

func rateLimiterPeerFilter(rateLimiter p2p.RateLimiter) p2p.PeerFilter {
	return func(p peer.ID) error {
		if rateLimiter.IsRateLimited(p) {
			return fmt.Errorf("peer is rate limited")
		}

		return nil
	}
}

// BootstrapIdentities converts the bootstrap node addresses and keys to a Flow Identity list where
// each Flow Identity is initialized with the passed address, the networking key
// and the Node ID set to ZeroID, role set to Access, 0 stake and no staking key.
func BootstrapIdentities(addresses []string, keys []string) (flow.IdentitySkeletonList, error) {
	if len(addresses) != len(keys) {
		return nil, fmt.Errorf("number of addresses and keys provided for the boostrap nodes don't match")
	}

	ids := make(flow.IdentitySkeletonList, len(addresses))
	for i, address := range addresses {
		bytes, err := hex.DecodeString(keys[i])
		if err != nil {
			return nil, fmt.Errorf("failed to decode secured GRPC server public key hex %w", err)
		}

		publicFlowNetworkingKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to get public flow networking key could not decode public key bytes %w", err)
		}

		// create the identity of the peer by setting only the relevant fields
		ids[i] = &flow.IdentitySkeleton{
			NodeID:        flow.ZeroID, // the NodeID is the hash of the staking key and for the public network it does not apply
			Address:       address,
			Role:          flow.RoleAccess, // the upstream node has to be an access node
			NetworkPubKey: publicFlowNetworkingKey,
		}
	}
	return ids, nil
}

func CreatePublicIDTranslatorAndIdentifierProvider(
	logger zerolog.Logger,
	networkKey crypto.PrivateKey,
	sporkID flow.Identifier,
	getLibp2pNode func() p2p.LibP2PNode,
	idCache *cache.ProtocolStateIDCache,
) (
	p2p.IDTranslator,
	func() module.IdentifierProvider,
	error) {
	idTranslator := translator.NewHierarchicalIDTranslator(idCache, translator.NewPublicNetworkIDTranslator())

	peerID, err := peerIDFromNetworkKey(networkKey)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get peer ID from network key: %w", err)
	}
	// use the default identifier provider
	factory := func() module.IdentifierProvider {
		return id.NewCustomIdentifierProvider(func() flow.IdentifierList {
			pids := getLibp2pNode().GetPeersForProtocol(protocols.FlowProtocolID(sporkID))
			result := make(flow.IdentifierList, 0, len(pids))

			for _, pid := range pids {
				// exclude own Identifier
				if pid == peerID {
					continue
				}

				if flowID, err := idTranslator.GetFlowID(pid); err != nil {
					// TODO: this is an instance of "log error and continue with best effort" anti-pattern
					logger.Err(err).Str("peer", p2plogging.PeerId(pid)).Msg("failed to translate to Flow ID")
				} else {
					result = append(result, flowID)
				}
			}

			return result
		})
	}

	return idTranslator, factory, nil
}

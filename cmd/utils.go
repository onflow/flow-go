package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
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

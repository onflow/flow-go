package common

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/rs/zerolog"

	"github.com/multiformats/go-multiaddr"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/utils/io"
)

func FilesInDir(dir string) ([]string, error) {
	exists, err := PathExists(dir)
	if err != nil {
		return nil, fmt.Errorf("could not check if dir exists: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("dir %v does not exist", dir)
	}

	var files []string
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// PathExists
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func ReadJSON(path string, target interface{}) error {
	dat, err := io.ReadFile(path)
	if err != nil {
		return fmt.Errorf("cannot read json: %w", err)
	}
	err = json.Unmarshal(dat, target)
	if err != nil {
		return fmt.Errorf("cannot unmarshal json in file %s: %w", path, err)
	}
	return nil
}

func WriteJSON(path string, out string, data interface{}) error {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal json: %w", err)
	}

	return WriteText(path, out, bz)
}

func WriteText(path string, out string, data []byte) error {
	path = filepath.Join(out, path)

	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("could not create output dir: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}
	return nil
}

func PubKeyToString(key crypto.PublicKey) string {
	return fmt.Sprintf("%x", key.Encode())
}

func NodeCountByRole(nodes []bootstrap.NodeInfo) map[flow.Role]uint16 {
	roleCounts := map[flow.Role]uint16{
		flow.RoleCollection:   0,
		flow.RoleConsensus:    0,
		flow.RoleExecution:    0,
		flow.RoleVerification: 0,
		flow.RoleAccess:       0,
	}
	for _, node := range nodes {
		roleCounts[node.Role] = roleCounts[node.Role] + 1
	}

	return roleCounts
}

// ValidateAddressFormat validates the address provided by pretty much doing what the network layer would do before
// starting the node
func ValidateAddressFormat(log zerolog.Logger, address string) {
	checkErr := func(err error) {
		if err != nil {
			log.Fatal().Err(err).Str("address", address).Msg("invalid address format.\n" +
				`Address needs to be in the format hostname:port or ip:port e.g. "flow.com:3569"`)
		}
	}

	// split address into ip/hostname and port
	ip, port, err := net.SplitHostPort(address)
	checkErr(err)

	// check that port number is indeed a number
	_, err = strconv.Atoi(port)
	checkErr(err)

	// create a libp2p address from the ip and port
	lp2pAddr := utils.MultiAddressStr(ip, port)
	_, err = multiaddr.NewMultiaddr(lp2pAddr)
	checkErr(err)
}

// ValidateNodeID returns an error if node ID is non-zero.
// Args:
// - nodeID: the node ID to validate.
// Returns:
// - error: if node id is the zero value.
func ValidateNodeID(nodeID flow.Identifier) error {
	if nodeID == flow.ZeroID {
		return fmt.Errorf("NodeID must not be zero")
	}
	return nil
}

// ValidateNetworkPubKey returns an error if network public key is nil.
// Args:
// - key: the public key.
// Returns:
// - error: if the network key is nil.
func ValidateNetworkPubKey(key encodable.NetworkPubKey) error {
	if key.PublicKey == nil {
		return fmt.Errorf("network public key must not be nil")
	}
	return nil
}

// ValidateStakingPubKey returns an error if the staking key is nil.
// Args:
// - key: the public key.
// Returns:
// - error: if the staking key is nil.
func ValidateStakingPubKey(key encodable.StakingPubKey) error {
	if key.PublicKey == nil {
		return fmt.Errorf("staking public key must not be nil")
	}
	return nil
}

// ValidateWeight returns true if weight is greater than 0.
// Args:
// - weight: the weight to check.
// Returns:
// - bool: true if weight is greater than 0.
func ValidateWeight(weight uint64) bool {
	return weight > 0
}

// PartnerWeights is the format of the JSON file specifying partner node weights.
type PartnerWeights map[flow.Identifier]uint64

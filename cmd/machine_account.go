package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/io"
)

// LoadNodeMachineAccountInfoFile loads machine account info from the default location within the
// bootstrap directory - Currently being used by Collection and Consensus nodes
func LoadNodeMachineAccountInfoFile(bootstrapDir string, nodeID flow.Identifier) (*bootstrap.NodeMachineAccountInfo, error) {

	// attempt to read file
	machineAccountInfoPath := filepath.Join(bootstrapDir, fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeID))
	bz, err := io.ReadFile(machineAccountInfoPath)
	if err != nil {
		return nil, fmt.Errorf("could not read machine account info: %w", err)
	}

	// unmashal machine account info
	var machineAccountInfo bootstrap.NodeMachineAccountInfo
	err = json.Unmarshal(bz, &machineAccountInfo)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal machine account info: %w", err)
	}

	return &machineAccountInfo, nil
}

// IsValidNodeMachineAccountConfig returns true if the machine account file exists at the default path and
// if the `access-address` is set else returns false.
func IsValidNodeMachineAccountConfig(node *NodeConfig, accessAddress string) bool {

	// check if node machine account info file exists
	machineAccountInfoPath := filepath.Join(node.BaseConfig.BootstrapDir, fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, node.Me.NodeID()))
	exists := io.FileExists(machineAccountInfoPath)

	if accessAddress == "" || !exists {
		// log warning as one or more of the required components are missing
		node.Logger.Warn().Msg("node machine account info file or the required flags are not configured properly")
		return false
	}

	return true
}

package bootstrap

import "path/filepath"

// Canonical filenames/paths for bootstrapping files.
var (
	// execution state
	DirnameExecutionState = "execution-state"

	// public genesis information
	PathDKGDataPub            = filepath.Join("public-genesis-information", "dkg-data.pub.json")
	PathGenesisBlock          = filepath.Join("public-genesis-information", "genesis-block.json")
	PathGenesisClusterBlock   = filepath.Join("public-genesis-information", "genesis-cluster-block.%v.json") // %v will be replaced by cluster ID
	PathGenesisClusterQC      = filepath.Join("public-genesis-information", "genesis-cluster-qc.%v.json")    // %v will be replaced by cluster ID
	PathGenesisCommit         = filepath.Join("public-genesis-information", "genesis-commit.json")
	PathGenesisQC             = filepath.Join("public-genesis-information", "genesis-qc.json")
	PathNodeInfosPub          = filepath.Join("public-genesis-information", "node-infos.pub.json")
	PathNodeInfoPub           = filepath.Join("public-genesis-information", "node-info.pub.%v.json") // %v will be replaced by NodeID
	PathPartnerNodeInfoPrefix = filepath.Join("public-genesis-information", "node-info.pub.")

	// private genesis information
	PathAccount0Priv         = filepath.Join("private-genesis-information", "account-0.priv.json")
	FilenameNodeId           = "node-id"
	PathNodeId               = filepath.Join("private-genesis-information", "private-node-info_%v", FilenameNodeId)        // %v will be replaced by NodeID
	PathNodeInfoPriv         = filepath.Join("private-genesis-information", "private-node-info_%v", "node-info.priv.json") // %v will be replaced by NodeID
	FilenameRandomBeaconPriv = "random-beacon.priv.json"
	PathRandomBeaconPriv     = filepath.Join("private-genesis-information", "private-node-info_%v", FilenameRandomBeaconPriv) // %v will be replaced by NodeID
)

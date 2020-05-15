package bootstrap

import "path/filepath"

// Canonical filenames/paths for bootstrapping files.
var (
	// execution state
	DirnameExecutionState = "execution-state"

	// public genesis information
	PathPublicGenesis         = "public-genesis-information"
	PathDKGDataPub            = filepath.Join(PathPublicGenesis, "dkg-data.pub.json")
	PathGenesisBlock          = filepath.Join(PathPublicGenesis, "genesis-block.json")
	PathGenesisClusterBlock   = filepath.Join(PathPublicGenesis, "genesis-cluster-block.%v.json") // %v will be replaced by cluster ID
	PathGenesisClusterQC      = filepath.Join(PathPublicGenesis, "genesis-cluster-qc.%v.json")    // %v will be replaced by cluster ID
	PathGenesisCommit         = filepath.Join(PathPublicGenesis, "genesis-commit.json")
	PathGenesisQC             = filepath.Join(PathPublicGenesis, "genesis-qc.json")
	PathNodeInfosPub          = filepath.Join(PathPublicGenesis, "node-infos.pub.json")
	PathNodeInfoPub           = filepath.Join(PathPublicGenesis, "node-info.pub.%v.json") // %v will be replaced by NodeID
	PathPartnerNodeInfoPrefix = filepath.Join(PathPublicGenesis, "node-info.pub.")
	// The Node ID file is used as a helper by the transit scripts
	FilenameNodeId = "node-id"
	PathNodeId     = filepath.Join(PathPublicGenesis, FilenameNodeId) // %v will be replaced by NodeID

	// private genesis information
	PathPrivateGenesis       = "private-genesis-information"
	PathAccount0Priv         = filepath.Join(PathPrivateGenesis, "account-0.priv.json")
	PathNodeInfoPriv         = filepath.Join(PathPrivateGenesis, "private-node-info_%v", "node-info.priv.json") // %v will be replaced by NodeID
	FilenameRandomBeaconPriv = "random-beacon.priv.json"
	PathRandomBeaconPriv     = filepath.Join(PathPrivateGenesis, "private-node-info_%v", FilenameRandomBeaconPriv) // %v will be replaced by NodeID
)

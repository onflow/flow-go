package bootstrap

import "path/filepath"

// Canonical filenames/paths for bootstrapping files.
var (
	// execution state
	DirnameExecutionState = "execution-state"

	// public genesis information
	DirnamePublicGenesis      = "public-genesis-information"
	PathDKGDataPub            = filepath.Join(DirnamePublicGenesis, "dkg-data.pub.json")
	PathGenesisBlock          = filepath.Join(DirnamePublicGenesis, "genesis-block.json")
	PathGenesisClusterBlock   = filepath.Join(DirnamePublicGenesis, "genesis-cluster-block.%v.json") // %v will be replaced by cluster ID
	PathGenesisClusterQC      = filepath.Join(DirnamePublicGenesis, "genesis-cluster-qc.%v.json")    // %v will be replaced by cluster ID
	PathGenesisCommit         = filepath.Join(DirnamePublicGenesis, "genesis-commit.json")
	PathGenesisQC             = filepath.Join(DirnamePublicGenesis, "genesis-qc.json")
	PathNodeInfosPub          = filepath.Join(DirnamePublicGenesis, "node-infos.pub.json")
	PathNodeInfoPub           = filepath.Join(DirnamePublicGenesis, "node-info.pub.%v.json") // %v will be replaced by NodeID
	PathPartnerNodeInfoPrefix = filepath.Join(DirnamePublicGenesis, "node-info.pub.")
	// The Node ID file is used as a helper by the transit scripts
	FilenameNodeId = "node-id"
	PathNodeId     = filepath.Join(DirnamePublicGenesis, FilenameNodeId) // %v will be replaced by NodeID

	// private genesis information
	DirPrivateGenesis        = "private-genesis-information"
	PathAccount0Priv         = filepath.Join(DirPrivateGenesis, "account-0.priv.json")
	PathNodeInfoPriv         = filepath.Join(DirPrivateGenesis, "private-node-info_%v", "node-info.priv.json") // %v will be replaced by NodeID
	FilenameRandomBeaconPriv = "random-beacon.priv.json"
	PathRandomBeaconPriv     = filepath.Join(DirPrivateGenesis, "private-node-info_%v", FilenameRandomBeaconPriv) // %v will be replaced by NodeID
)

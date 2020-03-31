package cmd

const (
	randomSeedBytes           = 48
	minSeedBytes              = 48
	minNodesPerCluster uint16 = 3

	DirnameExecutionState         = "execution-state"
	FilenameGenesisBlock          = "genesis-block.json"
	FilenameGenesisClusterBlock   = "%v.genesis-cluster-block.json"
	FilenameNodeInfosPub          = "node-infos.pub.json"
	FilenameNodeInfoPriv          = "%v.node-info.priv.json"     // %v will be replaced by NodeID
	FilenameNodeInfoPub           = "%v.node-info.pub.json"      // %v will be replaced by NodeID
	FilenamePartnerNodeInfoSuffix = ".node-info.pub.json"        // %v will be replaced by NodeID
	FilenameRandomBeaconPriv      = "%v.random-beacon.priv.json" // %v will be replaced by NodeID
	FilenameDKGDataPub            = "dkg-data.pub.json"
	FilenameAccount0Priv          = "account-0.priv.json"
	FilenameGenesisQC             = "genesis-qc.json"
	FilenameGenesisClusterQC      = "%v.genesis-cluster-qc.json"
)

package cmd

const (
	randomSeedBytes           = 48
	minSeedBytes              = 48
	minNodesPerCluster uint16 = 3

	dirnameExecutionState         = "execution-state"
	filenameGenesisBlock          = "genesis-block.json"
	filenameGenesisClusterBlock   = "%v.genesis-cluster-block.json"
	filenameNodeInfosPub          = "node-infos.pub.json"
	filenameNodeInfoPriv          = "%v.node-info.priv.json"     // %v will be replaced by NodeID
	filenameNodeInfoPub           = "%v.node-info.pub.json"      // %v will be replaced by NodeID
	filenamePartnerNodeInfoSuffix = ".node-info.pub.json"        // %v will be replaced by NodeID
	filenameRandomBeaconPriv      = "%v.random-beacon.priv.json" // %v will be replaced by NodeID
	filenameDKGDataPub            = "dkg-data.pub.json"
	filenameAccount0Priv          = "account-0.priv.json"
	filenameGenesisQC             = "genesis-qc.json"
	filenameGenesisClusterQC      = "%v.genesis-cluster-qc.json"
)

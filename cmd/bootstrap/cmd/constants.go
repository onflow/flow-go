package cmd

const (
	randomSeedBytes = 64
	minSeedBytes    = 64

	DirnameExecutionState         = "execution-state"
	FilenameGenesisBlock          = "genesis-block.json"
	FilenameNodeInfosPub          = "node-infos.pub.json"
	FilenameNodeInfoPriv          = "%v.node-info.priv.json"     // %v will be replaced by NodeID
	FilenameNodeInfoPub           = "%v.node-info.pub.json"      // %v will be replaced by NodeID
	FilenamePartnerNodeInfoSuffix = ".node-info.pub.json"        // %v will be replaced by NodeID
	FilenameRandomBeaconPriv      = "%v.random-beacon.priv.json" // %v will be replaced by NodeID
	FilenameDKGDataPub            = "dkg-data.pub.json"
	FilenameAccount0Priv          = "account-0.priv.json"
	FilenameGenesisQC             = "genesis-qc.json"
)

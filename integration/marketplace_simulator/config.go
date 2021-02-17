package marketplace

import (
	"time"

	flowsdk "github.com/onflow/flow-go-sdk"
)

type NetworkConfig struct {
	ServiceAccountAddress       *flowsdk.Address
	FlowTokenAddress            *flowsdk.Address
	FungibleTokenAddress        *flowsdk.Address
	ServiceAccountPrivateKeyHex string
	AccessNodeAddresses         []string
}

type SimulatorConfig struct {
	NumberOfAccounts          int
	NumberOfMomentsPerAccount int
	AccountGroupSize          int
	MomentsToTransferPerTx    int
	Delay                     time.Duration
	NBATopshotAddress         *flowsdk.Address
	// DoSetup              bool
	// AccountsJSONFilePath string // if not provided will creates and prints the private key hex
}

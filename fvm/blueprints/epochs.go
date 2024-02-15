package blueprints

import (
	_ "embed"
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	flowsdk "github.com/onflow/flow-go-sdk"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/epochs"
)

//go:embed scripts/deployIDTableStakingTransactionTemplate.cdc
var deployIDTableStakingTransactionTemplate string

//go:embed scripts/deployEpochTransactionTemplate.cdc
var deployEpochTransactionTemplate string

//go:embed scripts/setupAccountTemplate.cdc
var setupAccountTemplate string

//go:embed scripts/fundAccountTemplate.cdc
var fundAccountTemplate string

//go:embed scripts/deployLockedTokensTemplate.cdc
var deployLockedTokensTemplate string

// DeployEpochTransaction returns the transaction body for the deploy epoch transaction
func DeployEpochTransaction(service flow.Address, contract []byte, epochConfig epochs.EpochConfig) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(
			templates.ReplaceAddresses(
				deployEpochTransactionTemplate,
				templates.Environment{
					QuorumCertificateAddress: service.Hex(),
				},
			),
		)).
		AddArgument(jsoncdc.MustEncode(cadence.String("FlowEpoch"))).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contract)))).
		AddArgument(jsoncdc.MustEncode(epochConfig.CurrentEpochCounter)).
		AddArgument(jsoncdc.MustEncode(epochConfig.NumViewsInEpoch)).
		AddArgument(jsoncdc.MustEncode(epochConfig.NumViewsInStakingAuction)).
		AddArgument(jsoncdc.MustEncode(epochConfig.NumViewsInDKGPhase)).
		AddArgument(jsoncdc.MustEncode(epochConfig.NumCollectorClusters)).
		AddArgument(jsoncdc.MustEncode(epochConfig.FLOWsupplyIncreasePercentage)).
		AddArgument(jsoncdc.MustEncode(epochConfig.RandomSource)).
		AddArgument(epochs.EncodeClusterAssignments(epochConfig.CollectorClusters)).
		AddAuthorizer(service)
}

// SetupAccountTransaction returns the transaction body for the setup account transaction
func SetupAccountTransaction(
	fungibleToken flow.Address,
	flowToken flow.Address,
	accountAddress flow.Address,
) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(
			templates.ReplaceAddresses(
				setupAccountTemplate,
				templates.Environment{
					FungibleTokenAddress: fungibleToken.Hex(),
					FlowTokenAddress:     flowToken.Hex(),
				},
			),
		)).
		AddAuthorizer(accountAddress)
}

// DeployIDTableStakingTransaction returns the transaction body for the deploy id table staking transaction
func DeployIDTableStakingTransaction(service flow.Address, contract []byte, epochTokenPayout cadence.UFix64, rewardCut cadence.UFix64) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(deployIDTableStakingTransactionTemplate)).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contract)))).
		AddArgument(jsoncdc.MustEncode(epochTokenPayout)).
		AddArgument(jsoncdc.MustEncode(rewardCut)).
		AddAuthorizer(service)
}

// FundAccountTransaction returns the transaction body for the fund account transaction
func FundAccountTransaction(
	service flow.Address,
	fungibleToken flow.Address,
	flowToken flow.Address,
	nodeAddress flow.Address,
) *flow.TransactionBody {

	cdcAmount, err := cadence.NewUFix64(fmt.Sprintf("%d.0", 2_000_000))
	if err != nil {
		panic(err)
	}

	return flow.NewTransactionBody().
		SetScript([]byte(templates.ReplaceAddresses(
			fundAccountTemplate,
			templates.Environment{
				FungibleTokenAddress: fungibleToken.Hex(),
				FlowTokenAddress:     flowToken.Hex(),
			},
		))).
		AddArgument(jsoncdc.MustEncode(cdcAmount)).
		AddArgument(jsoncdc.MustEncode(cadence.NewAddress(nodeAddress))).
		AddAuthorizer(service)
}

// DeployLockedTokensTransaction returns the transaction body for the deploy locked tokens transaction
func DeployLockedTokensTransaction(service flow.Address, contract []byte, publicKeys []cadence.Value) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(
			deployLockedTokensTemplate,
		)).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(publicKeys))).
		AddArgument(jsoncdc.MustEncode(cadence.String(hex.EncodeToString(contract)))).
		AddAuthorizer(service)
}

// RegisterNodeTransaction creates a new node struct object.
// Then, if the node is a collector node, creates a new account and adds a QC object to it
// If the node is a consensus node, it creates a new account and adds a DKG object to it
func RegisterNodeTransaction(
	service flow.Address,
	flowTokenAddress flow.Address,
	fungibleTokenAddress flow.Address,
	nodeAddress flow.Address,
	id *flow.Identity,
) *flow.TransactionBody {

	env := templates.Environment{
		FlowTokenAddress:         flowTokenAddress.HexWithPrefix(),
		FungibleTokenAddress:     fungibleTokenAddress.HexWithPrefix(),
		IDTableAddress:           service.HexWithPrefix(),
		QuorumCertificateAddress: service.HexWithPrefix(),
		DkgAddress:               service.HexWithPrefix(),
		EpochAddress:             service.HexWithPrefix(),
	}

	// Use NetworkingKey as the public key of the machine account.
	// We do this for tests/localnet but normally it should be a separate key.
	accountKey := &flowsdk.AccountKey{
		PublicKey: id.NetworkPubKey,
		SigAlgo:   id.NetworkPubKey.Algorithm(),
		HashAlgo:  bootstrap.DefaultMachineAccountHashAlgo,
		Weight:    1000,
	}

	cadenceCryptoKey, err := sdktemplates.AccountKeyToCadenceCryptoKey(accountKey)
	if err != nil {
		panic(err)
	}

	cadencePublicKeys := cadence.NewArray(
		[]cadence.Value{
			cadenceCryptoKey,
		},
	)

	cdcAmount, err := cadence.NewUFix64("1250000.0")
	if err != nil {
		panic(err)
	}

	cdcNodeID, err := cadence.NewString(id.NodeID.String())
	if err != nil {
		panic(err)
	}

	cdcAddress, err := cadence.NewString(id.Address)
	if err != nil {
		panic(err)
	}

	cdcNetworkPubKey, err := cadence.NewString(id.NetworkPubKey.String()[2:])
	if err != nil {
		panic(err)
	}

	cdcStakingPubKey, err := cadence.NewString(id.StakingPubKey.String()[2:])
	if err != nil {
		panic(err)
	}

	// register node
	return flow.NewTransactionBody().
		SetScript(templates.GenerateEpochRegisterNodeScript(env)).
		AddArgument(jsoncdc.MustEncode(cdcNodeID)).
		AddArgument(jsoncdc.MustEncode(cadence.NewUInt8(uint8(id.Role)))).
		AddArgument(jsoncdc.MustEncode(cdcAddress)).
		AddArgument(jsoncdc.MustEncode(cdcNetworkPubKey)).
		AddArgument(jsoncdc.MustEncode(cdcStakingPubKey)).
		AddArgument(jsoncdc.MustEncode(cdcAmount)).
		AddArgument(jsoncdc.MustEncode(cadencePublicKeys)).
		AddAuthorizer(nodeAddress)
}

// SetStakingAllowlistTransaction returns transaction body for set staking allowlist transaction
func SetStakingAllowlistTransaction(idTableStakingAddr flow.Address, allowedNodeIDs []flow.Identifier) *flow.TransactionBody {
	env := templates.Environment{
		IDTableAddress: idTableStakingAddr.HexWithPrefix(),
	}
	allowedNodesArg := SetStakingAllowlistTxArg(allowedNodeIDs)
	return flow.NewTransactionBody().
		SetScript(templates.GenerateSetApprovedNodesScript(env)).
		AddArgument(jsoncdc.MustEncode(allowedNodesArg)).
		AddAuthorizer(idTableStakingAddr)
}

// SetStakingAllowlistTxArg returns the transaction argument for setting the staking allow-list.
func SetStakingAllowlistTxArg(allowedNodeIDs []flow.Identifier) cadence.Value {
	cdcDictEntries := make([]cadence.KeyValuePair, 0, len(allowedNodeIDs))
	for _, id := range allowedNodeIDs {
		cdcNodeID, err := cadence.NewString(id.String())
		if err != nil {
			panic(err)
		}
		kvPair := cadence.KeyValuePair{
			Key:   cdcNodeID,
			Value: cadence.NewBool(true),
		}
		cdcDictEntries = append(cdcDictEntries, kvPair)
	}
	return cadence.NewDictionary(cdcDictEntries)
}

// BytesToCadenceArray converts byte slice to cadence array
func BytesToCadenceArray(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values)
}

package blueprints

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/model/bootstrap"

	"github.com/onflow/flow-go/module/epochs"

	"github.com/onflow/flow-go/model/flow"
)

const deployIDTableStakingTransactionTemplate = `
transaction {
  prepare(serviceAccount: AuthAccount) {
	serviceAccount.contracts.add(name: "FlowIDTableStaking", code: "%s".decodeHex(), epochTokenPayout: UFix64(%d), rewardCut: UFix64(%d))
  }
}
`

const deployEpochTransactionTemplate = `
import FlowClusterQC from 0x%s

transaction(clusterWeights: [{String: UInt64}]) {
  prepare(serviceAccount: AuthAccount)	{

    // first, construct Cluster objects from cluster weights
    let clusters: [FlowClusterQC.Cluster] = []
    var clusterIndex: UInt16 = 0
    for weightMapping in clusterWeights {
      let cluster = FlowClusterQC.Cluster(clusterIndex, weightMapping)
      clusterIndex = clusterIndex + 1
    }

	serviceAccount.contracts.add(
		name: "FlowEpoch",
		code: "%s".decodeHex(),
		currentEpochCounter: UInt64(%d),
		numViewsInEpoch: UInt64(%d),
		numViewsInStakingAuction: UInt64(%d),
		numViewsInDKGPhase: UInt64(%d),
		numCollectorClusters: UInt16(%d),
		FLOWsupplyIncreasePercentage: UFix64(%d),
		randomSource: %s,
		collectorClusters: clusters,
        // NOTE: clusterQCs and dkgPubKeys are empty because these initial values are not used
		clusterQCs: [],
		dkgPubKeys: [],
	)
  }
}
`

const setupAccountTemplate = `
// This transaction is a template for a transaction
// to add a Vault resource to their account
// so that they can use the flowToken

import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction {

    prepare(signer: AuthAccount) {

        if signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault) == nil {
            // Create a new flowToken Vault and put it in storage
            signer.save(<-FlowToken.createEmptyVault(), to: /storage/flowTokenVault)

            // Create a public capability to the Vault that only exposes
            // the deposit function through the Receiver interface
            signer.link<&FlowToken.Vault{FungibleToken.Receiver}>(
                /public/flowTokenReceiver,
                target: /storage/flowTokenVault
            )

            // Create a public capability to the Vault that only exposes
            // the balance field through the Balance interface
            signer.link<&FlowToken.Vault{FungibleToken.Balance}>(
                /public/flowTokenBalance,
                target: /storage/flowTokenVault
            )
        }
    }
}
`

const fundAccountTemplate = `
import FungibleToken from 0x%s
import FlowToken from 0x%s

transaction(amount: UFix64, recipient: Address) {
	let sentVault: @FungibleToken.Vault
	prepare(signer: AuthAccount) {
	let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
		?? panic("failed to borrow reference to sender vault")
	self.sentVault <- vaultRef.withdraw(amount: amount)
	}
	execute {
	let receiverRef =  getAccount(recipient)
		.getCapability(/public/flowTokenReceiver)
		.borrow<&{FungibleToken.Receiver}>()
		?? panic("failed to borrow reference to recipient vault")
	receiverRef.deposit(from: <-self.sentVault)
	}
}
`

const deployLockedTokensTemplate = `
transaction(publicKeys: [[UInt8]]) {
    
    prepare(admin: AuthAccount) {
        admin.contracts.add(name: "LockedTokens", code: "%s".decodeHex(), admin)

    }
}
`

// DeployEpochTransaction returns the transaction body for the deploy epoch transaction
func DeployEpochTransaction(service flow.Address, contract []byte, epochConfig epochs.EpochConfig) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			deployEpochTransactionTemplate,
			service,
			hex.EncodeToString(contract),
			epochConfig.CurrentEpochCounter,
			epochConfig.NumViewsInEpoch,
			epochConfig.NumViewsInStakingAuction,
			epochConfig.NumViewsInDKGPhase,
			epochConfig.NumCollectorClusters,
			epochConfig.FLOWsupplyIncreasePercentage,
			epochConfig.RandomSource,
		))).
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
		SetScript([]byte(fmt.Sprintf(setupAccountTemplate, fungibleToken, flowToken))).
		AddAuthorizer(accountAddress)
}

// DeployIDTableStakingTransaction returns the transaction body for the deploy id table staking transaction
func DeployIDTableStakingTransaction(service flow.Address, contract []byte, epochTokenPayout cadence.UFix64, rewardCut cadence.UFix64) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			deployIDTableStakingTransactionTemplate,
			hex.EncodeToString(contract),
			epochTokenPayout,
			rewardCut))).
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
		SetScript([]byte(fmt.Sprintf(fundAccountTemplate, fungibleToken, flowToken))).
		AddArgument(jsoncdc.MustEncode(cdcAmount)).
		AddArgument(jsoncdc.MustEncode(cadence.NewAddress(nodeAddress))).
		AddAuthorizer(service)
}

// DeployLockedTokensTransaction returns the transaction body for the deploy locked tokens transaction
func DeployLockedTokensTransaction(service flow.Address, contract []byte, publicKeys []cadence.Value) *flow.TransactionBody {
	return flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(
			deployLockedTokensTemplate,
			hex.EncodeToString(contract),
		))).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(publicKeys))).
		AddAuthorizer(service)
}

// RegisterNodeTransaction creates a new node struct object.
// Then, if the node is a collector node, creates a new account and adds a QC object to it
// If the node is a consensus node, it creates a new account and adds a DKG object to it
func RegisterNodeTransaction(
	service flow.Address,
	flowTokenAddress flow.Address,
	nodeAddress flow.Address,
	id *flow.Identity,
) *flow.TransactionBody {

	env := templates.Environment{
		FlowTokenAddress:         flowTokenAddress.HexWithPrefix(),
		IDTableAddress:           service.HexWithPrefix(),
		QuorumCertificateAddress: service.HexWithPrefix(),
		DkgAddress:               service.HexWithPrefix(),
		EpochAddress:             service.HexWithPrefix(),
	}

	// Use NetworkingKey as the public key of the machine account.
	// We do this for tests/localnet but normally it should be a separate key.
	accountKey := flow.AccountPublicKey{
		PublicKey: id.NetworkPubKey,
		SignAlgo:  id.NetworkPubKey.Algorithm(),
		HashAlgo:  bootstrap.DefaultMachineAccountHashAlgo,
		Weight:    1000,
	}

	encAccountKey, err := flow.EncodeRuntimeAccountPublicKey(accountKey)
	if err != nil {
		panic(err)
	}

	cadencePublicKeys := cadence.NewArray(
		[]cadence.Value{
			BytesToCadenceArray(encAccountKey),
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

	cdcNodeIDs := make([]cadence.Value, 0, len(allowedNodeIDs))
	for _, id := range allowedNodeIDs {
		cdcNodeID, err := cadence.NewString(id.String())
		if err != nil {
			panic(err)
		}
		cdcNodeIDs = append(cdcNodeIDs, cdcNodeID)
	}

	return flow.NewTransactionBody().
		SetScript(templates.GenerateSetApprovedNodesScript(env)).
		AddArgument(jsoncdc.MustEncode(cadence.NewArray(cdcNodeIDs))).
		AddAuthorizer(idTableStakingAddr)
}

// BytesToCadenceArray converts byte slice to cadence array
func BytesToCadenceArray(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values)
}

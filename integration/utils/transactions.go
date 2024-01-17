package utils

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/crypto"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	sdktemplates "github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

//go:embed templates/create-and-setup-node.cdc
var createAndSetupNodeTxScript string

//go:embed templates/remove-node.cdc
var removeNodeTxScript string

func LocalnetEnv() templates.Environment {
	return templates.Environment{
		IDTableAddress:           "f8d6e0586b0a20c7",
		FungibleTokenAddress:     "ee82856bf20e2aa6",
		FlowTokenAddress:         "0ae53cb6e3f42a79",
		LockedTokensAddress:      "f8d6e0586b0a20c7",
		StakingProxyAddress:      "f8d6e0586b0a20c7",
		DkgAddress:               "f8d6e0586b0a20c7",
		QuorumCertificateAddress: "f8d6e0586b0a20c7",
	}
}

// MakeCreateAndSetupNodeTx creates a transaction which creates and configures a new staking account.
// It creates the account, funds it, and registers the node using the staking collection.
func MakeCreateAndSetupNodeTx(
	env templates.Environment,
	service *sdk.Account,
	latestBlockID sdk.Identifier,
	// transaction arguments
	stakingAcctKey *sdk.AccountKey,
	stake string,
	nodeID flow.Identifier,
	role flow.Role,
	networkingAddress string,
	networkingKey string,
	stakingKey string,
	machineKey *sdk.AccountKey,
) (
	*sdk.Transaction,
	error,
) {

	script := []byte(templates.ReplaceAddresses(createAndSetupNodeTxScript, env))
	tx := sdk.NewTransaction().
		SetScript(script).
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(service.Address, 0, service.Keys[0].SequenceNumber).
		AddAuthorizer(service.Address).
		SetPayer(service.Address)

	// 0 - staking account key
	cdcStakingAcctKey, err := sdktemplates.AccountKeyToCadenceCryptoKey(stakingAcctKey)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(cdcStakingAcctKey)
	if err != nil {
		return nil, err
	}

	// 1 - stake
	cdcStake, err := cadence.NewUFix64(stake)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(cdcStake)
	if err != nil {
		return nil, err
	}

	// 2 - node ID
	id, err := cadence.NewString(nodeID.String())
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(id)
	if err != nil {
		return nil, err
	}

	// 3 - role
	r := cadence.NewUInt8(uint8(role))
	err = tx.AddArgument(r)
	if err != nil {
		return nil, err
	}

	// 4 - networking address
	networkingAddressCDC, err := cadence.NewString(networkingAddress)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(networkingAddressCDC)
	if err != nil {
		return nil, err
	}

	// 5 - networking key
	networkingKeyCDC, err := cadence.NewString(networkingKey)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(networkingKeyCDC)
	if err != nil {
		return nil, err
	}

	// 6 - staking key
	stakingKeyCDC, err := cadence.NewString(stakingKey)
	if err != nil {
		return nil, err
	}
	err = tx.AddArgument(stakingKeyCDC)
	if err != nil {
		return nil, err
	}

	if machineKey != nil {
		// for collection/consensus nodes, register the machine account key
		cdcMachineAcctKey, err := sdktemplates.AccountKeyToCadenceCryptoKey(machineKey)
		if err != nil {
			return nil, err
		}
		err = tx.AddArgument(cadence.NewOptional(cdcMachineAcctKey))
		if err != nil {
			return nil, err
		}
	} else {
		// for other nodes, pass nil to avoid registering any machine account key
		err = tx.AddArgument(cadence.NewOptional(nil))
		if err != nil {
			return nil, err
		}
	}

	return tx, nil
}

// MakeAdminRemoveNodeTx makes an admin transaction to remove the node. This transaction both
// manually unstakes and removes the node, and removes it from the allow-list.
func MakeAdminRemoveNodeTx(
	env templates.Environment,
	adminAccount *sdk.Account,
	adminAccountKeyID int,
	latestBlockID sdk.Identifier,
	nodeID flow.Identifier,
) (*sdk.Transaction, error) {
	accountKey := adminAccount.Keys[adminAccountKeyID]
	tx := sdk.NewTransaction().
		SetScript([]byte(templates.ReplaceAddresses(removeNodeTxScript, env))).
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(adminAccount.Address, adminAccountKeyID, accountKey.SequenceNumber).
		SetPayer(adminAccount.Address).
		AddAuthorizer(adminAccount.Address)

	id, _ := cadence.NewString(nodeID.String())
	err := tx.AddArgument(id)
	if err != nil {
		return nil, err
	}

	return tx, nil
}

// submitSmokeTestTransaction will submit a create account transaction to smoke test network
// This ensures a single transaction can be sealed by the network.
func CreateFlowAccount(ctx context.Context, client *testnet.Client) (sdk.Address, error) {
	fullAccountKey := sdk.NewAccountKey().
		SetPublicKey(unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLen).PublicKey()).
		SetHashAlgo(sdkcrypto.SHA2_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	latestBlockID, err := client.GetLatestBlockID(ctx)
	if err != nil {
		return sdk.EmptyAddress, fmt.Errorf("failed to get latest block id: %w", err)
	}

	// createAccount will submit a create account transaction and wait for it to be sealed
	addr, err := client.CreateAccount(ctx, fullAccountKey, sdk.Identifier(latestBlockID))
	if err != nil {
		return sdk.EmptyAddress, fmt.Errorf("failed to create account: %w", err)
	}

	return addr, nil
}

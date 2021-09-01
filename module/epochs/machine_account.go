package epochs

import (
	"bytes"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// Hard and soft balance limits for collection and consensus nodes.
	// We will log a warning once for a soft limit, and will log an error
	// in perpetuity for a hard limit.
	// Taken from https://www.notion.so/dapperlabs/Machine-Account-f3c293593ea442a39614fcebf705a132

	SoftMinBalanceLN cadence.UFix64
	HardMinBalanceLN cadence.UFix64
	SoftMinBalanceSN cadence.UFix64
	HardMinBalanceSN cadence.UFix64
)

func init() {
	var err error
	SoftMinBalanceLN, err = cadence.NewUFix64("0.0025")
	if err != nil {
		panic(fmt.Errorf("could not convert soft min balance for LN: %w", err))
	}
	HardMinBalanceLN, err = cadence.NewUFix64("0.001")
	if err != nil {
		panic(fmt.Errorf("could not convert hard min balance for LN: %w", err))
	}
	SoftMinBalanceSN, err = cadence.NewUFix64("0.125")
	if err != nil {
		panic(fmt.Errorf("could not convert soft min balance for SN: %w", err))
	}
	HardMinBalanceSN, err = cadence.NewUFix64("0.05")
	if err != nil {
		panic(fmt.Errorf("could not convert hard min balance for SN: %w", err))
	}
}

// CheckMachineAccountInfo checks a node machine account config, logging
// anything noteworthy but not critical, and returning an error if the machine
// account is not configured correctly, or the configuration cannot be checked.
func CheckMachineAccountInfo(
	log zerolog.Logger,
	role flow.Role,
	info bootstrap.NodeMachineAccountInfo,
	account *sdk.Account,
) error {

	log.Debug().
		Str("machine_account_address", info.Address).
		Str("role", role.String()).
		Msg("checking machine account configuration...")

	if role != flow.RoleCollection && role != flow.RoleConsensus {
		return fmt.Errorf("invalid role (%s) must be one of [collection, consensus]", role.String())
	}

	address := info.FlowAddress()
	if address == flow.EmptyAddress {
		return fmt.Errorf("could not parse machine account address: %s", info.Address)
	}

	privKey, err := sdkcrypto.DecodePrivateKey(info.SigningAlgorithm, info.EncodedPrivateKey)
	if err != nil {
		return fmt.Errorf("could not decode machine account private key: %w", err)
	}

	// FIRST - check the local account info independently
	if info.HashAlgorithm != bootstrap.DefaultMachineAccountHashAlgo {
		log.Warn().Msgf("non-standard hash algo (expected %s, got %s)", bootstrap.DefaultMachineAccountHashAlgo, info.HashAlgorithm.String())
	}
	if info.SigningAlgorithm != bootstrap.DefaultMachineAccountSignAlgo {
		log.Warn().Msgf("non-standard signing algo (expected %s, got %s)", bootstrap.DefaultMachineAccountSignAlgo, info.SigningAlgorithm.String())
	}
	if info.KeyIndex != bootstrap.DefaultMachineAccountKeyIndex {
		log.Warn().Msgf("non-standard key index (expected %d, got %d)", bootstrap.DefaultMachineAccountKeyIndex, info.KeyIndex)
	}

	// SECOND - compare the local account info to the on-chain account
	if !bytes.Equal(account.Address.Bytes(), address.Bytes()) {
		return fmt.Errorf("machine account address mismatch between local (%s) and on-chain (%s)", address, account.Address)
	}
	if len(account.Keys) <= int(info.KeyIndex) {
		return fmt.Errorf("machine account (%s) has %d keys - but configured with key index %d", account.Address, len(account.Keys), info.KeyIndex)
	}
	accountKey := account.Keys[info.KeyIndex]
	if accountKey.HashAlgo != info.HashAlgorithm {
		return fmt.Errorf("machine account hash algo mismatch between local (%s) and on-chain (%s)",
			info.HashAlgorithm.String(),
			accountKey.HashAlgo.String())
	}
	if accountKey.SigAlgo != info.SigningAlgorithm {
		return fmt.Errorf("machine account signing algo mismatch between local (%s) and on-chain (%s)",
			info.SigningAlgorithm.String(),
			accountKey.SigAlgo.String())
	}
	if accountKey.Index != int(info.KeyIndex) {
		return fmt.Errorf("machine account key index mismatch between local (%d) and on-chain (%d)",
			info.KeyIndex,
			accountKey.Index)
	}
	if !accountKey.PublicKey.Equals(privKey.PublicKey()) {
		return fmt.Errorf("machine account public key mismatch between local and on-chain")
	}

	// THIRD - check that the balance is sufficient
	balance := cadence.UFix64(account.Balance)
	log.Debug().Msgf("machine account balance: %s", balance.String())

	switch role {
	case flow.RoleCollection:
		if balance < HardMinBalanceLN {
			return fmt.Errorf("machine account balance is below hard minimum (%s < %s)", balance, HardMinBalanceLN)
		}
		if balance < SoftMinBalanceLN {
			log.Warn().Msgf("machine account balance is below recommended balance (%s < %s)", balance, SoftMinBalanceLN)
		}
	case flow.RoleConsensus:
		if balance < HardMinBalanceSN {
			return fmt.Errorf("machine account balance is below hard minimum (%s < %s)", balance, HardMinBalanceSN)
		}
		if balance < SoftMinBalanceSN {
			log.Warn().Msgf("machine account balance is below recommended balance (%s < %s)", balance, SoftMinBalanceSN)
		}
	default:
		// sanity check - should be caught earlier in this function
		return fmt.Errorf("invalid role (%s), must be collection or consensus", role)
	}

	return nil
}

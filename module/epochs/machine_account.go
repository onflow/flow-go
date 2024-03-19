package epochs

import (
	"bytes"
	"context"
	"fmt"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"strconv"
	"time"

	"github.com/onflow/cadence"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	sdk "github.com/onflow/flow-go-sdk"
	client "github.com/onflow/flow-go-sdk/access/grpc"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// Hard and soft balance limits for collection and consensus nodes.
	// We will log a warning once for a soft limit, and will log an error
	// in perpetuity for a hard limit.
	// Taken from https://www.notion.so/dapperlabs/Machine-Account-f3c293593ea442a39614fcebf705a132
	// TODO update these for FLIP74

	defaultSoftMinBalanceLN cadence.UFix64
	defaultHardMinBalanceLN cadence.UFix64
	defaultSoftMinBalanceSN cadence.UFix64
	defaultHardMinBalanceSN cadence.UFix64
)

func init() {
	var err error
	defaultSoftMinBalanceLN, err = cadence.NewUFix64("0.0025")
	if err != nil {
		panic(fmt.Errorf("could not convert soft min balance for LN: %w", err))
	}
	defaultHardMinBalanceLN, err = cadence.NewUFix64("0.002")
	if err != nil {
		panic(fmt.Errorf("could not convert hard min balance for LN: %w", err))
	}
	defaultSoftMinBalanceSN, err = cadence.NewUFix64("0.125")
	if err != nil {
		panic(fmt.Errorf("could not convert soft min balance for SN: %w", err))
	}
	defaultHardMinBalanceSN, err = cadence.NewUFix64("0.05")
	if err != nil {
		panic(fmt.Errorf("could not convert hard min balance for SN: %w", err))
	}
}

const (
	checkMachineAccountRetryBase      = time.Second * 30
	checkMachineAccountRetryMax       = time.Minute * 30
	checkMachineAccountRetryJitterPct = 10
)

// checkMachineAccountRetryBackoff returns the default backoff for checking machine account configs.
//   - exponential backoff with base of 30s
//   - maximum inter-check wait of 30
//   - 10% jitter
func checkMachineAccountRetryBackoff() retry.Backoff {
	backoff := retry.NewExponential(checkMachineAccountRetryBase)
	backoff = retry.WithCappedDuration(checkMachineAccountRetryMax, backoff)
	backoff = retry.WithJitterPercent(checkMachineAccountRetryJitterPct, backoff)
	return backoff
}

// MachineAccountValidatorConfig defines configuration options for MachineAccountConfigValidator.
type MachineAccountValidatorConfig struct {
	SoftMinBalanceLN cadence.UFix64
	HardMinBalanceLN cadence.UFix64
	SoftMinBalanceSN cadence.UFix64
	HardMinBalanceSN cadence.UFix64
}

func DefaultMachineAccountValidatorConfig() MachineAccountValidatorConfig {
	return MachineAccountValidatorConfig{
		SoftMinBalanceLN: defaultSoftMinBalanceLN,
		HardMinBalanceLN: defaultHardMinBalanceLN,
		SoftMinBalanceSN: defaultSoftMinBalanceSN,
		HardMinBalanceSN: defaultHardMinBalanceSN,
	}
}

// WithoutBalanceChecks sets minimum balances to 0 to effectively disable minimum
// balance checks. This is useful for test networks where transaction fees are
// disabled.
func WithoutBalanceChecks(conf *MachineAccountValidatorConfig) {
	conf.SoftMinBalanceLN = 0
	conf.HardMinBalanceLN = 0
	conf.SoftMinBalanceSN = 0
	conf.HardMinBalanceSN = 0
}

type MachineAccountValidatorConfigOption func(*MachineAccountValidatorConfig)

// MachineAccountConfigValidator is used to validate that a machine account is
// configured correctly.
type MachineAccountConfigValidator struct {
	config  MachineAccountValidatorConfig
	metrics module.MachineAccountMetrics
	log     zerolog.Logger
	client  *client.Client
	role    flow.Role
	info    bootstrap.NodeMachineAccountInfo

	component.Component
}

func NewMachineAccountConfigValidator(
	log zerolog.Logger,
	flowClient *client.Client,
	role flow.Role,
	info bootstrap.NodeMachineAccountInfo,
	metrics module.MachineAccountMetrics,
	opts ...MachineAccountValidatorConfigOption,
) (*MachineAccountConfigValidator, error) {

	conf := DefaultMachineAccountValidatorConfig()
	for _, apply := range opts {
		apply(&conf)
	}

	validator := &MachineAccountConfigValidator{
		config:  conf,
		log:     log.With().Str("component", "machine_account_config_validator").Logger(),
		client:  flowClient,
		role:    role,
		info:    info,
		metrics: metrics,
	}

	validator.Component = component.NewComponentManagerBuilder().
		AddWorker(validator.reportMachineAccountConfigWorker).
		Build()

	return validator, nil
}

// reportMachineAccountConfigWorker is a worker function that periodically checks
// and reports on the health of the node's configured machine account.
// When a misconfiguration or insufficient account balance is detected, the worker
// will report metrics and log specific information about what is wrong.
//
// This worker runs perpetually in the background, executing once per 30 minutes
// in the steady state. It will execute more frequently right after startup.
func (validator *MachineAccountConfigValidator) reportMachineAccountConfigWorker(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	backoff := checkMachineAccountRetryBackoff()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		err := validator.checkAndReportOnMachineAccountConfig(ctx)
		if err != nil {
			ctx.Throw(err)
		}

		next, _ := backoff.Next()
		t := time.NewTimer(next)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// checkAndReportOnMachineAccountConfig checks the node's machine account for misconfiguration
// or insufficient balance once. Any discovered issues are logged and reported in metrics.
// No errors are expected during normal operation.
func (validator *MachineAccountConfigValidator) checkAndReportOnMachineAccountConfig(ctx context.Context) error {

	account, err := validator.client.GetAccount(ctx, validator.info.SDKAddress())
	if err != nil {
		// we cannot validate a correct configuration - log an error and try again
		validator.log.Error().
			Err(err).
			Str("machine_account_address", validator.info.Address).
			Msg("failed to validate machine account config - could not get machine account")
		return nil
	}

	accountBalance, err := ufix64Tofloat64(cadence.UFix64(account.Balance))
	if err != nil {
		return irrecoverable.NewExceptionf("failed to convert account balance (%d): %w", account.Balance, err)
	}
	validator.metrics.AccountBalance(accountBalance)

	err = CheckMachineAccountInfo(validator.log, validator.config, validator.role, validator.info, account)
	if err != nil {
		// either we cannot validate the configuration or there is a critical
		// misconfiguration - log a warning and retry - we will continue checking
		// and logging until the problem is resolved
		validator.metrics.IsMisconfigured(true)
		validator.log.Error().
			Err(err).
			Msg("critical machine account misconfiguration")
		return nil
	}
	validator.metrics.IsMisconfigured(false)

	return nil
}

// CheckMachineAccountInfo checks a node machine account config, logging
// anything noteworthy but not critical, and returning an error if the machine
// account is not configured correctly, or the configuration cannot be checked.
//
// This function checks most aspects of correct configuration EXCEPT for
// confirming that the account contains the relevant QCVoter or DKGParticipant
// resource. This is omitted because it is not possible to query private account
// info from a script.
func CheckMachineAccountInfo(
	log zerolog.Logger,
	conf MachineAccountValidatorConfig,
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
		if balance < conf.HardMinBalanceLN {
			return fmt.Errorf("machine account balance is below hard minimum (%s < %s)", balance, conf.HardMinBalanceLN)
		}
		if balance < conf.SoftMinBalanceLN {
			log.Warn().Msgf("machine account balance is below recommended balance (%s < %s)", balance, conf.SoftMinBalanceLN)
		}
	case flow.RoleConsensus:
		if balance < conf.HardMinBalanceSN {
			return fmt.Errorf("machine account balance is below hard minimum (%s < %s)", balance, conf.HardMinBalanceSN)
		}
		if balance < conf.SoftMinBalanceSN {
			log.Warn().Msgf("machine account balance is below recommended balance (%s < %s)", balance, conf.SoftMinBalanceSN)
		}
	default:
		// sanity check - should be caught earlier in this function
		return fmt.Errorf("invalid role (%s), must be collection or consensus", role)
	}

	return nil
}

// ufix64Tofloat64 converts a cadence.UFix64 type to float64.
// All UFix64 values should be convertible to float64, so no errors are expected.
func ufix64Tofloat64(fix cadence.UFix64) (float64, error) {
	f, err := strconv.ParseFloat(fix.String(), 64)
	if err != nil {
		return 0, err
	}
	return f, nil
}

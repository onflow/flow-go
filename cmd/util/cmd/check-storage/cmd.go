package check_storage

import (
	"context"
	"fmt"
	"strings"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/interpreter"
	"github.com/onflow/crypto/hash"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	storageState "github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	moduleUtil "github.com/onflow/flow-go/module/util"
)

var (
	flagPayloads           string
	flagState              string
	flagStateCommitment    string
	flagOutputDirectory    string
	flagChain              string
	flagNWorker            int
	flagHasAccountFormatV1 bool
	flagHasAccountFormatV2 bool
	flagIsAccountStatusV4  bool
	flagCheckContract      bool
)

var (
	evmAccount       flow.Address
	evmStorageIDKeys = []string{
		state.AccountsStorageIDKey,
		state.CodesStorageIDKey,
	}
)

var Cmd = &cobra.Command{
	Use:   "check-storage",
	Short: "Check storage health",
	Run:   run,
}

const (
	ReporterName = "storage-health"
)

func init() {

	Cmd.Flags().StringVar(
		&flagPayloads,
		"payloads",
		"",
		"Input payload file name",
	)

	Cmd.Flags().StringVar(
		&flagState,
		"state",
		"",
		"Input state file name",
	)

	Cmd.Flags().StringVar(
		&flagStateCommitment,
		"state-commitment",
		"",
		"Input state commitment",
	)

	Cmd.Flags().StringVar(
		&flagOutputDirectory,
		"output-directory",
		"",
		"Output directory",
	)

	_ = Cmd.MarkFlagRequired("output-directory")

	Cmd.Flags().IntVar(
		&flagNWorker,
		"n-workers",
		10,
		"number of workers to use",
	)

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().BoolVar(
		&flagHasAccountFormatV1,
		"account-format-v1",
		false,
		"State contains accounts in v1 format",
	)

	Cmd.Flags().BoolVar(
		&flagHasAccountFormatV2,
		"account-format-v2",
		true,
		"State contains accounts in v2 format",
	)

	Cmd.Flags().BoolVar(
		&flagIsAccountStatusV4,
		"account-status-v4",
		false,
		"State is migrated to account status v4 format",
	)

	Cmd.Flags().BoolVar(
		&flagCheckContract,
		"check-contract",
		false,
		"check stored contract",
	)
}

func run(*cobra.Command, []string) {

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}
	if !flagHasAccountFormatV1 && !flagHasAccountFormatV2 {
		log.Fatal().Msg("both of or one of --account-format-v1 and --account-format-v2 must be true")
	}

	// Get EVM account by chain
	evmAccount = systemcontracts.SystemContractsForChain(chainID).EVMStorage.Address

	// Create report in JSONL format
	rw := reporters.NewReportFileWriterFactoryWithFormat(flagOutputDirectory, log.Logger, reporters.ReportFormatJSONL).
		ReportWriter(ReporterName)
	defer rw.Close()

	var payloads []*ledger.Payload
	var err error

	// Read payloads from payload file or checkpoint file

	if flagPayloads != "" {
		log.Info().Msgf("Reading payloads from %s", flagPayloads)

		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read payloads")
		}
	} else {
		log.Info().Msgf("Reading trie %s", flagStateCommitment)

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrieForPayloads(flagState, stateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read state")
		}
	}

	log.Info().Msgf("Grouping %d payloads by accounts ...", len(payloads))

	// Group payloads by accounts

	payloadAccountGrouping := util.GroupPayloadsByAccount(log.Logger, payloads, flagNWorker)

	log.Info().Msgf(
		"Creating registers from grouped payloads (%d) ...",
		len(payloads),
	)

	registersByAccount, err := util.NewByAccountRegistersFromPayloadAccountGrouping(payloadAccountGrouping, flagNWorker)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create ByAccount registers from payload")
	}

	accountCount := registersByAccount.AccountCount()

	log.Info().Msgf(
		"Created registers from payloads (%d accounts, %d payloads)",
		accountCount,
		len(payloads),
	)

	failedAccountAddresses, err := checkStorageHealth(registersByAccount, flagNWorker, rw)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to check storage health")
	}

	if len(failedAccountAddresses) == 0 {
		log.Info().Msgf("All %d accounts are healthy", accountCount)
		return
	}

	log.Info().Msgf(
		"%d out of %d accounts reported storage health check issues.  See report %s for more details.",
		len(failedAccountAddresses),
		accountCount,
		ReporterName,
	)

	log.Info().Msgf("Accounts with storage health issues:")
	for _, address := range failedAccountAddresses {
		log.Info().Msgf("  %x", []byte(address))
	}
}

func checkStorageHealth(
	registersByAccount *registers.ByAccount,
	nWorkers int,
	rw reporters.ReportWriter,
) (failedAccountAddresses []string, err error) {

	accountCount := registersByAccount.AccountCount()

	nWorkers = min(accountCount, nWorkers)

	log.Info().Msgf("Checking storage health of %d accounts using %d workers ...", accountCount, nWorkers)

	logAccount := moduleUtil.LogProgress(
		log.Logger,
		moduleUtil.DefaultLogProgressConfig(
			"processing account group",
			accountCount,
		),
	)

	if nWorkers <= 1 {
		// Skip goroutine to avoid overhead
		err = registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				defer logAccount(1)

				accountStorageIssues := checkAccountStorageHealth(accountRegisters, nWorkers)

				if len(accountStorageIssues) > 0 {
					failedAccountAddresses = append(failedAccountAddresses, accountRegisters.Owner())

					for _, issue := range accountStorageIssues {
						rw.Write(issue)
					}
				}

				return nil
			})

		return failedAccountAddresses, err
	}

	type job struct {
		accountRegisters *registers.AccountRegisters
	}

	type result struct {
		owner  string
		issues []accountStorageIssue
	}

	jobs := make(chan job, nWorkers)

	results := make(chan result, nWorkers)

	g, ctx := errgroup.WithContext(context.Background())

	// Launch goroutine to check account storage health
	for i := 0; i < nWorkers; i++ {
		g.Go(func() error {
			for job := range jobs {
				issues := checkAccountStorageHealth(job.accountRegisters, nWorkers)

				result := result{
					owner:  job.accountRegisters.Owner(),
					issues: issues,
				}

				select {
				case results <- result:
				case <-ctx.Done():
					return ctx.Err()
				}

				logAccount(1)
			}
			return nil
		})
	}

	// Launch goroutine to wait for workers to finish and close results (output) channel
	go func() {
		defer close(results)
		err = g.Wait()
	}()

	// Launch goroutine to send job to jobs channel and close jobs (input) channel
	go func() {
		defer close(jobs)

		err = registersByAccount.ForEachAccount(
			func(accountRegisters *registers.AccountRegisters) error {
				jobs <- job{accountRegisters: accountRegisters}
				return nil
			})
		if err != nil {
			log.Err(err).Msgf("failed to iterate accounts by registersByAccount")
		}
	}()

	// Gather results
	for result := range results {
		if len(result.issues) > 0 {
			failedAccountAddresses = append(failedAccountAddresses, result.owner)
			for _, issue := range result.issues {
				rw.Write(issue)
			}
		}
	}

	return failedAccountAddresses, err
}

func checkAccountStorageHealth(accountRegisters *registers.AccountRegisters, nWorkers int) []accountStorageIssue {
	owner := accountRegisters.Owner()

	address, err := common.BytesToAddress([]byte(owner))
	if err != nil {
		return []accountStorageIssue{
			{
				Address: address.Hex(),
				Kind:    storageErrorKindString[otherErrorKind],
				Msg:     err.Error(),
			}}
	}

	if isEVMAccount(address) {
		return checkEVMAccountStorageHealth(address, accountRegisters)
	}

	var issues []accountStorageIssue

	// Check atree storage health

	ledger := &registers.ReadOnlyLedger{Registers: accountRegisters}
	var config runtime.StorageConfig
	storage := runtime.NewStorage(ledger, nil, nil, config)

	// Check account format against specified flags.
	err = checkAccountFormat(
		ledger,
		address,
		flagHasAccountFormatV1,
		flagHasAccountFormatV2,
	)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[storageFormatErrorKind],
				Msg:     err.Error(),
			},
		)
		return issues
	}

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[otherErrorKind],
				Msg:     err.Error(),
			},
		)
		return issues
	}

	err = util.CheckStorageHealth(inter, address, storage, accountRegisters, common.AllStorageDomains, nWorkers)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[cadenceAtreeStorageErrorKind],
				Msg:     err.Error(),
			},
		)
	}

	if flagIsAccountStatusV4 {
		// Validate account public key storage
		err = migrations.ValidateAccountPublicKeyV4(address, accountRegisters)
		if err != nil {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[accountKeyErrorKind],
					Msg:     err.Error(),
				},
			)
		}

		// Check account public keys
		err = checkAccountPublicKeys(address, accountRegisters)
		if err != nil {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[accountKeyErrorKind],
					Msg:     err.Error(),
				},
			)
		}
	}

	if flagCheckContract {
		// Check contracts
		err = checkContracts(address, accountRegisters)
		if err != nil {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[contractErrorKind],
					Msg:     err.Error(),
				},
			)
		}
	}

	return issues
}

type storageErrorKind int

const (
	otherErrorKind storageErrorKind = iota
	cadenceAtreeStorageErrorKind
	evmAtreeStorageErrorKind
	storageFormatErrorKind
	accountKeyErrorKind
	contractErrorKind
)

var storageErrorKindString = map[storageErrorKind]string{
	otherErrorKind:               "error_check_storage_failed",
	cadenceAtreeStorageErrorKind: "error_cadence_atree_storage",
	evmAtreeStorageErrorKind:     "error_evm_atree_storage",
	accountKeyErrorKind:          "error_account_public_key",
	contractErrorKind:            "error_contract",
}

type accountStorageIssue struct {
	Address string
	Kind    string
	Msg     string
}

func hasDomainRegister(ledger atree.Ledger, address common.Address) (bool, error) {
	for _, domain := range common.AllStorageDomains {
		value, err := ledger.GetValue(address[:], []byte(domain.Identifier()))
		if err != nil {
			return false, err
		}
		if len(value) > 0 {
			return true, nil
		}
	}

	return false, nil
}

func hasAccountRegister(ledger atree.Ledger, address common.Address) (bool, error) {
	value, err := ledger.GetValue(address[:], []byte(runtime.AccountStorageKey))
	if err != nil {
		return false, err
	}
	return len(value) > 0, nil
}

func checkAccountFormat(
	ledger atree.Ledger,
	address common.Address,
	expectV1 bool,
	expectV2 bool,
) error {
	// Skip empty address because it doesn't have any account or domain registers.
	if len(address) == 0 || address == common.ZeroAddress {
		return nil
	}

	foundDomainRegister, err := hasDomainRegister(ledger, address)
	if err != nil {
		return err
	}

	foundAccountRegister, err := hasAccountRegister(ledger, address)
	if err != nil {
		return err
	}

	if !foundAccountRegister && !foundDomainRegister {
		return fmt.Errorf("found neither domain nor account registers")
	}

	if foundAccountRegister && foundDomainRegister {
		return fmt.Errorf("found both domain and account registers")
	}

	if foundAccountRegister && !expectV2 {
		return fmt.Errorf("found account in format v2 while only expect account in format v1")
	}

	if foundDomainRegister && !expectV1 {
		return fmt.Errorf("found account in format v1 while only expect account in format v2")
	}

	return nil
}

func checkAccountPublicKeys(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	// Skip empty address because it doesn't have any account status registers.
	if len(address) == 0 || address == common.ZeroAddress {
		return nil
	}

	accounts := newAccounts(accountRegisters)

	keyCount, err := accounts.GetAccountPublicKeyCount(flow.BytesToAddress(address.Bytes()))
	if err != nil {
		return err
	}

	// Check keys
	for keyIndex := range keyCount {
		_, err = accounts.GetAccountPublicKey(flow.BytesToAddress(address.Bytes()), keyIndex)
		if err != nil {
			return err
		}
	}

	// NOTE: don't need to check unreachable keys because it is checked in ValidateAccountPublicKeyV4().

	return nil
}

func checkContracts(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	if len(address) == 0 || address == common.ZeroAddress {
		return nil
	}

	accounts := newAccounts(accountRegisters)

	contractNames, err := accounts.GetContractNames(flow.BytesToAddress(address.Bytes()))
	if err != nil {
		return err
	}

	// Check contract
	contractRegisterKeys := make(map[string]bool)
	for _, contractName := range contractNames {
		contractRegisterKeys[flow.ContractKey(contractName)] = true

		_, err = accounts.GetContract(contractName, flow.BytesToAddress(address.Bytes()))
		if err != nil {
			return err
		}
	}

	// Check unreachable contract registers
	err = accountRegisters.ForEachKey(func(key string) error {
		if strings.HasPrefix(key, flow.CodeKeyPrefix) && !contractRegisterKeys[key] {
			return fmt.Errorf("found unreachable contract register %s, contract names %v", key, contractNames)
		}
		return nil
	})

	return err
}

func newAccounts(accountRegisters *registers.AccountRegisters) environment.Accounts {
	// Create a new transaction state with a dummy hasher
	// because we do not need spock proofs for migrations.
	transactionState := storageState.NewTransactionStateFromExecutionState(
		storageState.NewSpockExecutionStateWithSpockStateHasher(
			registers.StorageSnapshot{
				Registers: accountRegisters,
			},
			storageState.DefaultParameters(),
			func() hash.Hasher {
				return dummyHasher{}
			},
		),
	)
	return environment.NewAccounts(transactionState)
}

type dummyHasher struct{}

func (d dummyHasher) Algorithm() hash.HashingAlgorithm { return hash.UnknownHashingAlgorithm }
func (d dummyHasher) Size() int                        { return 0 }
func (d dummyHasher) ComputeHash([]byte) hash.Hash     { return nil }
func (d dummyHasher) Write([]byte) (int, error)        { return 0, nil }
func (d dummyHasher) SumHash() hash.Hash               { return nil }
func (d dummyHasher) Reset()                           {}

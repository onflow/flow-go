package generate_authorization_fixes

import (
	"compress/gzip"
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"

	common2 "github.com/onflow/flow-go/cmd/util/common"
	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

var (
	flagPayloads            string
	flagState               string
	flagStateCommitment     string
	flagOutputDirectory     string
	flagChain               string
	flagLinkMigrationReport string
	flagAddresses           string
)

var Cmd = &cobra.Command{
	Use:   "generate-authorization-fixes",
	Short: "generate authorization fixes for capability controllers",
	Run:   run,
}

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

	Cmd.Flags().StringVar(
		&flagChain,
		"chain",
		"",
		"Chain name",
	)
	_ = Cmd.MarkFlagRequired("chain")

	Cmd.Flags().StringVar(
		&flagLinkMigrationReport,
		"link-migration-report",
		"",
		"Input link migration report file name",
	)
	_ = Cmd.MarkFlagRequired("link-migration-report")

	Cmd.Flags().StringVar(
		&flagAddresses,
		"addresses",
		"",
		"only generate fixes for given accounts (comma-separated hex-encoded addresses)",
	)
}

func run(*cobra.Command, []string) {

	var addressFilter map[common.Address]struct{}

	if len(flagAddresses) > 0 {
		for _, hexAddr := range strings.Split(flagAddresses, ",") {

			hexAddr = strings.TrimSpace(hexAddr)

			if len(hexAddr) == 0 {
				continue
			}

			addr, err := common2.ParseAddress(hexAddr)
			if err != nil {
				log.Fatal().Err(err).Msgf("failed to parse address: %s", hexAddr)
			}

			if addressFilter == nil {
				addressFilter = make(map[common.Address]struct{})
			}
			addressFilter[common.Address(addr)] = struct{}{}
		}

		addresses := make([]string, 0, len(addressFilter))
		for addr := range addressFilter {
			addresses = append(addresses, addr.HexWithPrefix())
		}
		log.Info().Msgf(
			"Only generating fixes for %d accounts: %s",
			len(addressFilter),
			addresses,
		)
	}

	if flagPayloads == "" && flagState == "" {
		log.Fatal().Msg("Either --payloads or --state must be provided")
	} else if flagPayloads != "" && flagState != "" {
		log.Fatal().Msg("Only one of --payloads or --state must be provided")
	}
	if flagState != "" && flagStateCommitment == "" {
		log.Fatal().Msg("--state-commitment must be provided when --state is provided")
	}

	rwf := reporters.NewReportFileWriterFactory(flagOutputDirectory, log.Logger)

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	migratedPublicLinkSetChan := make(chan MigratedPublicLinkSet, 1)
	go func() {
		migratedPublicLinkSetChan <- readMigratedPublicLinkSet(
			flagLinkMigrationReport,
			addressFilter,
		)
	}()

	registersByAccountChan := make(chan *registers.ByAccount, 1)
	go func() {
		registersByAccountChan <- loadRegistersByAccount()
	}()

	migratedPublicLinkSet := <-migratedPublicLinkSetChan
	registersByAccount := <-registersByAccountChan

	fixReporter := rwf.ReportWriter("authorization-fixes")
	defer fixReporter.Close()

	authorizationFixGenerator := &AuthorizationFixGenerator{
		registersByAccount:    registersByAccount,
		chainID:               chainID,
		migratedPublicLinkSet: migratedPublicLinkSet,
		reporter:              fixReporter,
	}

	log.Info().Msg("Generating authorization fixes ...")

	if len(addressFilter) > 0 {
		authorizationFixGenerator.generateFixesForAccounts(addressFilter)
	} else {
		authorizationFixGenerator.generateFixesForAllAccounts()
	}
}

func loadRegistersByAccount() *registers.ByAccount {
	// Read payloads from payload file or checkpoint file

	var payloads []*ledger.Payload
	var err error

	if flagPayloads != "" {
		log.Info().Msgf("Reading payloads from %s", flagPayloads)

		_, payloads, err = util.ReadPayloadFile(log.Logger, flagPayloads)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read payloads")
		}
	} else {
		log.Info().Msgf("Reading trie %s", flagStateCommitment)

		stateCommitment := util.ParseStateCommitment(flagStateCommitment)
		payloads, err = util.ReadTrie(flagState, stateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to read state")
		}
	}

	log.Info().Msgf("creating registers from payloads (%d)", len(payloads))

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	if err != nil {
		log.Fatal().Err(err)
	}
	log.Info().Msgf(
		"created %d registers from payloads (%d accounts)",
		registersByAccount.Count(),
		registersByAccount.AccountCount(),
	)

	return registersByAccount
}

func readMigratedPublicLinkSet(path string, addressFilter map[common.Address]struct{}) MigratedPublicLinkSet {

	file, err := os.Open(path)
	if err != nil {
		log.Fatal().Err(err).Msgf("can't open link migration report: %s", path)
	}
	defer file.Close()

	var reader io.Reader = file
	if isGzip(file) {
		reader, err = gzip.NewReader(file)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to create gzip reader for %s", path)
		}
	}

	log.Info().Msgf("Reading link migration report from %s ...", path)

	migratedPublicLinkSet, err := ReadMigratedPublicLinkSet(reader, addressFilter)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read public link report: %s", path)
	}

	log.Info().Msgf("Read %d public link migration entries", len(migratedPublicLinkSet))

	return migratedPublicLinkSet
}

func jsonEncodeAuthorization(authorization interpreter.Authorization) string {
	switch authorization {
	case interpreter.UnauthorizedAccess, interpreter.InaccessibleAccess:
		return ""
	default:
		return string(authorization.ID())
	}
}

type fixEntitlementsEntry struct {
	CapabilityAddress common.Address
	CapabilityID      uint64
	ReferencedType    interpreter.StaticType
	Authorization     interpreter.Authorization
}

var _ json.Marshaler = fixEntitlementsEntry{}

func (e fixEntitlementsEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
		ReferencedType    string `json:"referenced_type"`
		Authorization     string `json:"authorization"`
	}{
		CapabilityAddress: e.CapabilityAddress.String(),
		CapabilityID:      e.CapabilityID,
		ReferencedType:    string(e.ReferencedType.ID()),
		Authorization:     jsonEncodeAuthorization(e.Authorization),
	})
}

type AuthorizationFixGenerator struct {
	registersByAccount    *registers.ByAccount
	chainID               flow.ChainID
	migratedPublicLinkSet MigratedPublicLinkSet
	reporter              reporters.ReportWriter
}

func (g *AuthorizationFixGenerator) generateFixesForAllAccounts() {
	var wg sync.WaitGroup
	progress := progressbar.Default(int64(g.registersByAccount.AccountCount()), "Processing:")

	err := g.registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {
		address := common.MustBytesToAddress([]byte(accountRegisters.Owner()))
		wg.Add(1)
		go func(address common.Address) {
			defer wg.Done()
			g.generateFixesForAccount(address)
			_ = progress.Add(1)
		}(address)
		return nil
	})
	if err != nil {
		log.Fatal().Err(err)
	}

	wg.Wait()
	_ = progress.Finish()
}

func (g *AuthorizationFixGenerator) generateFixesForAccounts(addresses map[common.Address]struct{}) {
	var wg sync.WaitGroup
	progress := progressbar.Default(int64(len(addresses)), "Processing:")

	for address := range addresses {
		wg.Add(1)
		go func(address common.Address) {
			defer wg.Done()
			g.generateFixesForAccount(address)
			_ = progress.Add(1)
		}(address)
	}

	wg.Wait()
	_ = progress.Finish()
}

func (g *AuthorizationFixGenerator) generateFixesForAccount(address common.Address) {
	mr, err := migrations.NewInterpreterMigrationRuntime(
		g.registersByAccount,
		g.chainID,
		migrations.InterpreterMigrationRuntimeConfig{},
	)
	if err != nil {
		log.Fatal().Err(err)
	}

	capabilityControllerStorage := mr.Storage.GetStorageMap(
		address,
		stdlib.CapabilityControllerStorageDomain,
		false,
	)
	if capabilityControllerStorage == nil {
		return
	}

	iterator := capabilityControllerStorage.Iterator(nil)
	for {
		k, v := iterator.Next()

		if k == nil || v == nil {
			break
		}

		key, ok := k.(interpreter.Uint64AtreeValue)
		if !ok {
			log.Fatal().Msgf("unexpected key type: %T", k)
		}

		capabilityID := uint64(key)

		value := interpreter.MustConvertUnmeteredStoredValue(v)

		capabilityController, ok := value.(*interpreter.StorageCapabilityControllerValue)
		if !ok {
			continue
		}

		borrowType := capabilityController.BorrowType

		switch borrowType.Authorization.(type) {
		case interpreter.EntitlementSetAuthorization:
			g.maybeGenerateFixForEntitledCapabilityController(
				address,
				capabilityID,
				borrowType,
			)

		case interpreter.Unauthorized:
			// Already unauthorized, nothing to do

		case interpreter.Inaccessible:
			log.Warn().Msgf(
				"capability controller %d in account %s has borrow type with inaccessible authorization",
				capabilityID,
				address.HexWithPrefix(),
			)

		case interpreter.EntitlementMapAuthorization:
			log.Warn().Msgf(
				"capability controller %d in account %s has borrow type with entitlement map authorization",
				capabilityID,
				address.HexWithPrefix(),
			)

		default:
			log.Warn().Msgf(
				"capability controller %d in account %s has borrow type with entitlement map authorization",
				capabilityID,
				address.HexWithPrefix(),
			)
		}
	}
}

func newEntitlementSetAuthorizationFromTypeIDs(
	typeIDs []common.TypeID,
	setKind sema.EntitlementSetKind,
) interpreter.EntitlementSetAuthorization {
	return interpreter.NewEntitlementSetAuthorization(
		nil,
		func() []common.TypeID {
			return typeIDs
		},
		len(typeIDs),
		setKind,
	)
}

func (g *AuthorizationFixGenerator) maybeGenerateFixForEntitledCapabilityController(
	capabilityAddress common.Address,
	capabilityID uint64,
	borrowType *interpreter.ReferenceStaticType,
) {
	// Only remove the authorization if the capability controller was migrated from a public link
	_, ok := g.migratedPublicLinkSet[AccountCapabilityID{
		Address:      capabilityAddress,
		CapabilityID: capabilityID,
	}]
	if !ok {
		return
	}

	g.reporter.Write(fixEntitlementsEntry{
		CapabilityAddress: capabilityAddress,
		CapabilityID:      capabilityID,
		ReferencedType:    borrowType.ReferencedType,
		Authorization:     borrowType.Authorization,
	})
}

func isGzip(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".gz")
}

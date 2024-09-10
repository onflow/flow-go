package generate_authorization_fixes

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
	"github.com/onflow/cadence/runtime/stdlib"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

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
	flagPublicLinkReport    string
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
		&flagPublicLinkReport,
		"public-link-report",
		"",
		"Input public link report file name",
	)
	_ = Cmd.MarkFlagRequired("public-link-report")

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

const contractCountEstimate = 1000

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

	reporter := rwf.ReportWriter("entitlement-fixes")
	defer reporter.Close()

	chainID := flow.ChainID(flagChain)
	// Validate chain ID
	_ = chainID.Chain()

	publicLinkReportChan := make(chan PublicLinkReport, 1)
	go func() {
		publicLinkReportChan <- readPublicLinkReport(addressFilter)
	}()

	publicLinkMigrationReportChan := make(chan PublicLinkMigrationReport, 1)
	go func() {
		publicLinkMigrationReportChan <- readLinkMigrationReport(addressFilter)
	}()

	publicLinkReport := <-publicLinkReportChan
	publicLinkMigrationReport := <-publicLinkMigrationReportChan

	registersByAccount := loadRegistersByAccount()

	mr, err := migrations.NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		migrations.InterpreterMigrationRuntimeConfig{},
	)
	if err != nil {
		log.Fatal().Err(err)
	}

	checkContracts(
		registersByAccount,
		mr,
		reporter,
	)

	authorizationFixGenerator := &AuthorizationFixGenerator{
		registersByAccount:        registersByAccount,
		mr:                        mr,
		publicLinkReport:          publicLinkReport,
		publicLinkMigrationReport: publicLinkMigrationReport,
		reporter:                  reporter,
	}
	authorizationFixGenerator.generateFixesForAllAccounts()
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

func readPublicLinkReport(addressFilter map[common.Address]struct{}) PublicLinkReport {
	// Read public link report

	publicLinkReportFile, err := os.Open(flagPublicLinkReport)
	if err != nil {
		log.Fatal().Err(err).Msgf("can't open link report: %s", flagPublicLinkReport)
	}
	defer publicLinkReportFile.Close()

	var publicLinkReportReader io.Reader = publicLinkReportFile
	if isGzip(publicLinkReportFile) {
		publicLinkReportReader, err = gzip.NewReader(publicLinkReportFile)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to create gzip reader for %s", flagPublicLinkReport)
		}
	}

	log.Info().Msgf("Reading public link report from %s ...", flagPublicLinkReport)

	publicLinkReport, err := ReadPublicLinkReport(publicLinkReportReader, addressFilter)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read public link report %s", flagPublicLinkReport)
	}

	log.Info().Msgf("Read %d public link entries", len(publicLinkReport))

	return publicLinkReport
}

func readLinkMigrationReport(addressFilter map[common.Address]struct{}) PublicLinkMigrationReport {
	// Read link migration report

	linkMigrationReportFile, err := os.Open(flagLinkMigrationReport)
	if err != nil {
		log.Fatal().Err(err).Msgf("can't open link migration report: %s", flagLinkMigrationReport)
	}
	defer linkMigrationReportFile.Close()

	var linkMigrationReportReader io.Reader = linkMigrationReportFile
	if isGzip(linkMigrationReportFile) {
		linkMigrationReportReader, err = gzip.NewReader(linkMigrationReportFile)
		if err != nil {
			log.Fatal().Err(err).Msgf("failed to create gzip reader for %s", flagLinkMigrationReport)
		}
	}

	log.Info().Msgf("Reading link migration report from %s ...", flagLinkMigrationReport)

	publicLinkMigrationReport, err := ReadPublicLinkMigrationReport(linkMigrationReportReader, addressFilter)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to read public link report: %s", flagLinkMigrationReport)
	}

	log.Info().Msgf("Read %d public link migration entries", len(publicLinkMigrationReport))

	return publicLinkMigrationReport
}

func checkContracts(
	registersByAccount *registers.ByAccount,
	mr *migrations.InterpreterMigrationRuntime,
	reporter reporters.ReportWriter,
) {
	contracts, err := migrations.GatherContractsFromRegisters(registersByAccount, log.Logger)
	if err != nil {
		log.Fatal().Err(err)
	}

	programs := make(map[common.Location]*interpreter.Program, contractCountEstimate)

	contractsForPrettyPrinting := make(map[common.Location][]byte, len(contracts))
	for _, contract := range contracts {
		contractsForPrettyPrinting[contract.Location] = contract.Code
	}

	log.Info().Msg("Checking contracts ...")

	for _, contract := range contracts {
		migrations.CheckContract(
			contract,
			log.Logger,
			mr,
			contractsForPrettyPrinting,
			false,
			reporter,
			nil,
			programs,
		)
	}

	log.Info().Msgf("Checked %d contracts ...", len(contracts))
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
	AccountCapabilityID
	NewAuthorization  interpreter.Authorization
	UnresolvedMembers map[string]error
}

var _ json.Marshaler = fixEntitlementsEntry{}

func (e fixEntitlementsEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Kind              string `json:"kind"`
		CapabilityAddress string `json:"capability_address"`
		CapabilityID      uint64 `json:"capability_id"`
		NewAuthorization  string `json:"new_authorization"`
		UnresolvedMembers map[string]string
	}{
		Kind:              "fix-entitlements",
		CapabilityAddress: e.Address.String(),
		CapabilityID:      e.CapabilityID,
		NewAuthorization:  jsonEncodeAuthorization(e.NewAuthorization),
		UnresolvedMembers: jsonEncodeMemberErrorMap(e.UnresolvedMembers),
	})
}

func jsonEncodeMemberErrorMap(m map[string]error) map[string]string {
	result := make(map[string]string, len(m))
	for key, value := range m {
		result[key] = value.Error()
	}
	return result
}

type AuthorizationFixGenerator struct {
	registersByAccount        *registers.ByAccount
	mr                        *migrations.InterpreterMigrationRuntime
	publicLinkReport          PublicLinkReport
	publicLinkMigrationReport PublicLinkMigrationReport
	reporter                  reporters.ReportWriter
}

func (g *AuthorizationFixGenerator) generateFixesForAllAccounts() {
	err := g.registersByAccount.ForEachAccount(func(accountRegisters *registers.AccountRegisters) error {
		address := common.MustBytesToAddress([]byte(accountRegisters.Owner()))
		g.generateFixesForAccount(address)
		return nil
	})
	if err != nil {
		log.Fatal().Err(err)
	}
}

func (g *AuthorizationFixGenerator) generateFixesForAccount(address common.Address) {
	capabilityControllerStorage := g.mr.Storage.GetStorageMap(
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
			g.maybeGenerateFixForCapabilityController(
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

func (g *AuthorizationFixGenerator) maybeGenerateFixForCapabilityController(
	capabilityAddress common.Address,
	capabilityID uint64,
	borrowType *interpreter.ReferenceStaticType,
) {
	// Only fix the entitlements if the capability controller was migrated from a public link
	publicPathIdentifier := g.capabilityControllerPublicPathIdentifier(capabilityAddress, capabilityID)
	if publicPathIdentifier == "" {
		return
	}

	linkInfo := g.publicPathLinkInfo(capabilityAddress, publicPathIdentifier)
	if linkInfo.BorrowType == "" {
		log.Warn().Msgf(
			"missing link info for /public/%s in account %s",
			publicPathIdentifier,
			capabilityAddress.HexWithPrefix(),
		)
		return
	}

	// Compare previously accessible members with new accessible members.
	// They should be the same.

	oldAccessibleMembers := linkInfo.AccessibleMembers
	if oldAccessibleMembers == nil {
		log.Warn().Msgf(
			"missing old accessible members for for /public/%s in account %s",
			publicPathIdentifier,
			capabilityAddress.HexWithPrefix(),
		)
		return
	}

	// Assume we already had access to public built-in functions.
	// For example, forEachAttachment was added in Cadence 1.0,
	// so we should not consider it as a new member.

	oldAccessibleMembers = append(
		[]string{"getType", "isInstance", "forEachAttachment"},
		oldAccessibleMembers...,
	)

	semaBorrowType, err := convertStaticToSemaType(g.mr.Interpreter, borrowType)
	if err != nil {
		log.Warn().Err(err).Msgf(
			"failed to get new accessible members for capability controller %d in account %s",
			capabilityID,
			capabilityAddress.HexWithPrefix(),
		)
		return
	}

	newAccessibleMembers := getAccessibleMembers(semaBorrowType)

	sort.Strings(oldAccessibleMembers)
	sort.Strings(newAccessibleMembers)

	if slices.Equal(oldAccessibleMembers, newAccessibleMembers) {
		// Nothing to fix
		return
	}

	log.Info().Msgf(
		"member mismatch for capability controller %d in account %s: expected %v, got %v",
		capabilityID,
		capabilityAddress.HexWithPrefix(),
		oldAccessibleMembers,
		newAccessibleMembers,
	)

	oldAccessibleMemberSet := make(map[string]struct{})
	for _, memberName := range oldAccessibleMembers {
		oldAccessibleMemberSet[memberName] = struct{}{}
	}

	newAuthorization, unresolvedMembers := findMinimalAuthorization(
		semaBorrowType,
		oldAccessibleMemberSet,
	)

	if len(unresolvedMembers) > 0 {
		// TODO: format unresolved members
		log.Warn().Msgf(
			"failed to find minimal entitlement set for capability controller %d in account %s: unresolved members: %v",
			capabilityID,
			capabilityAddress.HexWithPrefix(),
			unresolvedMembers,
		)

		// NOTE: still continue with the fix,
		// we should not leave the capability controller vulnerable
	}

	g.reporter.Write(fixEntitlementsEntry{
		AccountCapabilityID: AccountCapabilityID{
			Address:      capabilityAddress,
			CapabilityID: capabilityID,
		},
		NewAuthorization:  newAuthorization,
		UnresolvedMembers: unresolvedMembers,
	})

}

func convertStaticToSemaType(
	inter *interpreter.Interpreter,
	staticType interpreter.StaticType,
) (
	semaType sema.Type,
	err error,
) {

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	semaType, err = inter.ConvertStaticToSemaType(staticType)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to convert static type %s to semantic type: %w",
			staticType.ID(),
			err,
		)
	}
	if semaType == nil {
		return nil, fmt.Errorf(
			"failed to convert static type %s to semantic type",
			staticType.ID(),
		)
	}

	return semaType, nil
}

func (g *AuthorizationFixGenerator) capabilityControllerPublicPathIdentifier(
	address common.Address,
	capabilityID uint64,
) string {
	return g.publicLinkMigrationReport[AccountCapabilityID{
		Address:      address,
		CapabilityID: capabilityID,
	}]
}

func (g *AuthorizationFixGenerator) publicPathLinkInfo(
	address common.Address,
	publicPathIdentifier string,
) LinkInfo {
	return g.publicLinkReport[AddressPublicPath{
		Address:    address,
		Identifier: publicPathIdentifier,
	}]
}

func isGzip(file *os.File) bool {
	return strings.HasSuffix(file.Name(), ".gz")
}

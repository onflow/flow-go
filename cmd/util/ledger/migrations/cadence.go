package migrations

import (
	"context"
	_ "embed"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
)

func NewInterfaceTypeConversionRules(chainID flow.ChainID) StaticTypeMigrationRules {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	oldFungibleTokenResolverType, newFungibleTokenResolverType :=
		newFungibleTokenMetadataViewsToViewResolverRule(systemContracts, "Resolver")

	oldFungibleTokenResolverCollectionType, newFungibleTokenResolverCollectionType :=
		newFungibleTokenMetadataViewsToViewResolverRule(systemContracts, "ResolverCollection")

	oldNonFungibleTokenINFTType, newNonFungibleTokenNFTType :=
		nonFungibleTokenInterfaceToInterfaceRule(systemContracts, "INFT", "NFT")

	return StaticTypeMigrationRules{
		oldFungibleTokenResolverType.ID():           newFungibleTokenResolverType,
		oldFungibleTokenResolverCollectionType.ID(): newFungibleTokenResolverCollectionType,
		oldNonFungibleTokenINFTType.ID():            newNonFungibleTokenNFTType,
	}
}

func NewCompositeTypeConversionRules(chainID flow.ChainID) StaticTypeMigrationRules {
	systemContracts := systemcontracts.SystemContractsForChain(chainID)

	oldFungibleTokenVaultCompositeType, newFungibleTokenVaultType :=
		fungibleTokenRule(systemContracts, "Vault")
	oldNonFungibleTokenNFTCompositeType, newNonFungibleTokenNFTType :=
		nonFungibleTokenCompositeToInterfaceRule(systemContracts, "NFT")
	oldNonFungibleTokenCollectionCompositeType, newNonFungibleTokenCollectionType :=
		nonFungibleTokenCompositeToInterfaceRule(systemContracts, "Collection")

	return StaticTypeMigrationRules{
		oldFungibleTokenVaultCompositeType.ID():         newFungibleTokenVaultType,
		oldNonFungibleTokenNFTCompositeType.ID():        newNonFungibleTokenNFTType,
		oldNonFungibleTokenCollectionCompositeType.ID(): newNonFungibleTokenCollectionType,
	}
}

func NewCadence1InterfaceStaticTypeConverter(chainID flow.ChainID) statictypes.InterfaceTypeConverterFunc {
	rules := NewInterfaceTypeConversionRules(chainID)
	return NewStaticTypeMigration[*interpreter.InterfaceStaticType](rules)
}

func NewCadence1CompositeStaticTypeConverter(chainID flow.ChainID) statictypes.CompositeTypeConverterFunc {
	rules := NewCompositeTypeConversionRules(chainID)
	return NewStaticTypeMigration[*interpreter.CompositeStaticType](rules)
}

func nonFungibleTokenCompositeToInterfaceRule(
	systemContracts *systemcontracts.SystemContracts,
	identifier string,
) (
	*interpreter.CompositeStaticType,
	*interpreter.IntersectionStaticType,
) {
	contract := systemContracts.NonFungibleToken

	qualifiedIdentifier := fmt.Sprintf("%s.%s", contract.Name, identifier)

	location := common.AddressLocation{
		Address: common.Address(contract.Address),
		Name:    contract.Name,
	}

	nftTypeID := location.TypeID(nil, qualifiedIdentifier)

	oldType := &interpreter.CompositeStaticType{
		Location:            location,
		QualifiedIdentifier: qualifiedIdentifier,
		TypeID:              nftTypeID,
	}

	newType := &interpreter.IntersectionStaticType{
		Types: []*interpreter.InterfaceStaticType{
			{
				Location:            location,
				QualifiedIdentifier: qualifiedIdentifier,
				TypeID:              nftTypeID,
			},
		},
	}

	return oldType, newType
}

func nonFungibleTokenInterfaceToInterfaceRule(
	systemContracts *systemcontracts.SystemContracts,
	oldIdentifier string,
	newIdentifier string,
) (
	*interpreter.InterfaceStaticType,
	*interpreter.InterfaceStaticType,
) {
	contract := systemContracts.NonFungibleToken

	oldQualifiedIdentifier := fmt.Sprintf("%s.%s", contract.Name, oldIdentifier)
	newQualifiedIdentifier := fmt.Sprintf("%s.%s", contract.Name, newIdentifier)

	location := common.AddressLocation{
		Address: common.Address(contract.Address),
		Name:    contract.Name,
	}

	oldTypeID := location.TypeID(nil, oldQualifiedIdentifier)
	newTypeID := location.TypeID(nil, newQualifiedIdentifier)

	oldType := &interpreter.InterfaceStaticType{
		Location:            location,
		QualifiedIdentifier: oldQualifiedIdentifier,
		TypeID:              oldTypeID,
	}

	newType := &interpreter.InterfaceStaticType{
		Location:            location,
		QualifiedIdentifier: newQualifiedIdentifier,
		TypeID:              newTypeID,
	}

	return oldType, newType
}

func fungibleTokenRule(
	systemContracts *systemcontracts.SystemContracts,
	identifier string,
) (
	*interpreter.CompositeStaticType,
	*interpreter.IntersectionStaticType,
) {
	contract := systemContracts.FungibleToken

	qualifiedIdentifier := fmt.Sprintf("%s.%s", contract.Name, identifier)

	location := common.AddressLocation{
		Address: common.Address(contract.Address),
		Name:    contract.Name,
	}

	vaultTypeID := location.TypeID(nil, qualifiedIdentifier)

	oldType := &interpreter.CompositeStaticType{
		Location:            location,
		QualifiedIdentifier: qualifiedIdentifier,
		TypeID:              vaultTypeID,
	}

	newType := &interpreter.IntersectionStaticType{
		Types: []*interpreter.InterfaceStaticType{
			{
				Location:            location,
				QualifiedIdentifier: qualifiedIdentifier,
				TypeID:              vaultTypeID,
			},
		},
	}

	return oldType, newType
}

func newFungibleTokenMetadataViewsToViewResolverRule(
	systemContracts *systemcontracts.SystemContracts,
	typeName string,
) (
	*interpreter.InterfaceStaticType,
	*interpreter.InterfaceStaticType,
) {
	oldContract := systemContracts.MetadataViews
	newContract := systemContracts.ViewResolver

	oldLocation := common.AddressLocation{
		Address: common.Address(oldContract.Address),
		Name:    oldContract.Name,
	}

	newLocation := common.AddressLocation{
		Address: common.Address(newContract.Address),
		Name:    newContract.Name,
	}

	oldQualifiedIdentifier := fmt.Sprintf("%s.%s", oldContract.Name, typeName)
	newQualifiedIdentifier := fmt.Sprintf("%s.%s", newContract.Name, typeName)

	oldType := &interpreter.InterfaceStaticType{
		Location:            oldLocation,
		QualifiedIdentifier: oldQualifiedIdentifier,
		TypeID:              oldLocation.TypeID(nil, oldQualifiedIdentifier),
	}

	newType := &interpreter.InterfaceStaticType{
		Location:            newLocation,
		QualifiedIdentifier: newQualifiedIdentifier,
		TypeID:              newLocation.TypeID(nil, newQualifiedIdentifier),
	}

	return oldType, newType
}

type NamedMigration struct {
	Name    string
	Migrate RegistersMigration
}

type IssueStorageCapConMigration struct {
	name                              string
	chainID                           flow.ChainID
	accountsCapabilities              *capcons.AccountsCapabilities
	interpreterMigrationRuntimeConfig InterpreterMigrationRuntimeConfig
	programs                          map[runtime.Location]*interpreter.Program
	typedCapabilityMapping            *capcons.PathTypeCapabilityMapping
	untypedCapabilityMapping          *capcons.PathCapabilityMapping
	reporter                          reporters.ReportWriter
	logVerboseDiff                    bool
	verboseErrorOutput                bool
	errorMessageHandler               *errorMessageHandler
	log                               zerolog.Logger
}

const issueStorageCapConMigrationReporterName = "cadence-storage-capcon-issue-migration"

func NewIssueStorageCapConMigration(
	rwf reporters.ReportWriterFactory,
	errorMessageHandler *errorMessageHandler,
	chainID flow.ChainID,
	storageDomainCapabilities *capcons.AccountsCapabilities,
	programs map[runtime.Location]*interpreter.Program,
	typedStorageCapabilityMapping *capcons.PathTypeCapabilityMapping,
	untypedStorageCapabilityMapping *capcons.PathCapabilityMapping,
	opts Options,
) *IssueStorageCapConMigration {
	return &IssueStorageCapConMigration{
		name:                     "cadence_storage_cap_con_issue_migration",
		reporter:                 rwf.ReportWriter(issueStorageCapConMigrationReporterName),
		chainID:                  chainID,
		accountsCapabilities:     storageDomainCapabilities,
		programs:                 programs,
		typedCapabilityMapping:   typedStorageCapabilityMapping,
		untypedCapabilityMapping: untypedStorageCapabilityMapping,
		logVerboseDiff:           opts.LogVerboseDiff,
		verboseErrorOutput:       opts.VerboseErrorOutput,
		errorMessageHandler:      errorMessageHandler,
	}
}

func (m *IssueStorageCapConMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("migration", m.name).Logger()

	// During the migration, we only provide already checked programs,
	// no parsing/checking of contracts is expected.

	m.interpreterMigrationRuntimeConfig = InterpreterMigrationRuntimeConfig{
		GetOrLoadProgram: func(
			location runtime.Location,
			_ func() (*interpreter.Program, error),
		) (*interpreter.Program, error) {
			program, ok := m.programs[location]
			if !ok {
				return nil, fmt.Errorf("program not found: %s", location)
			}
			return program, nil
		},
		GetCode: func(_ common.AddressLocation) ([]byte, error) {
			return nil, fmt.Errorf("unexpected call to GetCode")
		},
		GetContractNames: func(address flow.Address) ([]string, error) {
			return nil, fmt.Errorf("unexpected call to GetContractNames")
		},
	}

	return nil
}

func (m *IssueStorageCapConMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {
	accountCapabilities := m.accountsCapabilities.Get(address)
	if accountCapabilities == nil {
		return nil
	}

	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		accountRegisters,
		m.chainID,
		m.interpreterMigrationRuntimeConfig,
	)
	if err != nil {
		return fmt.Errorf("failed to create interpreter migration runtime: %w", err)
	}

	idGenerator := environment.NewAccountLocalIDGenerator(
		tracing.NewMockTracerSpan(),
		util.NopMeter{},
		migrationRuntime.Accounts,
	)

	handler := capabilityControllerHandler{
		idGenerator: idGenerator,
	}

	reporter := newValueMigrationReporter(
		m.reporter,
		m.log,
		m.errorMessageHandler,
		m.verboseErrorOutput,
	)

	inter := migrationRuntime.Interpreter

	capcons.IssueAccountCapabilities(
		inter,
		migrationRuntime.Storage,
		reporter,
		address,
		accountCapabilities,
		handler,
		m.typedCapabilityMapping,
		m.untypedCapabilityMapping,
		func(valueType interpreter.StaticType) interpreter.Authorization {
			// TODO:
			return interpreter.UnauthorizedAccess
		},
	)

	err = migrationRuntime.Storage.NondeterministicCommit(inter, false)
	if err != nil {
		return fmt.Errorf("failed to commit changes: %w", err)
	}

	// finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes into the registers
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply changes: %w", err)
	}

	return nil
}

func (m *IssueStorageCapConMigration) Close() error {
	m.reporter.Close()
	return nil
}

var _ AccountBasedMigration = &IssueStorageCapConMigration{}

func NewCadence1ValueMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	importantLocations map[common.AddressLocation]struct{},
	opts Options,
) (migs []NamedMigration) {

	// Populated by CadenceLinkValueMigration,
	// used by CadenceCapabilityValueMigration
	privatePublicCapabilityMapping := &capcons.PathCapabilityMapping{}
	// Populated by IssueStorageCapConMigration
	// used by CadenceCapabilityValueMigration
	typedStorageCapabilityMapping := &capcons.PathTypeCapabilityMapping{}
	untypedStorageCapabilityMapping := &capcons.PathCapabilityMapping{}

	// Populated by StorageCapMigration,
	// used by IssueStorageCapConMigration
	storageDomainCapabilities := &capcons.AccountsCapabilities{}

	errorMessageHandler := &errorMessageHandler{}

	// The value migrations are run as account-based migrations,
	// i.e. the migrations are only given the payloads for the account to be migrated.
	// However, the migrations need to be able to get the code for contracts of any account.
	//
	// To achieve this, the contracts are extracted from the payloads once,
	// before the value migrations are run.

	programs := make(map[common.Location]*interpreter.Program, 1000)

	migs = []NamedMigration{
		{
			Name: "cleanup-contracts",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewContractCleanupMigration(rwf),
				},
			),
		},
		{
			Name: "check-contracts",
			Migrate: NewContractCheckingMigration(
				log,
				rwf,
				opts.ChainID,
				opts.VerboseErrorOutput,
				importantLocations,
				programs,
			),
		},
	}

	for index, migrationConstructor := range []func(opts Options) (string, AccountBasedMigration){
		func(opts Options) (string, AccountBasedMigration) {
			migration := NewCadence1ValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				NewCadence1CompositeStaticTypeConverter(opts.ChainID),
				NewCadence1InterfaceStaticTypeConverter(opts.ChainID),
				storageDomainCapabilities,
				opts,
			)
			return migration.name, migration
		},
		func(opts Options) (string, AccountBasedMigration) {
			migration := NewIssueStorageCapConMigration(
				rwf,
				errorMessageHandler,
				opts.ChainID,
				storageDomainCapabilities,
				programs,
				typedStorageCapabilityMapping,
				untypedStorageCapabilityMapping,
				opts,
			)
			return migration.name, migration

		},
		func(opts Options) (string, AccountBasedMigration) {
			migration := NewCadence1LinkValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				privatePublicCapabilityMapping,
				opts,
			)
			return migration.name, migration
		},
		func(opts Options) (string, AccountBasedMigration) {
			migration := NewCadence1CapabilityValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				privatePublicCapabilityMapping,
				typedStorageCapabilityMapping,
				untypedStorageCapabilityMapping,
				opts,
			)
			return migration.name, migration
		},
	} {
		opts := opts
		// Only check storage health before the first migration
		opts.CheckStorageHealthBeforeMigration = opts.CheckStorageHealthBeforeMigration && index == 0

		name, accountBasedMigration := migrationConstructor(opts)

		migs = append(
			migs,
			NamedMigration{
				Name: name,
				Migrate: NewAccountBasedMigration(
					log,
					opts.NWorker,
					[]AccountBasedMigration{
						accountBasedMigration,
					},
				),
			},
		)
	}

	if opts.ReportMetrics {
		migs = append(migs, NamedMigration{
			Name: metricsCollectingMigrationName,
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				[]AccountBasedMigration{
					NewMetricsCollectingMigration(
						log,
						opts.ChainID,
						rwf,
						programs,
					),
				},
			),
		})
	}

	return
}

const stagedContractUpdateMigrationName = "staged-contracts-update-migration"

func NewCadence1ContractsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts Options,
) (
	migs []NamedMigration,
	importantLocations map[common.AddressLocation]struct{},
) {

	stagedContractsMigrationOptions := StagedContractsMigrationOptions{
		ChainID:            opts.ChainID,
		VerboseErrorOutput: opts.VerboseErrorOutput,
	}

	systemContractsMigrationOptions := SystemContractsMigrationOptions{
		StagedContractsMigrationOptions: stagedContractsMigrationOptions,
		EVM:                             opts.EVMContractChange,
		Burner:                          opts.BurnerContractChange,
	}

	var systemContractsMigration *StagedContractsMigration
	systemContractsMigration, importantLocations = NewSystemContractsMigration(
		log,
		rwf,
		systemContractsMigrationOptions,
	)

	stagedContractsMigration := NewStagedContractsMigration(
		"StagedContractsMigration",
		"staged-contracts-migration",
		log,
		rwf,
		stagedContractsMigrationOptions,
	).WithContractUpdateValidation().
		WithStagedContractUpdates(opts.StagedContracts)

	toAccountBasedMigration := func(migration AccountBasedMigration) RegistersMigration {
		return NewAccountBasedMigration(
			log,
			opts.NWorker,
			[]AccountBasedMigration{
				migration,
			},
		)
	}

	switch opts.EVMContractChange {
	case EVMContractChangeNone:
		// NO-OP

	case EVMContractChangeUpdateFull:
		// handled in system contract updates (SystemContractChanges)

	case EVMContractChangeDeployFull,
		EVMContractChangeDeployMinimalAndUpdateFull:

		full := opts.EVMContractChange == EVMContractChangeDeployFull

		migs = append(
			migs,
			NamedMigration{
				Name:    "evm-deployment-migration",
				Migrate: NewEVMDeploymentMigration(opts.ChainID, log, full),
			},
		)
	}

	if opts.BurnerContractChange == BurnerContractChangeDeploy {
		migs = append(
			migs,
			NamedMigration{
				Name:    "burner-deployment-migration",
				Migrate: NewBurnerDeploymentMigration(opts.ChainID, log),
			},
		)
	}

	migs = append(
		migs,
		NamedMigration{
			Name:    "system-contracts-update-migration",
			Migrate: toAccountBasedMigration(systemContractsMigration),
		},
		NamedMigration{
			Name:    stagedContractUpdateMigrationName,
			Migrate: toAccountBasedMigration(stagedContractsMigration),
		},
	)

	return migs, importantLocations
}

var testnetAccountsWithBrokenSlabReferences = func() map[common.Address]struct{} {
	testnetAddresses := map[common.Address]struct{}{
		mustHexToAddress("434a1f199a7ae3ba"): {},
		mustHexToAddress("454c9991c2b8d947"): {},
		mustHexToAddress("48602d8056ff9d93"): {},
		mustHexToAddress("5d63c34d7f05e5a4"): {},
		mustHexToAddress("5e3448b3cffb97f2"): {},
		mustHexToAddress("7d8c7e050c694eaa"): {},
		mustHexToAddress("ba53f16ede01972d"): {},
		mustHexToAddress("c843c1f5a4805c3a"): {},
		mustHexToAddress("48d3be92e6e4a973"): {},
	}

	for address := range testnetAddresses {
		if !flow.Testnet.Chain().IsValid(flow.Address(address)) {
			panic(fmt.Sprintf("invalid testnet address: %s", address.Hex()))
		}
	}

	return testnetAddresses
}()

func mustHexToAddress(hex string) common.Address {
	address, err := common.HexToAddress(hex)
	if err != nil {
		panic(err)
	}
	return address
}

type Options struct {
	NWorker                           int
	DiffMigrations                    bool
	LogVerboseDiff                    bool
	CheckStorageHealthBeforeMigration bool
	VerboseErrorOutput                bool
	ChainID                           flow.ChainID
	EVMContractChange                 EVMContractChange
	BurnerContractChange              BurnerContractChange
	StagedContracts                   []StagedContract
	Prune                             bool
	MaxAccountSize                    uint64
	FixSlabsWithBrokenReferences      bool
	FilterUnreferencedSlabs           bool
	ReportMetrics                     bool
	CacheStaticTypeMigrationResults   bool
	CacheEntitlementsMigrationResults bool
}

func NewCadence1Migrations(
	log zerolog.Logger,
	outputDir string,
	rwf reporters.ReportWriterFactory,
	opts Options,
) (migs []NamedMigration) {

	if opts.MaxAccountSize > 0 {

		maxSizeExceptions := map[string]struct{}{}

		systemContracts := systemcontracts.SystemContractsForChain(opts.ChainID)
		for _, systemContract := range systemContracts.All() {
			maxSizeExceptions[string(systemContract.Address.Bytes())] = struct{}{}
		}

		migs = append(
			migs,
			NamedMigration{
				Name: "account-size-filter-migration",
				Migrate: NewAccountSizeFilterMigration(
					opts.MaxAccountSize,
					maxSizeExceptions,
					log,
				),
			},
		)
	}

	if opts.FixSlabsWithBrokenReferences || opts.FilterUnreferencedSlabs {

		var accountBasedMigrations []AccountBasedMigration

		if opts.FixSlabsWithBrokenReferences {
			accountBasedMigrations = append(
				accountBasedMigrations,
				NewFixBrokenReferencesInSlabsMigration(outputDir, rwf, testnetAccountsWithBrokenSlabReferences),
			)
		}

		if opts.FilterUnreferencedSlabs {
			accountBasedMigrations = append(
				accountBasedMigrations,
				// NOTE: migration to filter unreferenced slabs should happen
				// after migration to fix slabs with references to nonexistent slabs.
				NewFilterUnreferencedSlabsMigration(outputDir, rwf),
			)
		}

		migs = append(
			migs,
			NamedMigration{
				Name: "fix-slabs-migration",
				Migrate: NewAccountBasedMigration(
					log,
					opts.NWorker,
					accountBasedMigrations,
				),
			},
		)
	}

	if opts.Prune {
		migration := NewCadence1PruneMigration(opts.ChainID, log, opts.NWorker)
		if migration != nil {
			migs = append(
				migs,
				NamedMigration{
					Name:    "prune-migration",
					Migrate: migration,
				},
			)
		}
	}

	cadence1ContractsMigrations, importantLocations := NewCadence1ContractsMigrations(
		log,
		rwf,
		opts,
	)

	migs = append(
		migs,
		cadence1ContractsMigrations...,
	)

	migs = append(
		migs,
		NewCadence1ValueMigrations(
			log,
			rwf,
			importantLocations,
			opts,
		)...,
	)

	switch opts.EVMContractChange {
	case EVMContractChangeNone,
		EVMContractChangeDeployFull:
		// NO-OP
	case EVMContractChangeUpdateFull, EVMContractChangeDeployMinimalAndUpdateFull:
		migs = append(
			migs,
			NamedMigration{
				Name:    "evm-setup-migration",
				Migrate: NewEVMSetupMigration(opts.ChainID, log),
			},
		)
	}

	return migs
}

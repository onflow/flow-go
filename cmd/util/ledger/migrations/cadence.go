package migrations

import (
	_ "embed"
	"fmt"

	"github.com/onflow/cadence/migrations/capcons"
	"github.com/onflow/cadence/migrations/statictypes"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm/systemcontracts"
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

func NewCadence1ValueMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts Options,
) (migs []NamedMigration) {

	// Populated by CadenceLinkValueMigration,
	// used by CadenceCapabilityValueMigration
	capabilityMapping := &capcons.CapabilityMapping{}

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
			Name: "check-contracts",
			Migrate: NewContractCheckingMigration(
				log,
				rwf,
				opts.ChainID,
				opts.VerboseErrorOutput,
				programs,
			),
		},
	}

	for index, migrationConstructor := range []func(opts Options) *CadenceBaseMigration{
		func(opts Options) *CadenceBaseMigration {
			return NewCadence1ValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				NewCadence1CompositeStaticTypeConverter(opts.ChainID),
				NewCadence1InterfaceStaticTypeConverter(opts.ChainID),
				opts,
			)
		},
		func(opts Options) *CadenceBaseMigration {
			return NewCadence1LinkValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				capabilityMapping,
				opts,
			)
		},
		func(opts Options) *CadenceBaseMigration {
			return NewCadence1CapabilityValueMigration(
				rwf,
				errorMessageHandler,
				programs,
				capabilityMapping,
				opts,
			)
		},
	} {
		opts := opts
		// Only check storage health before the first migration
		opts.CheckStorageHealthBeforeMigration = opts.CheckStorageHealthBeforeMigration && index == 0

		accountBasedMigration := migrationConstructor(opts)

		migs = append(
			migs,
			NamedMigration{
				Name: accountBasedMigration.name,
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
) (migs []NamedMigration) {

	stagedContractsMigrationOptions := StagedContractsMigrationOptions{
		ChainID:            opts.ChainID,
		VerboseErrorOutput: opts.VerboseErrorOutput,
	}

	systemContractsMigrationOptions := SystemContractsMigrationOptions{
		StagedContractsMigrationOptions: stagedContractsMigrationOptions,
		EVM:                             opts.EVMContractChange,
		Burner:                          opts.BurnerContractChange,
	}

	systemContractsMigration := NewSystemContractsMigration(
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

	if opts.EVMContractChange == EVMContractChangeDeploy {
		migs = append(
			migs,
			NamedMigration{
				Name:    "evm-deployment-migration",
				Migrate: NewEVMDeploymentMigration(opts.ChainID, log),
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

	return migs
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

	migs = append(
		migs,
		NewCadence1ContractsMigrations(
			log,
			rwf,
			opts,
		)...,
	)

	migs = append(
		migs,
		NewCadence1ValueMigrations(
			log,
			rwf,
			opts,
		)...,
	)

	return migs
}

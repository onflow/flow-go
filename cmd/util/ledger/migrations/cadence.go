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
	"github.com/onflow/flow-go/ledger"
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
	return NewStaticTypeMigrator[*interpreter.InterfaceStaticType](rules)
}

func NewCadence1CompositeStaticTypeConverter(chainID flow.ChainID) statictypes.CompositeTypeConverterFunc {
	rules := NewCompositeTypeConversionRules(chainID)
	return NewStaticTypeMigrator[*interpreter.CompositeStaticType](rules)
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
	Migrate ledger.Migration
}

func NewCadence1ValueMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts Options,
) (migrations []NamedMigration) {

	// Populated by CadenceLinkValueMigrator,
	// used by CadenceCapabilityValueMigrator
	capabilityMapping := &capcons.CapabilityMapping{}

	errorMessageHandler := &errorMessageHandler{}

	// The value migrations are run as account-based migrations,
	// i.e. the migrations are only given the payloads for the account to be migrated.
	// However, the migrations need to be able to get the code for contracts of any account.
	//
	// To achieve this, the contracts are extracted from the payloads once,
	// before the value migrations are run.

	programs := make(map[common.Location]*interpreter.Program, 1000)

	migrations = []NamedMigration{
		{
			Name: "check-contracts",
			Migrate: NewContractCheckingMigration(
				log,
				rwf,
				opts.ChainID,
				opts.VerboseErrorOutput,
				programs,
				opts.NWorker,
			),
		},
	}

	for index, migrationConstructor := range []func(opts Options) *CadenceBaseMigrator{
		func(opts Options) *CadenceBaseMigrator {
			return NewCadence1ValueMigrator(
				rwf,
				errorMessageHandler,
				programs,
				NewCadence1CompositeStaticTypeConverter(opts.ChainID),
				NewCadence1InterfaceStaticTypeConverter(opts.ChainID),
				opts,
			)
		},
		func(opts Options) *CadenceBaseMigrator {
			return NewCadence1LinkValueMigrator(
				rwf,
				errorMessageHandler,
				programs,
				capabilityMapping,
				opts,
			)
		},
		func(opts Options) *CadenceBaseMigrator {
			return NewCadence1CapabilityValueMigrator(
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

		migrations = append(
			migrations,
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

	return
}

func NewCadence1ContractsMigrations(
	log zerolog.Logger,
	rwf reporters.ReportWriterFactory,
	opts Options,
) []NamedMigration {

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
		"staged-contracts-migrator",
		log,
		rwf,
		stagedContractsMigrationOptions,
	).WithContractUpdateValidation().
		WithStagedContractUpdates(opts.StagedContracts)

	toAccountBasedMigration := func(migration AccountBasedMigration) ledger.Migration {
		return NewAccountBasedMigration(
			log,
			opts.NWorker,
			[]AccountBasedMigration{
				migration,
			},
		)
	}

	var migrations []NamedMigration

	if opts.BurnerContractChange == BurnerContractChangeDeploy {
		migrations = append(
			migrations,
			NamedMigration{
				Name:    "burner-deployment-migration",
				Migrate: NewBurnerDeploymentMigration(opts.ChainID, log),
			},
		)
	}

	migrations = append(
		migrations,
		NamedMigration{
			Name:    "system-contracts-update-migration",
			Migrate: toAccountBasedMigration(systemContractsMigration),
		},
		NamedMigration{
			Name:    "staged-contracts-update-migration",
			Migrate: toAccountBasedMigration(stagedContractsMigration),
		},
	)

	return migrations
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
}

func NewCadence1Migrations(
	log zerolog.Logger,
	outputDir string,
	rwf reporters.ReportWriterFactory,
	opts Options,
) []NamedMigration {

	var migrations []NamedMigration

	if opts.MaxAccountSize > 0 {

		maxSizeExceptions := map[string]struct{}{}

		systemContracts := systemcontracts.SystemContractsForChain(opts.ChainID)
		for _, systemContract := range systemContracts.All() {
			maxSizeExceptions[string(systemContract.Address.Bytes())] = struct{}{}
		}

		migrations = append(
			migrations,
			NamedMigration{
				Name:    "account-size-filter-migration",
				Migrate: NewAccountSizeFilterMigration(opts.MaxAccountSize, maxSizeExceptions, log),
			},
		)
	}

	if opts.FixSlabsWithBrokenReferences || opts.FilterUnreferencedSlabs {

		var accountBasedMigrations []AccountBasedMigration

		if opts.FixSlabsWithBrokenReferences {
			accountBasedMigrations = append(
				accountBasedMigrations,
				NewFixBrokenReferencesInSlabsMigration(rwf, testnetAccountsWithBrokenSlabReferences),
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

		migrations = append(migrations, NamedMigration{
			Name: "fix-slabs-migration",
			Migrate: NewAccountBasedMigration(
				log,
				opts.NWorker,
				accountBasedMigrations,
			),
		})
	}

	if opts.Prune {
		migration := NewCadence1PruneMigration(opts.ChainID, log, opts.NWorker)
		if migration != nil {
			migrations = append(
				migrations,
				NamedMigration{
					Name:    "prune-migration",
					Migrate: migration,
				},
			)
		}
	}

	migrations = append(
		migrations,
		NewCadence1ContractsMigrations(
			log,
			rwf,
			opts,
		)...,
	)

	migrations = append(
		migrations,
		NewCadence1ValueMigrations(
			log,
			rwf,
			opts,
		)...,
	)

	return migrations
}

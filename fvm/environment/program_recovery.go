package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func RecoverProgram(
	chainID flow.ChainID,
	program *ast.Program,
	location common.Location,
) (
	[]byte,
	error,
) {
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, nil
	}

	sc := systemcontracts.SystemContractsForChain(chainID)

	fungibleTokenAddress := common.Address(sc.FungibleToken.Address)
	nonFungibleTokenAddress := common.Address(sc.NonFungibleToken.Address)

	switch {
	case isFungibleTokenContract(program, fungibleTokenAddress):
		return RecoveredFungibleTokenCode(fungibleTokenAddress, addressLocation.Name), nil

	case isNonFungibleTokenContract(program, nonFungibleTokenAddress):
		return RecoveredNonFungibleTokenCode(nonFungibleTokenAddress, addressLocation.Name), nil
	}

	return nil, nil
}

func RecoveredFungibleTokenCode(fungibleTokenAddress common.Address, contractName string) []byte {
	return []byte(fmt.Sprintf(
		//language=Cadence
		`
          import FungibleToken from %[1]s

          access(all)
          contract %[2]s: FungibleToken {

              access(self)
              view fun recoveryPanic(_ functionName: String): Never {
                  return panic(
                      "%[3]s ".concat(functionName).concat(" is not available in recovered program.")
                  )
              }

              access(all)
              var totalSupply: UFix64

              init() {
                  self.totalSupply = 0.0
              }

              access(all)
              view fun getContractViews(resourceType: Type?): [Type] {
                  %[2]s.recoveryPanic("getContractViews")
              }

              access(all)
              fun resolveContractView(resourceType: Type?, viewType: Type): AnyStruct? {
                  %[2]s.recoveryPanic("resolveContractView")
              }

              access(all)
              fun createEmptyVault(vaultType: Type): @{FungibleToken.Vault} {
                  %[2]s.recoveryPanic("createEmptyVault")
              }

              access(all)
              resource Vault: FungibleToken.Vault {

                  access(all)
                  var balance: UFix64

                  init(balance: UFix64) {
                      self.balance = balance
                  }

                  access(FungibleToken.Withdraw)
                  fun withdraw(amount: UFix64): @{FungibleToken.Vault} {
                      %[2]s.recoveryPanic("Vault.withdraw")
                  }

                  access(all)
                  view fun isAvailableToWithdraw(amount: UFix64): Bool {
                      %[2]s.recoveryPanic("Vault.isAvailableToWithdraw")
                  }

                  access(all)
                  fun deposit(from: @{FungibleToken.Vault}) {
                      %[2]s.recoveryPanic("Vault.deposit")
                  }

                  access(all)
                  fun createEmptyVault(): @{FungibleToken.Vault} {
                      %[2]s.recoveryPanic("Vault.createEmptyVault")
                  }

                  access(all)
                  view fun getViews(): [Type] {
                      %[2]s.recoveryPanic("Vault.getViews")
                  }

                  access(all)
                  fun resolveView(_ view: Type): AnyStruct? {
                      %[2]s.recoveryPanic("Vault.resolveView")
                  }
              }
          }
        `,
		fungibleTokenAddress.HexWithPrefix(),
		contractName,
		fmt.Sprintf("Contract %s is no longer functional. "+
			"A version of the contract has been recovered to allow access to the fields declared in the FT standard.",
			contractName,
		),
	))
}

func RecoveredNonFungibleTokenCode(nonFungibleTokenAddress common.Address, contractName string) []byte {
	return []byte(fmt.Sprintf(
		//language=Cadence
		`
          import NonFungibleToken from %[1]s

          access(all)
          contract %[2]s: NonFungibleToken {

              access(self)
              view fun recoveryPanic(_ functionName: String): Never {
                  return panic(
                      "%[3]s ".concat(functionName).concat(" is not available in recovered program.")
                  )
              }

              access(all)
              view fun getContractViews(resourceType: Type?): [Type] {
                  %[2]s.recoveryPanic("getContractViews")
              }

              access(all)
              fun resolveContractView(resourceType: Type?, viewType: Type): AnyStruct? {
                  %[2]s.recoveryPanic("resolveContractView")
              }

              access(all)
              fun createEmptyCollection(nftType: Type): @{NonFungibleToken.Collection} {
                  %[2]s.recoveryPanic("createEmptyCollection")
              }

              access(all)
              resource NFT: NonFungibleToken.NFT {

                  access(all)
                  let id: UInt64

                  init(id: UInt64) {
                      self.id = id
                  }

                  access(all)
                  view fun getViews(): [Type] {
                      %[2]s.recoveryPanic("NFT.getViews")
                  }

                  access(all)
                  fun resolveView(_ view: Type): AnyStruct? {
                      %[2]s.recoveryPanic("NFT.resolveView")
                  }

                  access(all)
                  fun createEmptyCollection(): @{NonFungibleToken.Collection} {
                      %[2]s.recoveryPanic("NFT.createEmptyCollection")
                  }
              }

              access(all)
              resource Collection: NonFungibleToken.Collection {

                  access(all)
                  var ownedNFTs: @{UInt64: {NonFungibleToken.NFT}}

                  init() {
                      self.ownedNFTs <- {}
                  }

                  access(all)
                  fun deposit(token: @{NonFungibleToken.NFT}) {
                      %[2]s.recoveryPanic("Collection.deposit")
                  }

                  access(all)
                  view fun getSupportedNFTTypes(): {Type: Bool} {
                      %[2]s.recoveryPanic("Collection.getSupportedNFTTypes")
                  }

                  access(all)
                  view fun isSupportedNFTType(type: Type): Bool {
                      %[2]s.recoveryPanic("Collection.isSupportedNFTType")
                  }

                  access(NonFungibleToken.Withdraw)
                  fun withdraw(withdrawID: UInt64): @{NonFungibleToken.NFT} {
                      %[2]s.recoveryPanic("Collection.withdraw")
                  }

                  access(all)
                  view fun borrowNFT(_ id: UInt64): &{NonFungibleToken.NFT}? {
                      %[2]s.recoveryPanic("Collection.borrowNFT")
                  }

                  access(all)
                  fun createEmptyCollection(): @{NonFungibleToken.Collection} {
                      %[2]s.recoveryPanic("Collection.createEmptyCollection")
                  }
              }
          }
        `,
		nonFungibleTokenAddress.HexWithPrefix(),
		contractName,
		fmt.Sprintf("Contract %s is no longer functional. "+
			"A version of the contract has been recovered to allow access to the fields declared in the NFT standard.",
			contractName,
		),
	))
}

func importsAddressLocation(program *ast.Program, address common.Address, name string) bool {
	importDeclarations := program.ImportDeclarations()

	// Check if the location is imported by any import declaration
	for _, importDeclaration := range importDeclarations {

		// The import declaration imports from the same address
		importedLocation, ok := importDeclaration.Location.(common.AddressLocation)
		if !ok || importedLocation.Address != address {
			continue
		}

		// The import declaration imports all identifiers, so also the location
		if len(importDeclaration.Identifiers) == 0 {
			return true
		}

		// The import declaration imports specific identifiers, so check if the location is imported
		for _, identifier := range importDeclaration.Identifiers {
			if identifier.Identifier == name {
				return true
			}
		}
	}

	return false
}

func declaresConformanceTo(conformingDeclaration ast.ConformingDeclaration, name string) bool {
	for _, conformance := range conformingDeclaration.ConformanceList() {
		if conformance.Identifier.Identifier == name {
			return true
		}
	}

	return false
}

func isNominalType(ty ast.Type, name string) bool {
	nominalType, ok := ty.(*ast.NominalType)
	return ok &&
		len(nominalType.NestedIdentifiers) == 0 &&
		nominalType.Identifier.Identifier == name
}

const fungibleTokenTypeIdentifier = "FungibleToken"
const fungibleTokenTypeTotalSupplyFieldName = "totalSupply"
const fungibleTokenVaultTypeIdentifier = "Vault"
const fungibleTokenVaultTypeBalanceFieldName = "balance"

const nonFungibleTokenTypeIdentifier = "NonFungibleToken"
const nonFungibleTokenTypeTotalSupplyFieldName = "totalSupply"
const nonFungibleTokenNFTTypeIdentifier = "NFT"
const nonFungibleTokenNFTTypeIDFieldName = "id"
const nonFungibleTokenCollectionTypeIdentifier = "Collection"
const nonFungibleTokenCollectionTypeOwnedNFTsFieldName = "ownedNFTs"

func isFungibleTokenContract(program *ast.Program, fungibleTokenAddress common.Address) bool {

	// Check if the contract imports the FungibleToken contract
	if !importsAddressLocation(program, fungibleTokenAddress, fungibleTokenTypeIdentifier) {
		return false
	}

	contractDeclaration := program.SoleContractDeclaration()
	if contractDeclaration == nil {
		return false
	}

	// Check if the contract implements the FungibleToken interface
	if !declaresConformanceTo(contractDeclaration, fungibleTokenTypeIdentifier) {
		return false
	}

	// Check if the contract has a totalSupply field
	totalSupplyFieldDeclaration := getField(contractDeclaration, fungibleTokenTypeTotalSupplyFieldName)
	if totalSupplyFieldDeclaration == nil {
		return false
	}

	// Check if the totalSupply field is of type UFix64
	if !isNominalType(totalSupplyFieldDeclaration.TypeAnnotation.Type, sema.UFix64TypeName) {
		return false
	}

	// Check if the contract has a Vault resource

	vaultDeclaration := contractDeclaration.Members.CompositesByIdentifier()[fungibleTokenVaultTypeIdentifier]
	if vaultDeclaration == nil {
		return false
	}

	// Check if the Vault resource has a balance field
	balanceFieldDeclaration := getField(vaultDeclaration, fungibleTokenVaultTypeBalanceFieldName)
	if balanceFieldDeclaration == nil {
		return false
	}

	// Check if the balance field is of type UFix64
	if !isNominalType(balanceFieldDeclaration.TypeAnnotation.Type, sema.UFix64TypeName) {
		return false
	}

	return true
}

func isNonFungibleTokenContract(program *ast.Program, nonFungibleTokenAddress common.Address) bool {

	// Check if the contract imports the NonFungibleToken contract
	if !importsAddressLocation(program, nonFungibleTokenAddress, nonFungibleTokenTypeIdentifier) {
		return false
	}

	contractDeclaration := program.SoleContractDeclaration()
	if contractDeclaration == nil {
		return false
	}

	// Check if the contract implements the NonFungibleToken interface
	if !declaresConformanceTo(contractDeclaration, nonFungibleTokenTypeIdentifier) {
		return false
	}

	// Check if the contract has a totalSupply field
	totalSupplyFieldDeclaration := getField(contractDeclaration, nonFungibleTokenTypeTotalSupplyFieldName)
	if totalSupplyFieldDeclaration == nil {
		return false
	}

	// Check if the totalSupply field is of type UInt64
	if !isNominalType(totalSupplyFieldDeclaration.TypeAnnotation.Type, sema.UInt64TypeName) {
		return false
	}

	// Check if the contract has an NFT resource

	nestedComposites := contractDeclaration.Members.CompositesByIdentifier()

	nftDeclaration := nestedComposites[nonFungibleTokenNFTTypeIdentifier]
	if nftDeclaration == nil {
		return false
	}

	// Check if the NFT resource has an id field
	idFieldDeclaration := getField(nftDeclaration, nonFungibleTokenNFTTypeIDFieldName)
	if idFieldDeclaration == nil {
		return false
	}

	// Check if the id field is of type UInt64
	if !isNominalType(idFieldDeclaration.TypeAnnotation.Type, sema.UInt64TypeName) {
		return false
	}

	// Check if the contract has a Collection resource
	collectionDeclaration := nestedComposites[nonFungibleTokenCollectionTypeIdentifier]
	if collectionDeclaration == nil {
		return false
	}

	// Check if the Collection resource has an ownedNFTs field
	ownedNFTsFieldDeclaration := getField(collectionDeclaration, nonFungibleTokenCollectionTypeOwnedNFTsFieldName)
	if ownedNFTsFieldDeclaration == nil {
		return false
	}

	// Check if the ownedNFTs field is of type {UInt64: NonFungibleToken.NFT} (NOTE: old syntax)
	ownedNFTsFieldType := ownedNFTsFieldDeclaration.TypeAnnotation.Type
	ownedNFTsFieldDictionaryType, ok := ownedNFTsFieldType.(*ast.DictionaryType)
	if !ok ||
		!isNominalType(ownedNFTsFieldDictionaryType.KeyType, sema.UInt64TypeName) ||
		!isNonFungibleTokenNFTNominalType(ownedNFTsFieldDictionaryType.ValueType) {

		return false
	}

	return true
}

// isNonFungibleTokenNFTNominalType checks if the given type is a nominal type representing `NonFungibleToken.NFT`
func isNonFungibleTokenNFTNominalType(ty ast.Type) bool {
	nominalType, ok := ty.(*ast.NominalType)
	return ok &&
		nominalType.Identifier.Identifier == nonFungibleTokenTypeIdentifier &&
		len(nominalType.NestedIdentifiers) == 1 &&
		nominalType.NestedIdentifiers[0].Identifier == nonFungibleTokenNFTTypeIdentifier
}

func getField(declaration *ast.CompositeDeclaration, name string) *ast.FieldDeclaration {
	for _, fieldDeclaration := range declaration.Members.Fields() {
		if fieldDeclaration.Identifier.Identifier == name {
			return fieldDeclaration
		}
	}

	return nil
}

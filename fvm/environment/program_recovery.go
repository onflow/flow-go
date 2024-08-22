package environment

import (
	"fmt"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/cadence/runtime/sema"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func RecoverProgram(
	memoryGauge common.MemoryGauge,
	chainID flow.ChainID,
	program *ast.Program,
	location common.Location,
) (
	*ast.Program,
	error,
) {
	addressLocation, ok := location.(common.AddressLocation)
	if !ok {
		return nil, nil
	}

	sc := systemcontracts.SystemContractsForChain(chainID)

	fungibleTokenAddress := common.Address(sc.FungibleToken.Address)
	nonFungibleTokenAddress := common.Address(sc.NonFungibleToken.Address)

	var code string

	switch {
	case isFungibleTokenContract(program, fungibleTokenAddress):
		code = RecoveredFungibleTokenCode(fungibleTokenAddress, addressLocation.Name)

	case isNonFungibleTokenContract(program, nonFungibleTokenAddress):
		code = RecoveredNonFungibleTokenCode(nonFungibleTokenAddress, addressLocation.Name)
	default:
		return nil, nil
	}

	return parser.ParseProgram(memoryGauge, []byte(code), parser.Config{})

	return nil, nil
}

func RecoveredFungibleTokenCode(fungibleTokenAddress common.Address, contractName string) string {
	return fmt.Sprintf(
		//language=Cadence
		`
          import FungibleToken from %s

          access(all)
          contract %s: FungibleToken {

              access(all)
              var totalSupply: UFix64

              init() {
                  self.totalSupply = 0.0
              }

              access(all)
              view fun getContractViews(resourceType: Type?): [Type] {
                  panic("getContractViews is not available in recovered program")
              }

              access(all)
              fun resolveContractView(resourceType: Type?, viewType: Type): AnyStruct? {
                  panic("resolveContractView is not available in recovered program")
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
                      panic("withdraw is not available in recovered program")
                  }

                  access(all)
                  view fun isAvailableToWithdraw(amount: UFix64): Bool {
                      panic("isAvailableToWithdraw is not available in recovered program")
                  }

                  access(all)
                  fun deposit(from: @{FungibleToken.Vault}) {
                      panic("deposit is not available in recovered program")
                  }

                  access(all)
                  fun createEmptyVault(): @{FungibleToken.Vault} {
                      panic("createEmptyVault is not available in recovered program")
                  }

                  access(all)
                  view fun getViews(): [Type] {
                      panic("getViews is not available in recovered program")
                  }

                  access(all)
                  fun resolveView(_ view: Type): AnyStruct? {
                      panic("resolveView is not available in recovered program")
                  }
              }

              access(all)
              fun createEmptyVault(vaultType: Type): @{FungibleToken.Vault} {
                  panic("createEmptyVault is not available in recovered program")
              }
          }
        `,
		fungibleTokenAddress.HexWithPrefix(),
		contractName,
	)
}

func RecoveredNonFungibleTokenCode(nonFungibleTokenAddress common.Address, contractName string) string {
	return fmt.Sprintf(
		//language=Cadence
		`
          import NonFungibleToken from %s

          access(all)
          contract %s: NonFungibleToken {

              access(all)
              resource NFT: NonFungibleToken.NFT {

                  access(all)
                  let id: UInt64

                  init(id: UInt64) {
                      self.id = id
                  }

                  access(all)
                  view fun getViews(): [Type] {
                      panic("getViews is not available in recovered program")
                  }

                  access(all)
                  fun resolveView(_ view: Type): AnyStruct? {
                      panic("resolveView is not available in recovered program")
                  }

                  access(all)
                  fun createEmptyCollection(): @{NonFungibleToken.Collection} {
                      panic("createEmptyCollection is not available in recovered program")
                  }
              }

              access(all)
              view fun getContractViews(resourceType: Type?): [Type] {
                  panic("getContractViews is not available in recovered program")
              }

              access(all)
              fun resolveContractView(resourceType: Type?, viewType: Type): AnyStruct? {
                  panic("resolveContractView is not available in recovered program")
              }

              access(all)
              fun createEmptyCollection(nftType: Type): @{NonFungibleToken.Collection} {
                  panic("createEmptyCollection is not available in recovered program")
              }
          }
        `,
		nonFungibleTokenAddress.HexWithPrefix(),
		contractName,
	)
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
	nftDeclaration := contractDeclaration.Members.CompositesByIdentifier()[nonFungibleTokenNFTTypeIdentifier]
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

	return true
}

func getField(declaration *ast.CompositeDeclaration, name string) *ast.FieldDeclaration {
	for _, fieldDeclaration := range declaration.Members.Fields() {
		if fieldDeclaration.Identifier.Identifier == name {
			return fieldDeclaration
		}
	}

	return nil
}

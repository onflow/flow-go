package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateSaleScript creates a cadence transaction that creates a Sale collection
// and stores in in the callers account published
func GenerateCreateSaleScript(tokenAddr flow.Address, marketAddr flow.Address) []byte {
	template := `
		import FungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: Account) {

				let ownerVault = acct.published[&FungibleToken.Receiver] ?? panic("No receiver reference!")

				let collection <- Market.createSaleCollection(ownerVault: ownerVault)
				
				let oldCollection <- acct.storage[Market.SaleCollection] <- collection
				destroy oldCollection

				acct.published[&Market.SaleCollection] = &acct.storage[Market.SaleCollection] as Market.SaleCollection
			}
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, marketAddr))
}

// GenerateStartSaleScript creates a cadence transaction that starts a sale by depositing
// an NFT into the Sale Collection with an associated price
func GenerateStartSaleScript(nftAddr flow.Address, marketAddr flow.Address, id, price int) []byte {
	template := `
		import NonFungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: Account) {

				let token <- acct.published[&NonFungibleToken.NFTCollection]?.withdraw(tokenID: %d) ?? panic("missing token!")

				let saleRef = acct.published[&Market.SaleCollection] ?? panic("no sale collection reference!")
			
				saleRef.listForSale(token: <-token, price: %d)

			}
		}`
	return []byte(fmt.Sprintf(template, nftAddr, marketAddr, id, price))
}

// GenerateBuySaleScript creates a cadence transaction that makes a purchase of
// an existing sale
func GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, userAddr flow.Address, id, amount int) []byte {
	template := `
		import FungibleToken from 0x%s
		import NonFungibleToken from 0x%s
		import Market from 0x%s

		transaction {
			prepare(acct: Account) {
				let seller = getAccount(0x%s)

				let collectionRef = acct.published[&NonFungibleToken.NFTCollection] ?? panic("missing collection!")
				let providerRef = acct.published[&FungibleToken.Provider] ?? panic("missing Provider!")
				
				let tokens <- providerRef.withdraw(amount: %d)

				let saleRef = seller.published[&Market.SaleCollection] ?? panic("no sale collection reference!")
			
				saleRef.purchase(tokenID: %d, recipient: collectionRef, buyTokens: <-tokens)

			}
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, nftAddr, marketAddr, userAddr, amount, id))
}

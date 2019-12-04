package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateSaleScript creates a cadence transaction that creates a Sale collection
// and stores in in the callers account published
func GenerateCreateSaleScript(tokenAddr flow.Address, marketAddr flow.Address) []byte {
	template := `
		import Receiver from 0x%s
		import SaleCollection, createSaleCollection from 0x%s

		transaction {
			prepare(acct: Account) {

				let ownerVault = acct.published[&Receiver] ?? panic("No receiver reference!")

				let collection: <-SaleCollection <- createSaleCollection(ownerVault: ownerVault)
				
				let oldCollection <- acct.storage[SaleCollection] <- collection
				destroy oldCollection

				acct.published[&SaleCollection] = &acct.storage[SaleCollection] as SaleCollection

			}
			execute {}

		}`
	return []byte(fmt.Sprintf(template, tokenAddr, marketAddr))
}

// GenerateStartSaleScript creates a cadence transaction that starts a sale by depositing
// an NFT into the Sale Collection with an associated price
func GenerateStartSaleScript(nftAddr flow.Address, marketAddr flow.Address, id, price int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s
		import SaleCollection, createSaleCollection from 0x%s

		transaction {
			prepare(acct: Account) {

				let token <- acct.published[&NFTCollection]?.withdraw(tokenID: %d) ?? panic("missing token!")

				let saleRef = acct.published[&SaleCollection] ?? panic("no sale collection reference!")
			
				saleRef.listForSale(token: <-token, price: %d)

			}
			execute {}

		}`
	return []byte(fmt.Sprintf(template, nftAddr, marketAddr, id, price))
}

// GenerateBuySaleScript creates a cadence transaction that makes a purchase of
// an existing sale
func GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, userAddr flow.Address, id, amount int) []byte {
	template := `
		import Receiver, Provider, Vault from 0x%s
		import NFT, NFTCollection from 0x%s
		import SaleCollection, createSaleCollection from 0x%s

		transaction {
			prepare(acct: Account) {
				let seller = getAccount(0x%s)

				let collectionRef = acct.published[&NFTCollection] ?? panic("missing collection!")
				let providerRef = acct.published[&Provider] ?? panic("missing Provider!")
				
				let tokens <- providerRef.withdraw(amount: %d)

				let saleRef = seller.published[&SaleCollection] ?? panic("no sale collection reference!")
			
				saleRef.purchase(tokenID: %d, recipient: collectionRef, buyTokens: <-tokens)

			}
			execute {}

		}`
	return []byte(fmt.Sprintf(template, tokenAddr, nftAddr, marketAddr, userAddr, amount, id))
}

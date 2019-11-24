package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateSaleScript creates a cadence transaction that creates a Sale collection
// and stores in in the callers account storage
func GenerateCreateSaleScript(tokenAddr flow.Address, marketAddr flow.Address) []byte {
	template := `
		import Receiver from 0x%s
		import SaleCollection, createCollection from 0x%s

		pub fun main(acct: Account) {

			let ownerVault = acct.storage[&Receiver] ?? panic("No receiver reference!")

			let collection: <-SaleCollection <- createCollection(ownerVault: ownerVault)
			
			let oldCollection <- acct.storage[SaleCollection] <- collection
			destroy oldCollection

			acct.storage[&SaleCollection] = &acct.storage[SaleCollection] as SaleCollection

		}`
	return []byte(fmt.Sprintf(template, tokenAddr, marketAddr))
}

// GenerateStartSaleScript creates a cadence transaction that starts a sale by depositing
// an NFT into the Sale Collection with an associated price
func GenerateStartSaleScript(nftAddr flow.Address, marketAddr flow.Address, id, price int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s
		import SaleCollection, createCollection from 0x%s

		pub fun main(acct: Account) {

			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing collection!")
			
			let token <- collectionRef.withdraw(tokenID: %d)

			// why doesn't this work?
			// let token <- acct.storage[&NFTCollection]?.withdraw(tokenID: ) ?? panic("missing token!")

			let saleRef = acct.storage[&SaleCollection] ?? panic("no sale collection reference!")
		
			saleRef.listForSale(token: <-token, price: %d)

		}`
	return []byte(fmt.Sprintf(template, nftAddr, marketAddr, id, price))
}

// GenerateBuySaleScript creates a cadence transaction that makes a purchase of
// an existing sale
func GenerateBuySaleScript(tokenAddr, nftAddr, marketAddr, userAddr flow.Address, id, amount int) []byte {
	template := `
		import Receiver, Provider, Vault from 0x%s
		import NFT, NFTCollection from 0x%s
		import SaleCollection, createCollection from 0x%s

		pub fun main(acct: Account) {
			let seller = getAccount("%s")

			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing collection!")
			let providerRef = acct.storage[&Provider] ?? panic("missing Provider!")
			
			let tokens <- providerRef.withdraw(amount: %d)

			let saleRef = seller.storage[&SaleCollection] ?? panic("no sale collection reference!")
		
			saleRef.purchase(tokenID: %d, recipient: collectionRef, buyTokens: <-tokens)

		}`
	return []byte(fmt.Sprintf(template, tokenAddr, nftAddr, marketAddr, userAddr, amount, id))
}

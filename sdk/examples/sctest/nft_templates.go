package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Creates a script that instantiates a new NFT instance,
// then creates an NFT collection instance,
// and stores the NFT in the collection
// then stores the collection in memory
// The id must be greater than zero
func GenerateCreateNFTScript(tokenAddr flow.Address, id int) []byte {
	template := `
		import NFT, NFTCollection, createNFT, createCollection from 0x%s

		fun main(acct: Account) {
			var tokenA: <-NFT <- createNFT(id: %d)
			//var tokenB: <-NFT? <- createNFT(id: 2)

			var collection: <-NFTCollection <- createCollection(token: <-tokenA)

			// collection.deposit(token: <-tokenB, id: 2)

			// if collection.idExists(tokenID: 1) == false {
			// 	panic("Token ID doesn't exist!")
			// }

			var collectionA: <-NFTCollection? <- collection
			
			acct.storage[NFTCollection] <-> collectionA

			acct.storage[&NFTCollection] = &acct.storage[NFTCollection] as NFTCollection

			destroy collectionA
		}`
	return []byte(fmt.Sprintf(template, tokenAddr, id))
}

// Creates a script that withdraws tokens from a vault
// and deposits them to another vault
func GenerateTransferScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, transferNFTID int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main(acct: Account) {
			let recipient = getAccount("%s")

			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing NFT collection reference")
			let depositRef = recipient.storage[&NFTCollection] ?? panic("missing deposit reference")

			collectionRef.transfer(recipient: depositRef, tokenID: %d)
		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), receiverAddr.String(), transferNFTID))
}

// Creates a script that withdraws tokens from a vault
// and deposits them to another vault
func GenerateDepositScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, transferNFTID int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main(acct: Account) {
			let recipient = getAccount("%s")

			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing NFT collection reference")
			let depositRef = recipient.storage[&NFTCollection] ?? panic("missing deposit reference")

			let nft <- collectionRef.withdraw(tokenID: %d)

			depositRef.deposit(token: <-nft, id: 1)

		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), receiverAddr.String(), transferNFTID))
}

// Creates a script that retrieves an NFT collection from storage and makes assertions
// about the NFT IDs that it contains
func GenerateInspectCollectionScript(nftCodeAddr, userAddr flow.Address, nftID int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing collection reference")

			// if collectionRef.idExists(tokenID: %d) == false {
			// 	panic("Token ID doesn't exist!")
			// }

			if collectionRef.ownedNFTs[%d].id 
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, nftID))
}

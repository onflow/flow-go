package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateNFTScript Creates a script that instantiates a new
// NFT instance, then creates an NFT collection instance, stores the
// NFT in the collection, stores the collection in memory, then stores a
// reference to the collection. It also makes sure that the token exists
// in the collection after it has been added to.
// The id must be greater than zero
func GenerateCreateNFTScript(tokenAddr flow.Address, id int) []byte {
	template := `
		import NFT, NFTCollection, createNFT, createCollection from 0x%s

		fun main(acct: Account) {
			var tokenA: <-NFT <- createNFT(id: %d)

			var collection: <-NFTCollection <- createCollection()

			collection.deposit(token: <-tokenA)

			if collection.idExists(tokenID: %d) == false {
				panic("Token ID doesn't exist!")
			}
			
			let oldCollection <- acct.storage[NFTCollection] <- collection
			destroy oldCollection

			acct.storage[&NFTCollection] = &acct.storage[NFTCollection] as NFTCollection

		}`
	return []byte(fmt.Sprintf(template, tokenAddr, id, id))
}

// GenerateDepositScript creates a script that withdraws an NFT token
// from a collection and deposits it to another collection
func GenerateDepositScript(tokenCodeAddr flow.Address, receiverAddr flow.Address, transferNFTID int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main(acct: Account) {
			let recipient = getAccount("%s")

			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing NFT collection reference")
			let depositRef = recipient.storage[&NFTCollection] ?? panic("missing deposit reference")

			let nft <- collectionRef.withdraw(tokenID: %d)

			depositRef.deposit(token: <-nft)
		}`

	return []byte(fmt.Sprintf(template, tokenCodeAddr.String(), receiverAddr.String(), transferNFTID))
}

// GenerateTransferScript Creates a script that transfers an NFT
// to another vault
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

// GenerateInspectCollectionScript creates a script that retrieves an NFT collection
// from storage and makes assertions about an NFT ID that it contains with the idExists
// function, which uses an array of IDs
func GenerateInspectCollectionScript(nftCodeAddr, userAddr flow.Address, nftID int, shouldExist bool) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing collection reference")

			if collectionRef.idExists(tokenID: %d) != %v {
				panic("Token ID doesn't exist!")
			}
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, nftID, shouldExist))
}

// GenerateInspectKeysScript creates a script that retrieves an NFT collection
// from storage and reads the array of keys in the dictionary
func GenerateInspectKeysScript(nftCodeAddr, userAddr flow.Address, id1, id2 int) []byte {
	template := `
		import NFT, NFTCollection from 0x%s

		fun main() {
			let acct = getAccount("%s")
			let collectionRef = acct.storage[&NFTCollection] ?? panic("missing collection reference")

			let array = collectionRef.getIDs() 
			
			if array[0] != %d || array[1] != %d {
				panic("Keys array is incorrect!")
			}
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, id1, id2))
}

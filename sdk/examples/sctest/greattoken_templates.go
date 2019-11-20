package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateMinterScript Creates a script that instantiates a new GreatNFTMinter instance
// and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func GenerateCreateMinterScript(nftAddr flow.Address, initialID, specialMod int) []byte {
	template := `
		import GreatNFTMinter from 0x%s

		pub fun main(acct: Account) {
			let minter = GreatNFTMinter(firstID: %d, specialMod: %d)
			acct.storage[GreatNFTMinter] = minter
		}`
	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// GenerateMintScript Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func GenerateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		import GreatNFTMinter, GreatNFT from 0x%s

		pub fun main(acct: Account) {
			let minter = acct.storage[GreatNFTMinter] ?? panic("missing minter")
			let nft = minter.mint()

			acct.storage[GreatNFT] = nft
			acct.storage[GreatNFTMinter] = minter
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr.String()))
}

// GenerateInspectNFTScript Creates a script that retrieves an NFT from storage and makes assertions
// about its properties. If these assertions fail, the script panics.
func GenerateInspectNFTScript(nftCodeAddr, userAddr flow.Address, expectedID int, expectedIsSpecial bool) []byte {
	template := `
		import GreatNFT from 0x%s

		pub fun main() {
			let acct = getAccount("%s")
			let nft = acct.storage[GreatNFT] ?? panic("missing nft")
			if nft.id() != %d {
				panic("incorrect id")
			}
			if nft.isSpecial() != %t {
				panic("incorrect specialness")
			}
		}`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, expectedID, expectedIsSpecial))
}

// GenerateNFTIDScript Creates a script that retrieves the id of the NFT that you own
func GenerateNFTIDScript(nftCodeAddr, userAddr flow.Address) []byte {
	template := `
		import GreatNFT from 0x%s

		pub fun main(): Int {
			let acct = getAccount("%s")
			let nft = acct.storage[GreatNFT] ?? panic("missing nft")
			return nft.id()
		}`
	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr))
}

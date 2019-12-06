package sctest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// GenerateCreateMinterScript Creates a script that instantiates
// a new GreatNFTMinter instance and stores it in memory.
// Initial ID and special mod are arguments to the GreatNFTMinter constructor.
// The GreatNFTMinter must have been deployed already.
func GenerateCreateMinterScript(nftAddr flow.Address, initialID, specialMod int) []byte {
	template := `
		import 0x%s

		transaction {

		  prepare(acct: Account) {
			let existing <- acct.storage[GreatNFTMinter] <- createGreatNFTMinter(firstID: %d, specialMod: %d)
			assert(existing == nil, message: "existed")
			destroy existing

			acct.storage[&GreatNFTMinter] = &acct.storage[GreatNFTMinter] as GreatNFTMinter
		  }
		}
	`

	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// GenerateMintScript Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func GenerateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		import GreatNFTMinter, GreatNFT from 0x%s

		transaction {

		  prepare(acct: Account) {
			let minter = acct.storage[&GreatNFTMinter] ?? panic("missing minter")
			let existing <- acct.storage[GreatNFT] <- minter.mint()
			destroy existing
            acct.published[&GreatNFT] = &acct.storage[GreatNFT] as GreatNFT
		  }
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr.String()))
}

// GenerateInspectNFTScript Creates a script that retrieves an NFT
// from storage and makes assertions about its properties.
// If these assertions fail, the script panics.
func GenerateInspectNFTScript(nftCodeAddr, userAddr flow.Address, expectedID int, expectedIsSpecial bool) []byte {
	template := `
		import GreatNFT from 0x%s

		pub fun main() {
		  let acct = getAccount(0x%s)
		  let nft = acct.published[&GreatNFT] ?? panic("missing nft")
		  assert(
              nft.id() == %d,
              message: "incorrect id"
          )
		  assert(
              nft.isSpecial() == %t,
              message: "incorrect specialness"
          )
		}
	`

	return []byte(fmt.Sprintf(template, nftCodeAddr, userAddr, expectedID, expectedIsSpecial))
}

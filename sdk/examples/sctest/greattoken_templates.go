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
		import GreatToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let existing <- acct.storage[GreatToken.GreatNFTMinter] <- GreatToken.createGreatNFTMinter(firstID: %d, specialMod: %d)
			assert(existing == nil, message: "existed")
			destroy existing

			acct.storage[&GreatToken.GreatNFTMinter] = &acct.storage[GreatToken.GreatNFTMinter] as GreatToken.GreatNFTMinter
		  }
		}
	`

	return []byte(fmt.Sprintf(template, nftAddr, initialID, specialMod))
}

// GenerateMintScript Creates a script that mints an NFT and put it into storage.
// The minter must have been instantiated already.
func GenerateMintScript(nftCodeAddr flow.Address) []byte {
	template := `
		import GreatToken from 0x%s

		transaction {

		  prepare(acct: Account) {
			let minter = acct.storage[&GreatToken.GreatNFTMinter] ?? panic("missing minter")
			let existing <- acct.storage[GreatToken.GreatNFT] <- minter.mint()
			destroy existing
            acct.published[&GreatToken.GreatNFT] = &acct.storage[GreatToken.GreatNFT] as GreatToken.GreatNFT
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
		import GreatToken from 0x%s

		pub fun main() {
		  let acct = getAccount(0x%s)
		  let nft = acct.published[&GreatToken.GreatNFT] ?? panic("missing nft")
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

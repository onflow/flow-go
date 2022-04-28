/*
    Help manage a primary sale for SomePlace collectibles utilizing preminted NFTs that are in storage.
    This is meant to be curated for specific drops where there are leftover NFTs to be sold publically from a private sale.
*/
import NonFungibleToken from 0x1d7e57aa55817448
import SomePlaceCollectible from 0x667a16294a089ef8

pub contract SomePlacePrimarySaleHelper {
    access(self) let premintedNFTCap: Capability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>
    
    access(account) fun retrieveAvailableNFT(): @SomePlaceCollectible.NFT {
        let nftCollection = self.premintedNFTCap.borrow()!
        let randomIndex = unsafeRandom() % UInt64(nftCollection.getIDs().length)
        return <-(nftCollection.withdraw(withdrawID: nftCollection.getIDs()[randomIndex]) as! @SomePlaceCollectible.NFT)
    }

    pub fun getRemainingNFTCount(): Int {
        return self.premintedNFTCap.borrow()!.getIDs().length
    }

    init() {
        self.account.link<&SomePlaceCollectible.Collection>(/private/SomePlacePrimarySaleHelperAccess, target: SomePlaceCollectible.CollectionStoragePath)
        self.premintedNFTCap = self.account.getCapability<&{NonFungibleToken.Provider, NonFungibleToken.CollectionPublic}>(/private/SomePlacePrimarySaleHelperAccess)
    }
}

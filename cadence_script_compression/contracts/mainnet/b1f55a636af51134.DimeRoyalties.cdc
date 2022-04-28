/* SPDX-License-Identifier: UNLICENSED */

import DimeCollectibleV3 from 0xf5cdaace879e5a79
import FungibleToken from 0xf233dcee88fe0abe
import FUSD from 0x3c5959b568896393
import NonFungibleToken from 0x1d7e57aa55817448

pub contract DimeRoyalties {
    // Events
	pub event ReleaseCreated(releaseId: UInt64)
    pub event RoyaltyNFTAdded(itemId: UInt64)

    // Named Paths
    pub let ReleasesStoragePath: StoragePath
    pub let ReleasesPrivatePath: PrivatePath
    pub let ReleasesPublicPath: PublicPath

    access(self) var nextReleaseId: UInt64

	pub struct SaleShares {
		access(self) let allotments: {Address: UFix64}

		init(allotments: {Address: UFix64}) {
			var total = 0.0
			for allotment in allotments.values {
				assert(allotment > 0.0, message: "Each recipient must receive an allotment > 0")
				total = total + allotment
			}
			assert(total == 1.0, message: "Total sale shares must equal exactly 1")
			self.allotments = allotments
		}

		pub fun getShares(): {Address: UFix64} {
			return self.allotments
		}
	}

    pub resource interface ReleasePublic {
        pub let id: UInt64
        pub let royaltiesPerShare: UFix64
        pub let numRoyaltyNFTs: UInt64
        pub fun getRoyaltyIds(): [UInt64]
        pub fun getRoyaltyOwners(): {UInt64: Address}
        pub fun updateOwner(id: UInt64, newOwner: Address)
        pub fun getReleaseIds(): [UInt64]
        pub let managerFees: UFix64
        pub fun getArtistShares(): SaleShares
        pub fun getManagerShares(): SaleShares
        pub fun getSecondarySaleRoyalties(): DimeCollectibleV3.Royalties
    }

    pub resource Release: ReleasePublic {
        pub let id: UInt64

        pub let royaltiesPerShare: UFix64
        pub let numRoyaltyNFTs: UInt64

        // Map from each royalty NFT ID to the current owner.
        // When a royalty NFT is purchased, the corresponding address is updated.
        // When a release NFT is purchased, this list is used to pay the owners of
        // the royalty NFTs
        access(self) let royaltyNFTs: {UInt64: Address}
        pub fun getRoyaltyIds(): [UInt64] {
            return self.royaltyNFTs.keys
        }
        pub fun getRoyaltyOwners(): {UInt64: Address} {
            return self.royaltyNFTs
        }

        pub fun addRoyaltyNFT(id: UInt64) {
            assert(UInt64(self.royaltyNFTs.keys.length) < self.numRoyaltyNFTs,
                message: "This release already has the maximum number of royalty NFTs")
            self.royaltyNFTs[id] = self.owner!.address
            emit RoyaltyNFTAdded(itemId: id)
        }

        // Called whenever a royalty NFT is purchased.
        // Since this is publicly accessible, anyone call call it. We verify
        // that the address given currently owns the specified NFT to make sure that
        // only the actual owner can change the listing
        pub fun updateOwner(id: UInt64, newOwner: Address) {
            let collection = getAccount(newOwner).getCapability
                <&DimeCollectibleV3.Collection{DimeCollectibleV3.DimeCollectionPublic}>
                (DimeCollectibleV3.CollectionPublicPath).borrow()
                ?? panic("Couldn't borrow a capability to the new owner's collection")
            let nft = collection.borrowCollectible(id: id)
            assert(nft != nil, message: "That user doesn't own that NFT")
    
            // We've verified that the provided address does currently own the
            // royalty NFT, so we update the royalty map to reflect this
            self.royaltyNFTs[id] = newOwner
        }

        // A list of the associated release NFTs
        access(self) let releaseNFTs: [UInt64]
        pub fun getReleaseIds(): [UInt64] {
            return self.releaseNFTs
        }
        pub fun addReleaseNFT(id: UInt64) {
            self.releaseNFTs.append(id)
        }
        pub fun removeReleaseNFT(id: UInt64) {
            self.releaseNFTs.remove(at: id)
        }

        // How the proceeds from sales of this release will be divided
        pub let managerFees: UFix64
        access(self) var artistShares: SaleShares
        access(self) var managerShares: SaleShares
        pub fun getArtistShares(): SaleShares {
			return self.artistShares
		}
        pub fun getManagerShares(): SaleShares {
			return self.managerShares
		}

        pub let secondarySaleRoyalties: DimeCollectibleV3.Royalties
        pub fun getSecondarySaleRoyalties():  DimeCollectibleV3.Royalties {
            return self.secondarySaleRoyalties
        }

        pub init(id: UInt64, royaltiesPerShare: UFix64, numRoyaltyNFTs: UInt64,
            managerFees: UFix64, artistShares: SaleShares, managerShares: SaleShares,
            secondarySaleRoyalties: DimeCollectibleV3.Royalties) {
            self.id = id
            self.royaltiesPerShare = royaltiesPerShare
            self.numRoyaltyNFTs = numRoyaltyNFTs
            self.royaltyNFTs = {}
            self.releaseNFTs = []

            assert(managerFees >= 0.1 && managerFees <= 0.9,
                message: "Manager cut must be between 0.1 and 0.9")

            self.managerFees = managerFees
            self.artistShares = artistShares
            self.managerShares = managerShares

            self.secondarySaleRoyalties = secondarySaleRoyalties
        }
    }

    pub resource interface ReleaseCollectionPublic {
        pub fun getReleaseIds(): [UInt64]
        pub fun borrowPublicRelease(id: UInt64): &Release{ReleasePublic}?
    }

    pub resource ReleaseCollection: ReleaseCollectionPublic {
        pub let releases: @{UInt64: Release}

        init() {
            self.releases <- {}
        }
        
        destroy () {
			destroy self.releases
		}

        pub fun getReleaseIds(): [UInt64] {
            return self.releases.keys
        }

        pub fun borrowPublicRelease(id: UInt64): &Release{ReleasePublic}? {
            if self.releases[id] == nil {
                return nil
            }
            return &(self.releases[id]) as &Release{ReleasePublic}
        }

        pub fun borrowPrivateRelease(id: UInt64): &Release? {
            if self.releases[id] == nil {
                return nil
            }
            return &(self.releases[id]) as &Release
        }

        pub fun createRelease(collection: &DimeCollectibleV3.Collection{NonFungibleToken.CollectionPublic},
            totalRoyalties: UFix64, numRoyaltyNFTs: UInt64, tradeable: Bool, managerFees: UFix64,
            artistShares: SaleShares, managerShares: SaleShares, secondarySaleRoyalties: DimeCollectibleV3.Royalties) {
            let minterAddress: Address = 0xf5cdaace879e5a79
            let minterRef = getAccount(minterAddress)
                .getCapability<&DimeCollectibleV3.NFTMinter>(DimeCollectibleV3.MinterPublicPath)
                .borrow()!
            let release <- create Release(id: DimeRoyalties.nextReleaseId, royaltiesPerShare: totalRoyalties / UFix64(numRoyaltyNFTs),
                numRoyaltyNFTs: numRoyaltyNFTs, managerFees: managerFees, artistShares: artistShares,
                managerShares: managerShares, secondarySaleRoyalties: secondarySaleRoyalties)
            let existing <- self.releases[DimeRoyalties.nextReleaseId] <- release
            // This should always be null, but we need to handle this explicitly
            destroy existing

            emit ReleaseCreated(releaseId: DimeRoyalties.nextReleaseId)
            DimeRoyalties.nextReleaseId = DimeRoyalties.nextReleaseId + (1 as UInt64)
        }

        // A release can only be deleted if the creator still owns all the associated royalty
        // and release NFTs. In this case, we delete all of them and then destroy the Release.
        pub fun deleteRelease(releaseId: UInt64,
            collection: Capability<&DimeCollectibleV3.Collection>) {
            let release <- self.releases.remove(key:releaseId)!
            let collectionRef = collection.borrow() ?? panic("Couldn't borrow provided collection")
            let collectionIds = collectionRef.getIDs()
            for id in release.getRoyaltyIds().concat(release.getReleaseIds()) {
                assert(collectionIds.contains(id),
                    message: "Cannot destroy release because another user owns an associated NFT")
            }

            // The creator still owns all the related tokens, so we proceed by burning them
            for id in release.getRoyaltyIds().concat(release.getReleaseIds()) {
                let nft <- collectionRef.withdraw(withdrawID: id)
                destroy nft
            }
            destroy release
        }
    }

    pub fun createReleaseCollection(): @ReleaseCollection {
        return <- create ReleaseCollection()
    }

	init () {
		self.ReleasesStoragePath = /storage/DimeReleases
		self.ReleasesPrivatePath = /private/DimeReleases
		self.ReleasesPublicPath = /public/DimeReleases

        self.nextReleaseId = 0
	}
}
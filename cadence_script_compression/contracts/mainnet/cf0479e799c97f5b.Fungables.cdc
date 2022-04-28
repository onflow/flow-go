import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448

pub contract Fungables: NonFungibleToken {
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64)

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let MinterStoragePath: StoragePath

    pub var totalSupply: UInt64

    pub fun getItemPrice(): UFix64 {
        return 5.0
    }

    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64

        init(id: UInt64) {
            self.id = id
        }

        pub fun name(): String {
            return "One (1) Fungable"
        }

        pub fun description(): String {
            return "Brought to you by Steamclock"
        }

        pub fun getViews(): [Type] {
            return [
                Type<MetadataViews.Display>()
            ]
        }

        pub fun resolveView(_ view: Type): AnyStruct? {
            switch view {
                case Type<MetadataViews.Display>():
                    return MetadataViews.Display(
                        name: self.name(),
                        description: self.description(),
                        thumbnail: MetadataViews.HTTPFile(
                            url: "https://www.fungable.fun/images/fungable-vid.mp4"
                        )
                    )
            }

            return nil
        }
    }

    pub resource interface FungablesCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowFungable(id: UInt64): &Fungables.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Fungable reference: The ID of the returned reference is incorrect"
            }
        }
    }

    pub resource Collection: FungablesCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @Fungables.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        pub fun borrowFungable(id: UInt64): &Fungables.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &Fungables.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }

        init () {
            self.ownedNFTs <- {}
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource NFTMinter {
        pub fun mintNFT(
            recipient: &{NonFungibleToken.CollectionPublic}
        ) {
            // deposit it in the recipient's account using their reference
            recipient.deposit(token: <-create Fungables.NFT(id: Fungables.totalSupply))

            emit Minted(
                id: Fungables.totalSupply
            )

            Fungables.totalSupply = Fungables.totalSupply + (1 as UInt64)
        }
    }

    pub fun fetch(_ from: Address, itemID: UInt64): &Fungables.NFT? {
        let collection = getAccount(from)
            .getCapability(Fungables.CollectionPublicPath)!
            .borrow<&Fungables.Collection{Fungables.FungablesCollectionPublic}>()
            ?? panic("Couldn't get collection")
        // We trust Fungables.Collection.borowFungable to get the correct itemID
        // (it checks it before returning it).
        return collection.borrowFungable(id: itemID)
    }

    init() {
        // Set our named paths
        self.CollectionStoragePath = /storage/fungablesCollectionV1
        self.CollectionPublicPath = /public/fungablesCollectionV1
        self.MinterStoragePath = /storage/fungablesMinterV1

        // Initialize the total supply
        self.totalSupply = 0

        // Create a Minter resource and save it to storage
        let minter <- create NFTMinter()
        self.account.save(<-minter, to: self.MinterStoragePath)

        emit ContractInitialized()
    }
}

/**

OneFootballCollectible.cdc

This smart contract is part of the NFT platform
created by OneFootball.

Author: Louis Guitton
    - email: louis.guitton@onefootball.com
    - discord: laguitte#6016

References:
- OneFootballCollectible relies for the most part on a simple onflow/freshmint NFT contract
  Ref: https://github.com/onflow/freshmint/blob/v2.2.0-alpha.0/src/templates/cadence/contracts/NFT.cdc
- It differs in how metadata is handled: we store media on IPFS and metadata on-chain
  We draw inspiration from what Animoca did for MotoGP: they use a separate contract MotoGPCardMetadata.cdc to make metadata explicit,
  the NFT then stores on a cardID that points to the central MotoGPCardMetadata smart contract, where metadata can be fetched from
  Ref: https://flow-view-source.com/mainnet/account/0xa49cc0ee46c54bfb/contract/MotoGPCardMetadata
- We use a separate contract NFTAirDrop.cdc (taken from onflow/freshmint) to unlock claimable drops
  Ref: https://github.com/onflow/freshmint/blob/v2.2.0-alpha.0/src/templates/cadence/contracts/NFTAirDrop.cdc
- We also referred to VIV3's Collectible contract (with Series/Set/Collectible/Item) but in the end we only
  kept the event emission when an NFT is destroyed from it
  Ref: https://flow-view-source.com/mainnet/account/0x34ba81b8b761306e/contract/Collectible
- We referred to prior art for the metadata naming:
    - Alchemy API NFT metadata view https://github.com/alchemyplatform/alchemy-flow-contracts/blob/main/src/cadence/scripts/getNFTs.cdc#L45
    - NFT Metadata FLIP https://github.com/onflow/flow/blob/master/flips/20210916-nft-metadata.md

**/
import NonFungibleToken from 0x1d7e57aa55817448

pub contract OneFootballCollectible: NonFungibleToken {

    // Events
    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)
    pub event Minted(id: UInt64, templateID: UInt64)
    pub event Destroyed(id: UInt64)
    pub event TemplateEdited(templateID: UInt64)
    pub event TemplateRemoved(templateID: UInt64)

    // Named Paths
    pub let CollectionPublicPath: PublicPath
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPrivatePath: PrivatePath
    pub let AdminStoragePath: StoragePath

    /* totalSupply
    The total number of OneFootballCollectible that have been minted. */
    pub var totalSupply: UInt64

    pub struct Template {
        // the id of the template that holds the metadata and to which NFTs will be pointing
        pub let templateID: UInt64
        // name of the series this template belongs to, e.g.: '2021-xmas-jerseys', 'onefootballerz', or '2022-stars-drop'
        pub let seriesName: String
        // title of the NFT Template
        pub let name: String
        // description of the NFT Template
        pub let description: String
        // IPFS CID of the preview image of the NFT Template
        pub let preview: String
        // IPFS CID of the media file (.jpg, .mp4, .glb or any IPFS supported format)
        pub let media: String
        // data contains all 'other' metadata fields, e.g. team, position, etc
        pub let data: {String: String}

        init(templateID: UInt64, seriesName: String, name: String, description: String, preview: String, media: String, data: {String:String}){
            pre {
                !data.containsKey("name") : "data dictionary contains 'name' key"
                !data.containsKey("description") : "data dictionary contains 'description' key"
                !data.containsKey("media") : "data dictionary contains 'media' key"
            }
            self.templateID = templateID
            self.seriesName = seriesName
            self.name = name
            self.description = description
            self.preview = preview
            self.media = media
            self.data = data
        }
   }

    // Dictionary to hold all metadata with templateID as key
    access(self) let templates: {UInt64: Template}

    pub fun getTemplates(): {UInt64: Template} {
        return self.templates;
    }

    // Get metadata for a specific templateID
    pub fun getTemplate(templateID: UInt64): Template? {
        return self.templates[templateID]
    }

    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64

        pub let templateID: UInt64

        init(templateID: UInt64) {
            OneFootballCollectible.totalSupply = OneFootballCollectible.totalSupply + 1
            self.id = OneFootballCollectible.totalSupply
            self.templateID = templateID

            emit Minted(id: self.id, templateID: self.templateID)
        }

        pub fun getTemplate(): Template? {
            return OneFootballCollectible.getTemplate(templateID: self.templateID)
        }

        destroy() {
            emit Destroyed(id: self.id)
        }
    }

    /* Custom public interface for our collection capability.
    NonFungibleToken.CollectionPublic has borrowNFT but not borrowOneFootballCollectible. */
    pub resource interface OneFootballCollectibleCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowOneFootballCollectible(id: UInt64): &OneFootballCollectible.NFT? {
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow OneFootballCollectible reference: The ID of the returned reference is incorrect"
            }
        }
    }

    /* Collection
    Mostly vanilla Collection resource from the NFT Tutorials.
    Added borrowOneFootballCollectible, pop and size methods following freshmint's NFT contract. */
    pub resource Collection: OneFootballCollectibleCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        /* dictionary of NFTs
        Each NFT is uniquely identified by its `NFT.id: UInt64` field.

        Dictionary definitions don't usually have the @ symbol in the type specification,
        but because the ownedNFTs mapping stores resources, the whole field also has to
        become a resource type, which is why the field has the @ symbol indicating that it is a resource type.
        */
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        /* withdraw
        Remove an NFT from the collection and returns it. */
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        /* deposit
        Deposit an NFT to the collection. */
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @OneFootballCollectible.NFT

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

        /* borrowOneFootballCollectible
        Gets a reference to an NFT in the collection as a OneFootballCollectible. */
        pub fun borrowOneFootballCollectible(id: UInt64): &OneFootballCollectible.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &OneFootballCollectible.NFT
            } else {
                return nil
            }
        }

        /* pop
        Removes and returns the next NFT from the collection.*/
        pub fun pop(): @NonFungibleToken.NFT {
            let nextID = self.ownedNFTs.keys[0]
            return <- self.withdraw(withdrawID: nextID)
        }

        /* size
        Returns the current size of the collection. */
        pub fun size(): Int {
            return self.ownedNFTs.length
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    pub resource interface Minter {
        pub fun mintNFT(templateID: UInt64): @OneFootballCollectible.NFT
    }

    pub resource interface TemplateEditor {
        pub fun setTemplate(templateID: UInt64, seriesName: String, name: String, description: String, preview: String, media: String, data:{String: String})
        pub fun removeTemplate(templateID: UInt64)
    }

    pub resource Admin: Minter, TemplateEditor {
        pub fun mintNFT(templateID: UInt64): @OneFootballCollectible.NFT {
            let nft <- create OneFootballCollectible.NFT(templateID: templateID)
            return <- nft
        }

        pub fun setTemplate(templateID: UInt64, seriesName: String, name: String, description: String, preview: String, media: String, data: {String: String}) {
            let metadata = OneFootballCollectible.Template(templateID: templateID, seriesName: seriesName, name: name, description: description, preview: preview, media: media, data: data)
            OneFootballCollectible.templates[templateID] = metadata
            emit TemplateEdited(templateID: templateID)
        }

        pub fun removeTemplate(templateID: UInt64) {
            OneFootballCollectible.templates.remove(key: templateID)
            emit TemplateRemoved(templateID: templateID)
        }

    }

    pub fun check(_ address: Address): Bool {
        return getAccount(address)
            .getCapability<&{NonFungibleToken.CollectionPublic}>(OneFootballCollectible.CollectionPublicPath)
            .check()
            // else "Collection Reference was not created correctly"
    }

    // fetch
    // Get a reference to a OneFootballCollectible from an account's Collection, if available.
    pub fun fetch(_ from: Address, itemID: UInt64): &OneFootballCollectible.NFT? {
        let collection = getAccount(from)
            .getCapability(OneFootballCollectible.CollectionPublicPath)
            .borrow<&{ OneFootballCollectible.OneFootballCollectibleCollectionPublic }>()
            ?? panic("Couldn't get collection")

        let nft = collection.borrowOneFootballCollectible(id: itemID)
        return nft
    }

    init() {
        self.templates = {}

        // set named paths
        self.CollectionPublicPath = /public/OneFootballCollectibleCollection
        self.CollectionStoragePath = /storage/OneFootballCollectibleCollection
        self.CollectionPrivatePath = /private/OneFootballCollectibleCollection
        self.AdminStoragePath = /storage/OneFootballCollectibleAdmin

        self.totalSupply = 0

        self.account.save(<-OneFootballCollectible.createEmptyCollection(), to: OneFootballCollectible.CollectionStoragePath)

        self.account.link<&OneFootballCollectible.Collection>(OneFootballCollectible.CollectionPrivatePath, target: OneFootballCollectible.CollectionStoragePath)

        self.account.link<&OneFootballCollectible.Collection{NonFungibleToken.CollectionPublic, OneFootballCollectible.OneFootballCollectibleCollectionPublic}>(
            OneFootballCollectible.CollectionPublicPath,
            target: OneFootballCollectible.CollectionStoragePath
        )

        self.account.save(<-create Admin(), to: OneFootballCollectible.AdminStoragePath)

        emit ContractInitialized()
    }
}

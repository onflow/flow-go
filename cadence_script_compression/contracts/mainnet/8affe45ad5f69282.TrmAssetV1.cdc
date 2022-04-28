/**

    TrmAssetV1.cdc

    Description: Contract definitions for initializing Asset Collections, Asset NFT Resource and Asset Minter

    Asset contract is used for defining the Asset NFT, Asset Collection, 
    Asset Collection Public Interface and Asset Minter

    ## `NFT` resource

    The core resource type that represents an Asset NFT in the smart contract.

    ## `Collection` Resource

    The resource that stores a user's Asset NFT collection.
    It includes a few functions to allow the owner to easily
    move tokens in and out of the collection.

    ## `Receiver` resource interfaces

    This interface is used for depositing Asset NFTs to the Asset Collectio.
    It also exposes few functions to fetch data about Asset

    To send an NFT to another user, a user would simply withdraw the NFT
    from their Collection, then call the deposit function on another user's
    Collection to complete the transfer.

    ## `Minter` Resource

    Minter resource is used for minting Asset NFTs

*/

import NonFungibleToken from 0x1d7e57aa55817448

pub contract TrmAssetV1: NonFungibleToken {
    // -----------------------------------------------------------------------
    // Asset contract Event definitions
    // -----------------------------------------------------------------------

    // The total number of tokens of this type in existence
    pub var totalSupply: UInt64
    // Event that emitted when the NFT contract is initialized
    pub event ContractInitialized()
    // Emitted when an Asset is minted
    pub event AssetMinted(id: UInt64, kID: String, serialNumber: UInt64, assetURL: String, assetType: String)
    // Emitted when an Asset is updated
    pub event AssetUpdated(id: UInt64, assetType: String)
    // Emitted when a batch of Assets are minted successfully
    pub event AssetBatchMinted(startId: UInt64, endId: UInt64, kID: String, assetURL: String, assetType: String, totalSerialNumbers: UInt64)
    // Emitted when a batch of Assets are updated successfully
    pub event AssetBatchUpdated(startId: UInt64, endId: UInt64, kID: String, assetType: String)
    // Emitted when an Asset is destroyed
    pub event AssetDestroyed(id: UInt64)
    // Event that is emitted when a token is withdrawn, indicating the owner of the collection that it was withdrawn from. If the collection is not in an account's storage, `from` will be `nil`.
    pub event Withdraw(id: UInt64, from: Address?)
    // Event that emitted when a token is deposited to a collection. It indicates the owner of the collection that it was deposited to.
    pub event Deposit(id: UInt64, to: Address?)

    /// Paths where Storage and capabilities are stored
    pub let collectionStoragePath: StoragePath
    pub let collectionPublicPath: PublicPath
    pub let collectionPrivatePath: PrivatePath
    pub let minterStoragePath: StoragePath

    pub enum AssetType: UInt8 {
        pub case private
        pub case public
    }

    pub fun assetTypeToString(_ assetType: AssetType): String {
        switch assetType {
            case AssetType.private:
                return "private"
            case AssetType.public:
                return "public"
            default:
                return ""
        }
    }

    pub fun stringToAssetType(_ assetTypeStr: String): AssetType {
        switch assetTypeStr {
            case "private":
                return AssetType.private
            case "public":
                return AssetType.public
            default:
                return panic("Asset Type must be \"private\" or \"public\"")
        }
    }

    // AssetData
    //
    // Struct for storing metadata for Asset
    pub struct AssetData {
        pub let kID: String
        pub let serialNumber: UInt64
        pub let assetURL: String
        pub var assetType: AssetType

        access(contract) fun setAssetType(assetTypeStr: String) {
            self.assetType = TrmAssetV1.stringToAssetType(assetTypeStr)
        }

        init(kID: String, serialNumber: UInt64, assetURL: String, assetTypeStr: String) {
            pre {
                serialNumber >= 0: "Serial Number cannot be less than 0"

                kID.length > 0: "KID is invalid"
            }

            self.kID = kID
            self.serialNumber = serialNumber
            self.assetURL = assetURL
            self.assetType = TrmAssetV1.stringToAssetType(assetTypeStr)
        }
    }

    //  NFT
    //
    // The main Asset NFT resource that can be bought and sold by users
    pub resource NFT: NonFungibleToken.INFT {
        pub let id: UInt64
        pub let data: AssetData

        access(contract) fun setAssetType(assetType: String) {
            self.data.setAssetType(assetTypeStr: assetType)

            emit AssetUpdated(id: self.id, assetType: assetType)
        }

        init(id: UInt64, kID: String, serialNumber: UInt64, assetURL: String, assetType: String) {
            self.id = id
            self.data = AssetData(kID: kID, serialNumber: serialNumber, assetURL: assetURL, assetTypeStr: assetType)

            emit AssetMinted(id: self.id, kID: kID, serialNumber: serialNumber, assetURL: assetURL, assetType: assetType)
        }

        // If the Asset NFT is destroyed, emit an event to indicate to outside observers that it has been destroyed
        destroy() {
            emit AssetDestroyed(id: self.id)
        }
    }

    // CollectionPublic
    //
    // Public interface for Asset Collection
    // This exposes functions for depositing NFTs
    // and also for returning some info for a specific
    // Asset NFT id
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowAsset(id: UInt64): &NFT
        pub fun idExists(id: UInt64): Bool
        pub fun getKID(id: UInt64): String
        pub fun getSerialNumber(id: UInt64): UInt64
        pub fun getAssetURL(id: UInt64): String
        pub fun getAssetType(id: UInt64): String
    }

    // Collection
    //
    // The resource that stores a user's Asset NFT collection.
    // It includes a few functions to allow the owner to easily
    // move tokens in and out of the collection.
    pub resource Collection: NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, CollectionPublic {
        
        // Dictionary to hold the NFTs in the Collection
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init() {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("Error withdrawing Asset NFT")
            emit Withdraw(id: withdrawID, from: self.owner!.address)
            return <-token
        }

        // deposit takes an NFT as an argument and adds it to the Collection
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let assetToken <- token as! @NFT
            emit Deposit(id: assetToken.id, to: self.owner!.address)
            let oldToken <- self.ownedNFTs[assetToken.id] <- assetToken
            destroy oldToken
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // Returns a borrowed reference to an NFT in the collection so that the caller can read data and call methods from it
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // Returns a borrowed reference to the Asset in the collection so that the caller can read data and call methods from it
        pub fun borrowAsset(id: UInt64): &NFT {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            return refNFT as! &NFT
        }

        // Checks if id of NFT exists in collection
        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }

        // Returns the asset ID for an NFT in the collection
        pub fun getKID(id: UInt64): String {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let refAssetNFT = refNFT as! &NFT
            return refAssetNFT.data.kID
        }

        // Returns the serial number for an NFT in the collection
        pub fun getSerialNumber(id: UInt64): UInt64 {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let refAssetNFT = refNFT as! &NFT
            return refAssetNFT.data.serialNumber
        }

        // Returns the asset URL for an NFT in the collection
        pub fun getAssetURL(id: UInt64): String {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let refAssetNFT = refNFT as! &NFT
            return refAssetNFT.data.assetURL
        }

        // Returns the asset type for an NFT in the collection
        pub fun getAssetType(id: UInt64): String {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let refAssetNFT = refNFT as! &NFT
            return TrmAssetV1.assetTypeToString(refAssetNFT.data.assetType)
        }

        // Sets the asset type. Only the owner of the asset is allowed to set this
        pub fun setAssetType(id: UInt64, assetType: String) {
            pre {
                assetType == "private" || assetType == "public": 
                    "Asset Type must be private or public"
                
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }

            let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let refAssetNFT = refNFT as! &NFT
            refAssetNFT.setAssetType(assetType: assetType)
        }

        // Sets the asset type of multiple tokens. Only the owner of the asset is allowed to set this
        pub fun batchSetAssetType(ids: [UInt64], kID: String, assetType: String) {
            pre {
                assetType == "private" || assetType == "public":
                    "Asset Type must be private or public"

                ids.length > 0: "Total length of ids cannot be less than 1"
            }

            for id in ids {
                if self.idExists(id: id) {
                    let refNFT = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                    let refAssetNFT = refNFT as! &NFT
                    if (refAssetNFT.data.kID != kID) {
                        panic("Asset Token ID and KID do not match")
                    }
                    refAssetNFT.setAssetType(assetType: assetType)
                } else {
                    panic("Asset Token ID ".concat(id.toString()).concat(" not owned"))
                }
            }

            emit AssetBatchUpdated(startId: ids[0], endId: ids[ids.length-1], kID: kID, assetType: assetType)
        }

        // Destroys specified token in the collection
        pub fun destroyToken(id: UInt64) {
            pre {
                self.ownedNFTs[id] != nil: "Asset Token ID does not exist"
            }
            let oldToken <- self.ownedNFTs.remove(key: id)
            destroy oldToken
        }

        // If a transaction destroys the Collection object, all the NFTs contained within are also destroyed!
        destroy() {
            destroy self.ownedNFTs
        }
    }

    // createEmptyCollection creates an empty Collection and returns it to the caller so that they can own NFTs
    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    // Minter
    //
    // Minter is a special resource that is used for minting Assets
    pub resource Minter {

        // mintNFT mints the asset NFT and stores it in the collection of recipient
        pub fun mintNFT(kID: String, serialNumber: UInt64, assetURL: String, assetType: String, recipient: &TrmAssetV1.Collection{TrmAssetV1.CollectionPublic, NonFungibleToken.CollectionPublic}): UInt64 {
            pre {
                assetType == "private" || assetType == "public":
                    "Asset Type must be private or public"

                serialNumber >= 0: "Serial Number cannot be less than 0"

                kID.length > 0: "KID is invalid"
            }

            let tokenID = TrmAssetV1.totalSupply

            recipient.deposit(token: <- create NFT(id: tokenID, kID: kID, serialNumber: serialNumber, assetURL: assetURL, assetType: assetType))

            TrmAssetV1.totalSupply = tokenID + 1
            
            return TrmAssetV1.totalSupply
        }

        // batchMintNFTs mints the asset NFT in batch and stores it in the collection of recipient
        pub fun batchMintNFTs(kID: String, totalSerialNumbers: UInt64, assetURL: String, assetType: String, recipient: &TrmAssetV1.Collection{TrmAssetV1.CollectionPublic, NonFungibleToken.CollectionPublic}): UInt64 {
            pre {
                assetType == "private" || assetType == "public":
                    "Asset Type must be private or public"

                totalSerialNumbers > 0: "Total Serial Numbers cannot be less than 1"

                assetURL.length > 0: "Asset URL is invalid"

                kID.length > 0: "KID is invalid"
            }

            let startTokenID = TrmAssetV1.totalSupply
            var tokenID = startTokenID
            var counter: UInt64 = 0

            while counter < totalSerialNumbers {

                recipient.deposit(token: <- create NFT(id: tokenID, kID: kID, serialNumber: counter, assetURL: assetURL, assetType: assetType))

                counter = counter + 1
                tokenID = tokenID + 1
            }

            let endTokenID = tokenID - 1

            emit AssetBatchMinted(startId: startTokenID, endId: endTokenID, kID: kID, assetURL: assetURL, assetType: assetType, totalSerialNumbers: totalSerialNumbers)

            TrmAssetV1.totalSupply = tokenID
            
            return TrmAssetV1.totalSupply
        }
    }

    init() {
        self.totalSupply = 0

        self.collectionStoragePath = /storage/trmAssetV1Collection
        self.collectionPublicPath = /public/trmAssetV1Collection
        self.collectionPrivatePath = /private/trmAssetV1Collection
        self.minterStoragePath = /storage/trmAssetV1Minter

        // First, check to see if a minter resource already exists
        if self.account.borrow<&TrmAssetV1.Minter>(from: self.minterStoragePath) == nil {
            
            // Put the minter in storage with access only to admin
            self.account.save(<-create Minter(), to: self.minterStoragePath)

        }

        emit ContractInitialized()
    }
}
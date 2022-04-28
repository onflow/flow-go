import TopShot from 0x0b2a3299cc857e29

pub contract Ashes {
    pub var nextAshSerial: UInt64
    pub var allowMint: Bool
    pub var latestBroadcast: Message?

    pub struct Message {
        pub let subject: String
        pub let payload: String
        pub let encoding: String

        init(subject: String, payload: String, encoding: String) {
            self.subject = subject
            self.payload = payload
            self.encoding = encoding
        }
    }

    pub event AshMinted(id: UInt64, ashSerial: UInt64, setID: UInt32, playID: UInt32, topshotSerial: UInt32)
    pub event AshWithdrawn(id: UInt64, from: Address?)
    pub event AshDeposited(id: UInt64, to: Address?)
    pub event AshDestroyed(id: UInt64)
    pub event BroadcastMessage(subject: String, payload: String, encoding: String)
    pub event AllowMintToggled(allowMint: Bool)

    // Declare the NFT resource type
    pub resource Ash {
        // The unique ID that differentiates each NFT
        pub let id: UInt64
        pub let momentData: TopShot.MomentData
        pub let ashSerial: UInt64

        // Initialize both fields in the init function
        init(initID: UInt64, momentData: TopShot.MomentData) {
            if !Ashes.allowMint {
                panic("minting is closed")
            }
            self.id = initID
            self.momentData = momentData
            self.ashSerial = Ashes.nextAshSerial
            Ashes.nextAshSerial = Ashes.nextAshSerial + 1
            emit AshMinted(id: initID, ashSerial: self.ashSerial, setID: momentData.setID, playID: momentData.playID, topshotSerial: momentData.serialNumber)
        }

        destroy() {
            emit AshDestroyed(id: self.id)
        }
    }

    // We define this interface purely as a way to allow users
    // to create public, restricted references to their NFT Collection.
    // They would use this to only expose the deposit, getIDs,
    // and idExists fields in their Collection
    pub resource interface AshReceiver {

        pub fun deposit(token: @Ash)

        pub fun getIDs(): [UInt64]

        pub fun idExists(id: UInt64): Bool

        pub fun borrowAsh(id: UInt64): &Ash? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow ash reference: The ID of the returned reference is incorrect"
            }
        }

    }


    // The definition of the Collection resource that
    // holds the NFTs that a user owns
    pub resource Collection: AshReceiver {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: Ash}

        // Initialize the NFTs field to an empty collection
        init () {
            self.ownedNFTs <- {}
        }

        // withdraw 
        //
        // Function that removes an NFT from the collection 
        // and moves it to the calling context
        pub fun withdraw(withdrawID: UInt64): @Ash {
            // If the NFT isn't found, the transaction panics and reverts
            let token <- self.ownedNFTs.remove(key: withdrawID)!
            emit AshWithdrawn(id: token.id, from: self.owner?.address)
            return <-token
        }

        // deposit 
        //
        // Function that takes a NFT as an argument and 
        // adds it to the collections dictionary
        pub fun deposit(token: @Ash) {
            // add the new token to the dictionary with a force assignment
            // if there is already a value at that key, it will fail and revert
            emit AshDeposited(id: token.id, to: self.owner?.address)
            self.ownedNFTs[token.id] <-! token
        }

        // idExists checks to see if a NFT 
        // with the given ID exists in the collection
        pub fun idExists(id: UInt64): Bool {
            return self.ownedNFTs[id] != nil
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        pub fun borrowAsh(id: UInt64): &Ash? {
            return &self.ownedNFTs[id] as! &Ash
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // creates a new empty Collection resource and returns it 
    pub fun createEmptyCollection(): @Collection {
        return <- create Collection()
    }

    pub fun mint(topshotNFT: @TopShot.NFT): @Ash {
        let res <- create Ash(initID: topshotNFT.id, momentData: topshotNFT.data)
        destroy topshotNFT
        return <-res
    }

    pub resource Admin {
        pub fun createAdmin(): @Admin {
            return <- create Admin()
        }

        pub fun toggleAllowMint(allowMint: Bool) {
            Ashes.allowMint = allowMint
            emit AllowMintToggled(allowMint: allowMint)
        }

        pub fun broadcast(msg: Message) {
            Ashes.latestBroadcast = msg
            emit BroadcastMessage(subject: msg.subject, payload: msg.payload, encoding: msg.encoding)
        }
    }


    init() {
        self.nextAshSerial = 1
        self.allowMint = false
        self.latestBroadcast = nil

        self.account.save<@Admin>(<- create Admin(), to: /storage/AshesAdmin)
    }

}
 

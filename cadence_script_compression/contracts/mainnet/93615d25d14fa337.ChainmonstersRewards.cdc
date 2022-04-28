// This is the Kickstarter/Presale NFT contract of Chainmonsters.
// Based on the "current" NonFungibleToken standard on Flow.
// Does not include that much functionality as the only purpose it to mint and store the Presale NFTs.

import NonFungibleToken from 0x1d7e57aa55817448

pub contract ChainmonstersRewards: NonFungibleToken {

    pub var totalSupply: UInt64

    pub event ContractInitialized()
    pub event Withdraw(id: UInt64, from: Address?)
    pub event Deposit(id: UInt64, to: Address?)

    pub event RewardCreated(id: UInt32, metadata: String, season: UInt32)
    pub event NFTMinted(NFTID: UInt64, rewardID: UInt32, serialNumber: UInt32)
    pub event NewSeasonStarted(newCurrentSeason: UInt32)

    pub event ItemConsumed(itemID: UInt64, playerId: String)
    pub event ItemClaimed(itemID: UInt64, playerId: String, uid: String)

    pub var nextRewardID: UInt32

    // Variable size dictionary of Reward structs
    access(self) var rewardDatas: {UInt32: Reward}
    access(self) var rewardSupplies: {UInt32: UInt32}
    access(self) var rewardSeasons: {UInt32 : UInt32}

    // a mapping of Reward IDs that indicates what serial/mint number
    // have been minted for this specific Reward yet
    pub var numberMintedPerReward: {UInt32: UInt32}

    // the season a reward belongs to
    // A season is a concept where rewards are obtainable in-game for a limited time
    // After a season is over the rewards can no longer be minted and thus create
    // scarcity and drive the player-driven economy over time.
    pub var currentSeason: UInt32



    // A reward is a struct that keeps all the metadata information from an NFT in place.
    // There are 19 different rewards and all need an NFT-Interface.
    // Depending on the Reward-Type there are different ways to use and interact with future contracts.
    // E.g. the "Alpha Access" NFT needs to be claimed in order to gain game access with your account.
    // This process is destroying/moving the NFT to another contract.
    pub struct Reward {

        // The unique ID for the Reward
        pub let rewardID: UInt32

        // the game-season this reward belongs to
        // Kickstarter NFTs are Pre-Season and equal 0
        pub let season: UInt32

        // The metadata for the rewards is restricted to the name since
        // all other data is inside the token itself already
        // visual stuff and descriptions need to be retrieved via API
        pub let metadata: String

        init(metadata: String) {
            pre {
                metadata.length != 0: "New Reward metadata cannot be empty"
            }
            self.rewardID = ChainmonstersRewards.nextRewardID
            self.metadata = metadata
            self.season = ChainmonstersRewards.currentSeason;

            // Increment the ID so that it isn't used again
            ChainmonstersRewards.nextRewardID = ChainmonstersRewards.nextRewardID + UInt32(1)

            emit RewardCreated(id: self.rewardID, metadata: metadata, season: self.season)
        }
    }

     pub struct NFTData {

        // The ID of the Reward that the NFT references
        pub let rewardID: UInt32

        // The token mint number
        // Otherwise known as the serial number
        pub let serialNumber: UInt32

        init(rewardID: UInt32, serialNumber: UInt32) {
            self.rewardID = rewardID
            self.serialNumber = serialNumber
        }

    }


    pub resource NFT: NonFungibleToken.INFT {
        
        // Global unique NFT ID
        pub let id: UInt64

        pub let data: NFTData

        init(serialNumber: UInt32, rewardID: UInt32) {
            // Increment the global NFT IDs
            ChainmonstersRewards.totalSupply = ChainmonstersRewards.totalSupply + UInt64(1)
            
            self.id = ChainmonstersRewards.totalSupply 

            self.data = NFTData(rewardID: rewardID, serialNumber: serialNumber)

            emit NFTMinted(NFTID: self.id, rewardID: rewardID, serialNumber: self.data.serialNumber)
        }
    }

    // This is the interface that users can cast their Reward Collection as
    // to allow others to deposit Rewards into their Collection. It also allows for reading
    // the IDs of Rewards in the Collection.
    pub resource interface ChainmonstersRewardCollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowReward(id: UInt64): &ChainmonstersRewards.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id): 
                    "Cannot borrow Reward reference: The ID of the returned reference is incorrect"
            }
        }
    }


    pub resource Collection: ChainmonstersRewardCollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic {
        // dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT-Reward from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {

            // Remove the nft from the Collection
            let token <- self.ownedNFTs.remove(key: withdrawID) 
                ?? panic("Cannot withdraw: Reward does not exist in the collection")

            emit Withdraw(id: token.id, from: self.owner?.address)
            
            // Return the withdrawn token
            return <-token
        }


        // batchWithdraw withdraws multiple tokens and returns them as a Collection
        //
        // Parameters: ids: An array of IDs to withdraw
        //
        // Returns: @NonFungibleToken.Collection: A collection that contains
        //                                        the withdrawn rewards
        //
        pub fun batchWithdraw(ids: [UInt64]): @NonFungibleToken.Collection {
            // Create a new empty Collection
            var batchCollection <- create Collection()
            
            // Iterate through the ids and withdraw them from the Collection
            for id in ids {
                batchCollection.deposit(token: <-self.withdraw(withdrawID: id))
            }
            
            // Return the withdrawn tokens
            return <-batchCollection
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @ChainmonstersRewards.NFT

            let id: UInt64 = token.id

            // add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
        }

        // batchDeposit takes a Collection object as an argument
        // and deposits each contained NFT into this Collection
        pub fun batchDeposit(tokens: @NonFungibleToken.Collection) {

            // Get an array of the IDs to be deposited
            let keys = tokens.getIDs()

            // Iterate through the keys in the collection and deposit each one
            for key in keys {
                self.deposit(token: <-tokens.withdraw(withdrawID: key))
            }

            // Destroy the empty Collection
            destroy tokens
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedNFTs.keys
        }

        // borrowNFT gets a reference to an NFT in the collection
        // so that the caller can read its metadata and call its methods
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT {
            return &self.ownedNFTs[id] as &NonFungibleToken.NFT
        }

        // borrowMReward returns a borrowed reference to a Reward
        // so that the caller can read data and call methods from it.
        //
        // Parameters: id: The ID of the NFT to get the reference for
        //
        // Returns: A reference to the NFT
        pub fun borrowReward(id: UInt64): &ChainmonstersRewards.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &ChainmonstersRewards.NFT
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }




    // Resource that an admin or something similar would own to be
    // able to mint new NFTs
    //
	pub resource Admin {

        
        // creates a new Reward struct and stores it in the Rewards dictionary
        // Parameters: metadata: the name of the reward
        pub fun createReward(metadata: String, totalSupply: UInt32): UInt32 {
            // Create the new Reward
            var newReward = Reward(metadata: metadata)
            let newID = newReward.rewardID;

            // Kickstarter rewards are created with a fixed total supply.
            // Future season rewards are not technically limited by a total supply
            // but rather the time limitations in which a player can earn those.
            // Once a season is over the total supply for those rewards is fixed since
            // they can no longer be minted.

            ChainmonstersRewards.rewardSupplies[newID] = totalSupply
            ChainmonstersRewards.numberMintedPerReward[newID] = 0
            ChainmonstersRewards.rewardSeasons[newID] = newReward.season

            ChainmonstersRewards.rewardDatas[newID] = newReward

            return newID
        }
        
        // consuming an NFT (item) to be converted to in-game economy
        pub fun consumeItem(token: @NonFungibleToken.NFT, playerId: String) {
            let token <- token as! @ChainmonstersRewards.NFT

            let id: UInt64 = token.id

            emit ItemConsumed(itemID: id, playerId: playerId)

            destroy token
        }

        // claiming an NFT item from e.g. Season Pass or Store
        // rewardID - reward to be claimed
        // uid - unique identifier from system
        pub fun claimItem(rewardID: UInt32, playerId: String, uid: String): @NFT {
            let nft <- self.mintReward(rewardID: rewardID)

            emit ItemClaimed(itemID: nft.id, playerId: playerId, uid: uid)

            return <- nft
        }


		// mintReward mints a new NFT-Reward with a new ID
		// 
		pub fun mintReward(rewardID: UInt32): @NFT {
            pre {

                // check if the reward is still in "season"
                ChainmonstersRewards.rewardSeasons[rewardID] == ChainmonstersRewards.currentSeason
                // check if total supply allows additional NFTs || ignore if there is no hard cap specified == 0
                ChainmonstersRewards.numberMintedPerReward[rewardID] != ChainmonstersRewards.rewardSupplies[rewardID] || ChainmonstersRewards.rewardSupplies[rewardID] == UInt32(0)

            }

            // Gets the number of NFTs that have been minted for this Reward
            // to use as this NFT's serial number
            let numInReward = ChainmonstersRewards.numberMintedPerReward[rewardID]!

            // Mint the new NFT
            let newReward: @NFT <- create NFT(serialNumber: numInReward + UInt32(1),
                                              rewardID: rewardID)

            // Increment the count of NFTs minted for this Reward
            ChainmonstersRewards.numberMintedPerReward[rewardID] = numInReward + UInt32(1)

            return <-newReward
		}

        // batchMintReward mints an arbitrary quantity of Rewards 
        // 
        pub fun batchMintReward(rewardID: UInt32, quantity: UInt64): @Collection {
            let newCollection <- create Collection()

            var i: UInt64 = 0
            while i < quantity {
                newCollection.deposit(token: <-self.mintReward(rewardID: rewardID))
                i = i + UInt64(1)
            }

            return <-newCollection
        }

        pub fun borrowReward(rewardID: UInt32): &Reward {
            pre {
                ChainmonstersRewards.rewardDatas[rewardID] != nil: "Cannot borrow Reward: The Reward doesn't exist"
            }
            
            // Get a reference to the Set and return it
            // use `&` to indicate the reference to the object and type
            return &ChainmonstersRewards.rewardDatas[rewardID] as &Reward
        }


        // ends the current season by incrementing the season number
        // Rewards minted after this will use the new season number.
        pub fun startNewSeason(): UInt32 {
            ChainmonstersRewards.currentSeason = ChainmonstersRewards.currentSeason + UInt32(1)

            emit NewSeasonStarted(newCurrentSeason: ChainmonstersRewards.currentSeason)

            return ChainmonstersRewards.currentSeason
        }


         // createNewAdmin creates a new Admin resource
        //
        pub fun createNewAdmin(): @Admin {
            return <-create Admin()
        }
	}



    // -----------------------------------------------------------------------
    // ChainmonstersRewards contract-level function definitions
    // -----------------------------------------------------------------------

    // public function that anyone can call to create a new empty collection
    // This is required to receive Rewards in transactions.
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create ChainmonstersRewards.Collection()
    }

    // returns all the rewards setup in this contract
    pub fun getAllRewards(): [ChainmonstersRewards.Reward] {
        return ChainmonstersRewards.rewardDatas.values
    }

    // returns returns all the metadata associated with a specific Reward
    pub fun getRewardMetaData(rewardID: UInt32): String? {
        return self.rewardDatas[rewardID]?.metadata
    }


    // returns the season this specified reward belongs to
    pub fun getRewardSeason(rewardID: UInt32): UInt32? {
        return ChainmonstersRewards.rewardDatas[rewardID]?.season
    }


     // isRewardLocked returns a boolean that indicates if a Reward
    //                      can no longer be minted.
    // 
    // Parameters: rewardID: The id of the Set that is being searched
    //             
    //
    // Returns: Boolean indicating if the reward is locked or not
    pub fun isRewardLocked(rewardID: UInt32): Bool? {
        // Don't force a revert if the reward is invalid
        if (ChainmonstersRewards.rewardSupplies[rewardID] == ChainmonstersRewards.numberMintedPerReward[rewardID]) {

            return true
        } else {

            // If the Reward wasn't found , return nil
            return nil
        }
    }

    // returns the number of Rewards that have been minted already
    pub fun getNumRewardsMinted(rewardID: UInt32): UInt32? {
        let amount = ChainmonstersRewards.numberMintedPerReward[rewardID]

        return amount
    }



	init() {
        // Initialize contract fields
        self.rewardDatas = {}
        self.nextRewardID = 1
        self.totalSupply = 0
        self.rewardSupplies = {}
        self.numberMintedPerReward = {}
        self.currentSeason = 0
        self.rewardSeasons = {}

         // Put a new Collection in storage
        self.account.save<@Collection>(<- create Collection(), to: /storage/ChainmonstersRewardCollection)

        // Create a public capability for the Collection
        self.account.link<&{ChainmonstersRewardCollectionPublic}>(/public/ChainmonstersRewardCollection, target: /storage/ChainmonstersRewardCollection)

        // Put the Minter in storage
        self.account.save<@Admin>(<- create Admin(), to: /storage/ChainmonstersAdmin)

        emit ContractInitialized()
	}
}

 
 

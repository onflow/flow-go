// Implementation of the DayNFT contract

import NonFungibleToken from 0x1d7e57aa55817448
import MetadataViews from 0x1d7e57aa55817448
import DateUtils from 0xc0bcca6fd0fe81b0

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

pub contract DayNFT: NonFungibleToken {

    // Named Paths
    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath
    pub let AdminPublicPath: PublicPath

    // Total number of NFTs in circulation
    pub var totalSupply: UInt64

    // Resource containing data and logic around bids, minting and distribution
    access(contract) let manager: @ContractManager

    // Event emitted when the contract is initialized
    pub event ContractInitialized()

    // Event emitted when users withdraw from their NFT collection
    pub event Withdraw(id: UInt64, from: Address?)

    // Event emitted when users deposit into their NFT collection
    pub event Deposit(id: UInt64, to: Address?)

    // Event emitted when a new NFT is minted
    pub event Minted(id: UInt64, date: String, title: String)

    // Event emitted when a user makes a bid
    pub event BidReceived(user: Address, date: DateUtils.Date, title: String)

    // Resource containing data and logic around bids, minting and distribution
    pub resource ContractManager {
        // NFTs that can be claimed by users that won previous days' auction(s)
        access(self) var NFTsDue: @{Address: [NFT]}

        // Amounts of Flow available to be redistributed to each NFT holder
        access(self) var amountsDue: {UInt64: UFix64}
        
        // Percentage of any amount of Flow deposited to this contract that gets 
        // redistributed to NFT holders
        access(self) let percentageDistributed: UFix64

        // Vault to be used for flow redistribution
        access(self) let distributeVault: @FlowToken.Vault

        // Best bid of the day for minting today's NFT
        access(self) var bestBid: @Bid

        // NFT minter
        access(self) let minter: @NFTMinter

        // Contract address used for receiving tokens
        access(self) let contractAddress: Address

        init(contractAddress: Address) {
            self.amountsDue = {}
            self.NFTsDue <- {}

            self.percentageDistributed = 0.5
            self.distributeVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault  
            
            // Initialize dummy best bid
            let vault <- FlowToken.createEmptyVault() as! @FlowToken.Vault        
            let date = DateUtils.Date(day: 1, month: 1, year: 2021) 
            self.bestBid <- create Bid(vault: <- vault, 
                        recipient: Address(0x0),
                        title: "",
                        date: date)

            // Create a Minter resource and keep it in the resource (not accessible from outside the contract)
            self.minter <- create NFTMinter()

            self.contractAddress = contractAddress
        }

        // Get the best bid for today's auction
        pub fun getBestBidWithToday(today: DateUtils.Date): PublicBid {
            if (today.equals(self.bestBid.date)) {
                return PublicBid(amount: self.bestBid.vault.balance,
                                    user: self.bestBid.recipient,
                                    date: today)
            } else {
                return PublicBid(amount: 0.0,
                                    user: Address(0x0),
                                    date: today)
            }
        }

        // Verify if a user has any NFTs to claim after winning one or more auctions
        pub fun nbNFTsToClaimWithToday(address: Address, today: DateUtils.Date): Int {
            var res = 0
            if(self.NFTsDue[address] != nil) {
                res = self.NFTsDue[address]?.length!
            }
            if(!self.bestBid.date.equals(today) && self.bestBid.recipient == address) {
                res = res + 1
            }
            return res
        }

        // Handle new incoming bid
        pub fun handleBid(newBid: @Bid, today: DateUtils.Date) {
            var bid <- newBid
            if(self.bestBid.date.equals(today) || self.bestBid.vault.balance == 0.0) {
                if(bid.vault.balance > self.bestBid.vault.balance) {
                    if(self.bestBid.vault.balance > 0.0) {
                        // Refund current best bid and replace it with the new one
                        let rec = getAccount(self.bestBid.recipient).getCapability(/public/flowTokenReceiver)
                                    .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
                                    ?? panic("Could not borrow a reference to the receiver")
                        var tempVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
                        tempVault <-> self.bestBid.vault
                        rec.deposit(from: <- tempVault)
                    }
                    bid <-> self.bestBid
                } else {
                    // Refund the new bid
                    let rec = getAccount(bid.recipient).getCapability(/public/flowTokenReceiver)
                                .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
                                ?? panic("Could not borrow a reference to the receiver")
                    var tempVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
                    tempVault <-> bid.vault
                    rec.deposit(from: <- tempVault)
                }
            } else {
                // This is the first bid of the day
                // Assign NFT to best yesterday's bidder and replace today's bestBid with new bid
                // Deposit flow into contract account for redistribution
                var tempVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
                tempVault <-> self.bestBid.vault
                self.deposit(vault: <- tempVault)

                // Mint the NFT
                self.amountsDue[DayNFT.totalSupply] = 0.0
                let newNFT <- self.minter.mintNFT(date: self.bestBid.date, title: self.bestBid.title)
                // Record into due NFTs
                if(self.NFTsDue[self.bestBid.recipient] == nil) {
                    let newArray <- [<-newNFT]
                    self.NFTsDue[self.bestBid.recipient] <-! newArray
                } else {
                    var newArray: @[NFT] <- []
                    var a = 0
                    var len = self.NFTsDue[self.bestBid.recipient]?.length!
                    while a < len {
                        let nft <- self.NFTsDue[self.bestBid.recipient]?.removeFirst()!
                        newArray.append(<-nft)
                        a = a + 1
                    }
                    newArray.append(<-newNFT)
                    let old <- self.NFTsDue.remove(key: self.bestBid.recipient)
                    destroy old
                    self.NFTsDue[self.bestBid.recipient] <-! newArray
                }

                // Replace bid
                self.bestBid <-> bid
            }
            destroy bid
        }

        // Claim NFTs due to the user, and deposit them into their collection
        pub fun claimNFTsWithToday(address: Address, today: DateUtils.Date): Int {
            var res = 0
            let receiver = getAccount(address)
                .getCapability(DayNFT.CollectionPublicPath)
                .borrow<&{DayNFT.CollectionPublic}>()
                ?? panic("Could not get receiver reference to the NFT Collection")

            if(self.NFTsDue[address] != nil) {
                var a = 0
                let len = self.NFTsDue[address]?.length!
                while a < len {
                    let nft <- self.NFTsDue[self.bestBid.recipient]?.removeFirst()!
                    receiver.deposit(token: <-nft)
                    a = a + 1
                }
                res = len
            }
            if(!self.bestBid.date.equals(today) && self.bestBid.recipient == address) {
                // Deposit flow to contract account
                var tempVault <- FlowToken.createEmptyVault() as! @FlowToken.Vault
                tempVault <-> self.bestBid.vault
                self.deposit(vault: <- tempVault)

                // Mint the NFT and send it
                self.amountsDue[DayNFT.totalSupply] = 0.0
                let newNFT <- self.minter.mintNFT(date: self.bestBid.date, title: self.bestBid.title)
                receiver.deposit(token: <-newNFT)
                
                // Replace old best bid with a dummy one with zero balance for today
                let vault <- FlowToken.createEmptyVault() as! @FlowToken.Vault 
                var bid <- create Bid(vault: <- vault, 
                    recipient: Address(0x0),
                    title: "",
                    date: today)
                self.bestBid <-> bid
                destroy bid
                res = res + 1
            }
            return res
        }

        // Get amount of Flow due to the user
        pub fun tokensToClaim(address: Address): UFix64 {
            // Borrow the recipient's public NFT collection reference
            let holder = getAccount(address)
                        .getCapability(DayNFT.CollectionPublicPath)
                        .borrow<&DayNFT.Collection{DayNFT.CollectionPublic}>()
                        ?? panic("Could not get receiver reference to the NFT Collection")

            // Compute amount due based on number of NFTs detained
            var amountDue = 0.0
            for id in holder.getIDs() {
                amountDue = amountDue + self.amountsDue[id]!
            }
            return amountDue
        }

        // Claim Flow due to the user
        pub fun claimTokens(address: Address): UFix64 {
            // Borrow the recipient's public NFT collection reference
            let holder = getAccount(address)
                        .getCapability(DayNFT.CollectionPublicPath)
                        .borrow<&DayNFT.Collection{DayNFT.CollectionPublic}>()
                        ?? panic("Could not get receiver reference to the NFT Collection")

            // Borrow the recipient's flow token receiver
            let receiver = getAccount(address).getCapability(/public/flowTokenReceiver)
                            .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
                            ?? panic("Could not borrow a reference to the receiver")

            // Compute amount due based on number of NFTs detained
            var amountDue = 0.0
            for id in holder.getIDs() {
                amountDue = amountDue + self.amountsDue[id]!
                self.amountsDue[id] = 0.0
            }

            // Pay amount
            let vault <- self.distributeVault.withdraw(amount: amountDue)
            receiver.deposit(from: <- vault)

            return amountDue
        }

        pub fun max(_ a: UFix64, _ b: UFix64): UFix64 {
            var res = a
            if (b > a){
                res = b
            }
            return res
        }

        // Deposit Flow into the contract, to be redistributed among NFT holders
        pub fun deposit(vault: @FungibleToken.Vault) {
            let amount = vault.balance
            var distribute = amount * self.percentageDistributed
            if (DayNFT.totalSupply == 0) {
                distribute = 0.0
            }
            let distrVault <- vault.withdraw(amount: distribute)
            self.distributeVault.deposit(from: <- distrVault)
            // Deposit to the account
            let rec = getAccount(self.contractAddress).getCapability(/public/flowTokenReceiver) 
                        .borrow<&FlowToken.Vault{FungibleToken.Receiver}>()
                        ?? panic("Could not borrow a reference to the Flow receiver")
            rec.deposit(from: <- vault)

            // Assign part of the value to the current holders
            let id = DayNFT.totalSupply
            let distributeEach = distribute / self.max(UFix64(id), 1.0)
            var a = 0 as UInt64
            while a < id {
                self.amountsDue[a] = self.amountsDue[a]! + distributeEach
                a = a + 1
            }
        }

        destroy() {
            destroy self.NFTsDue
            destroy self.bestBid
            destroy self.minter
            destroy self.distributeVault
        }
    }

    // Standard NFT resource
    pub resource NFT: NonFungibleToken.INFT, MetadataViews.Resolver {
        pub let id: UInt64
        pub let name: String
        pub let description: String
        pub let thumbnail: String

        pub let title: String
        pub let date: DateUtils.Date
        pub let dateStr: String

        init(initID: UInt64, date: DateUtils.Date, title: String) {
            self.dateStr = date.toString()

            self.id = initID
            self.name = "DAY-NFT #".concat(self.dateStr)
            self.description = "Minted on day-nft.io on ".concat(self.dateStr)
            self.thumbnail = "https://day-nft.io/imgs/".concat(initID.toString()).concat(".png")

            self.title = title
            self.date = date

            emit Minted(id: initID, date: date.toString(), title: title)
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
                        name: self.name,
                        description: self.description,
                        thumbnail: MetadataViews.HTTPFile(
                            url: self.thumbnail
                        )
                    )
            }

            return nil
        }
    }

    // This is the interface that users can cast their DayNFT Collection as
    // to allow others to deposit DayNFT into their Collection. It also allows for reading
    // the details of a DayNFT in the Collection.
    pub resource interface CollectionPublic {
        pub fun deposit(token: @NonFungibleToken.NFT)
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(id: UInt64): &NonFungibleToken.NFT
        pub fun borrowDayNFT(id: UInt64): &DayNFT.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow DayNFT reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // Collection of NFTs implementing standard interfaces
    pub resource Collection: CollectionPublic, NonFungibleToken.Provider, NonFungibleToken.Receiver, NonFungibleToken.CollectionPublic, MetadataViews.ResolverCollection {
        // Dictionary of NFT conforming tokens
        // NFT is a resource type with an `UInt64` ID field
        pub var ownedNFTs: @{UInt64: NonFungibleToken.NFT}

        init () {
            self.ownedNFTs <- {}
        }

        // withdraw removes an NFT from the collection and moves it to the caller
        pub fun withdraw(withdrawID: UInt64): @NonFungibleToken.NFT {
            let token <- self.ownedNFTs.remove(key: withdrawID) ?? panic("missing NFT")

            emit Withdraw(id: token.id, from: self.owner?.address)

            return <-token
        }

        // deposit takes a NFT and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(token: @NonFungibleToken.NFT) {
            let token <- token as! @DayNFT.NFT

            let id: UInt64 = token.id

            // Add the new token to the dictionary which removes the old one
            let oldToken <- self.ownedNFTs[id] <- token

            emit Deposit(id: id, to: self.owner?.address)

            destroy oldToken
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

        // Gets a reference to an NFT in the collection as a DayNFT,
        // exposing all of its fields.
        pub fun borrowDayNFT(id: UInt64): &DayNFT.NFT? {
            if self.ownedNFTs[id] != nil {
                let ref = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
                return ref as! &DayNFT.NFT
            } else {
                return nil
            }
        }

        pub fun borrowViewResolver(id: UInt64): &AnyResource{MetadataViews.Resolver} {
            let nft = &self.ownedNFTs[id] as auth &NonFungibleToken.NFT
            let dayNFT = nft as! &DayNFT.NFT
            return dayNFT as &AnyResource{MetadataViews.Resolver}
        }

        destroy() {
            destroy self.ownedNFTs
        }
    }

    // Resource that the contract owns to create new NFTs
    pub resource NFTMinter {

        // mintNFT mints a new NFT with a new ID
        // and deposit it in the recipients collection using their collection reference
        pub fun mintNFT(date: DateUtils.Date, title: String) : @NFT {

            // create a new NFT
            let id = DayNFT.totalSupply
            var newNFT <- create NFT(initID: id, date: date, title: title)

            DayNFT.totalSupply = DayNFT.totalSupply + 1

            return <-newNFT
        }
    }

    // Resource containing a user's bid in the auction for today's NFT
    pub resource Bid {
        pub(set) var vault: @FlowToken.Vault
        pub let recipient: Address
        pub let title: String
        pub let date: DateUtils.Date

        init(vault: @FlowToken.Vault, 
              recipient: Address,
              title: String,
              date: DateUtils.Date) {
            self.vault <- vault
            self.recipient = recipient
            self.title = title
            self.date = date
        }

        destroy() {
            destroy self.vault
        }
    }

    // PUBLIC APIs //

    // Create an empty NFT collection
    pub fun createEmptyCollection(): @NonFungibleToken.Collection {
        return <- create Collection()
    }

    // Make a bid on today's NFT
    pub fun makeBid(vault: @FlowToken.Vault, 
                    recipient: Address,
                    title: String,
                    date: DateUtils.Date) {
        
        let today = DateUtils.getDate()
        self.makeBidWithToday(vault: <-vault, 
                              recipient: recipient,
                              title: title,
                              date: date,
                              today: today)
    }
    // Make this public when testing
    access(contract) fun makeBidWithToday(vault: @FlowToken.Vault, 
                              recipient: Address,
                              title: String,
                              date: DateUtils.Date,
                              today: DateUtils.Date) {
                      
        if (!date.equals(today)) {
          panic("You can only bid on today's NFT")
        }
        if (vault.balance == 0.0) {
          panic("You can only bid a positive amount")
        }
        if (title.length > 70) {
          panic("The title can only be 70 characters long at most")
        }
        
        var bid <- create Bid(vault: <-vault, 
                      recipient: recipient,
                      title: title,
                      date: date)
        
        self.manager.handleBid(newBid: <-bid, today: today)
        
        emit BidReceived(user: recipient, date: date, title: title)
    }

    pub struct PublicBid {
        pub let amount: UFix64
        pub let user: Address
        pub let date: DateUtils.Date

        init(amount: UFix64, 
              user: Address,
              date: DateUtils.Date) {
            self.amount = amount
            self.user = user
            self.date = date
        }
    }

    // Get the best bid for today's auction
    pub fun getBestBid(): PublicBid {
        var today = DateUtils.getDate()
        return self.getBestBidWithToday(today: today)
    }
    // Make this public when testing
    access(contract) fun getBestBidWithToday(today: DateUtils.Date): PublicBid {
        return self.manager.getBestBidWithToday(today: today)
    }

    // Verify if a user has any NFTs to claim after winning one or more auctions
    pub fun nbNFTsToClaim(address: Address): Int {
        let today = DateUtils.getDate()
        return self.nbNFTsToClaimWithToday(address: address, today: today)
    }
    // Make this public when testing
    access(contract) fun nbNFTsToClaimWithToday(address: Address, today: DateUtils.Date): Int {
        return self.manager.nbNFTsToClaimWithToday(address: address, today: today)
    }

    // Claim NFTs due to the user, and deposit them into their collection
    access(contract) fun claimNFTs(address: Address): Int {
        var today = DateUtils.getDate()
        return self.claimNFTsWithToday(address: address, today: today)
    }
    // Make this public when testing
    access(contract) fun claimNFTsWithToday(address: Address, today: DateUtils.Date): Int {
        return self.manager.claimNFTsWithToday(address: address, today: today)
    }

    // Get amount of Flow due to the user
    pub fun tokensToClaim(address: Address): UFix64 {
        return self.manager.tokensToClaim(address: address)
    }

    // Claim Flow due to the user
    pub fun claimTokens(address: Address): UFix64 {
        return self.manager.claimTokens(address: address)
    }

    // Resource to receive Flow tokens to be distributed
    pub resource Admin: FungibleToken.Receiver {
        pub fun deposit(from: @FungibleToken.Vault) {
            DayNFT.manager.deposit(vault: <-from)
        }
    }

    init() {
        // Set named paths
        // Add version suffix to the paths when deploying to testnet
        self.CollectionStoragePath = /storage/DayNFTCollection
        self.CollectionPublicPath = /public/DayNFTCollection
        self.AdminPublicPath = /public/DayNFTAdmin
        let adminStoragePath = /storage/DayNFTAdmin

        let admin <- create Admin()
        self.account.save(<-admin, to: adminStoragePath)
        // Create a public capability allowing external users (like marketplaces)
        // to deposit flow to the contract so that it can be redistributed
        self.account.link<&DayNFT.Admin{FungibleToken.Receiver}>(
            self.AdminPublicPath,
            target: adminStoragePath
        )
        self.totalSupply = 0
        self.manager <- create ContractManager(contractAddress: self.account.address)

        emit ContractInitialized()
    }
}

/**

    OlympicPinMarket.cdc

    Description: Contract definitions for users to sell their pieces

    Marketplace is where users can create a sale collection that they
    store in their account storage. In the sale collection,
    they can put their NFTs up for sale with a price and publish a
    reference so that others can see the sale.

    If another user sees an NFT that they want to buy,
    they can send fungible tokens that equal the buy price
    to buy the NFT. The NFT is transferred to them when
    they make the purchase.

    Each user who wants to sell tokens will have a sale collection
    instance in their account that contains price information
    for each node in their collection. The sale holds a capability that
    links to their main piece collection.

    They can give a reference to this collection to a central contract
    so that it can list the sales in a central place

    When a user creates a sale, they will supply four arguments:
    - A OlympicPin.Collection capability that allows their sale to withdraw
      a piece when it is purchased.
    - A FungibleToken.Receiver capability as the place where the payment for the token goes.
    - A FungibleToken.Receiver capability specifying a beneficiary, where a cut of the purchase gets sent.
    - A cut percentage, specifying how much the beneficiary will receive.
**/

import FungibleToken from 0xf233dcee88fe0abe
import NonFungibleToken from 0x1d7e57aa55817448
import OlympicPin from 0x1d007eed492fdbbe
import NWayUtilityCoin from 0x011b6f1425389550

pub contract OlympicPinMarket {

    // -----------------------------------------------------------------------
    // OlympicPin Market contract Event definitions
    // -----------------------------------------------------------------------

    /// emitted when an OlympicPin piece is listed for sale
    pub event PieceListed(id: UInt64, price: UFix64, seller: Address?)
    /// emitted when the price of a listed piece has changed
    pub event PiecePriceChanged(id: UInt64, newPrice: UFix64, seller: Address?)
    /// emitted when a piece is purchased from the market
    pub event PiecePurchased(id: UInt64, price: UFix64, seller: Address?)
    /// emitted when a piece has been withdrawn from the sale
    pub event PieceCanceled(id: UInt64, owner: Address?)
    // emitted when the cut percentage of the sale has been changed by the owner
    pub event CutPercentageChanged(newPercent: UFix64, seller: Address?)

    /// Path where the `SaleCollection` is stored
    pub let marketStoragePath: StoragePath

    /// Path where the public capability for the `SaleCollection` is
    pub let marketPublicPath: PublicPath

    // SalePublic
    //
    // The interface that a user can publish a capability to their sale
    // to allow others to access their sale
    pub resource interface SalePublic {
        pub var cutPercentage: UFix64
        pub fun purchase(pieceId: UInt64, buyTokens: @NWayUtilityCoin.Vault): @OlympicPin.NFT {
            post {
                result.id == pieceId: "The ID of the withdrawn piece must be the same as the requested pieceId"
            }
        }
        pub fun getPrice(pieceId: UInt64): UFix64?
        pub fun getIDs(): [UInt64]
        pub fun borrowPiece(id: UInt64): &OlympicPin.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == id):
                    "Cannot borrow Piece reference: The ID of the returned reference is incorrect"
            }
        }
    }

    /// SaleCollection
    ///
    /// This is the main resource that piece sellers will store in their account
    /// to manage the NFTs that they are selling.
    /// The SaleCollection stores the piece's price that are for sale.
    /// The SaleCollection also keeps track of the price of each token.
    ///
    /// When a piece is purchased, a cut is taken from the tokens
    /// and sent to the beneficiary, then the rest are sent to the seller.

    pub resource SaleCollection: SalePublic {

        /// A collection of the pieces that the user has for sale
        access(self) var ownerCollection: Capability<&OlympicPin.Collection>

        /// Dictionary of the prices for each NFT by ID
        access(self) var prices: {UInt64: UFix64}

        /// The fungible token vault of the seller
        /// so that when someone buys a token, the tokens are deposited
        /// to this Vault
        access(self) var ownerCapability: Capability<&{FungibleToken.Receiver}>

        /// The capability that is used for depositing
        /// the beneficiary's cut of every sale
        access(self) var beneficiaryCapability: Capability<&{FungibleToken.Receiver}>

        /// The percentage that is taken from every purchase for the beneficiary
        /// For example, if the percentage is 15%, cutPercentage = 0.15

        pub var cutPercentage: UFix64

        init (ownerCollection: Capability<&OlympicPin.Collection>,
              ownerCapability: Capability<&{FungibleToken.Receiver}>,
              beneficiaryCapability: Capability<&{FungibleToken.Receiver}>,
              cutPercentage: UFix64) {
            pre {
                // Check that the owner's piece collection capability is correct
                ownerCollection.check():
                    "Owner's Piece Collection Capability is invalid!"

                // Check that both capabilities are for fungible token Vault receivers
                ownerCapability.check():
                    "Owner's Receiver Capability is invalid!"
                beneficiaryCapability.check():
                    "Beneficiary's Receiver Capability is invalid!"
            }

            self.ownerCollection = ownerCollection
            self.ownerCapability = ownerCapability
            self.beneficiaryCapability = beneficiaryCapability
            // prices are initially empty because there are no pieces for sale
            self.prices = {}
            self.cutPercentage = cutPercentage
        }

        /// listForSale lists an NFT for sale in this sale collection
        /// at the specified price
        ///
        /// Parameters: pieceId: The id of the NFT to be put up for sale
        ///             price: The price of the NFT
        pub fun listForSale(pieceId: UInt64, price: UFix64) {
            pre {
                self.ownerCollection.borrow()!.borrowPiece(id: pieceId) != nil:
                    "Piece does not exist in the owner's collection"
            }

            // Set the token's price
            self.prices[pieceId] = price

            emit PieceListed(id: pieceId, price: price, seller: self.owner?.address)
        }

        /// cancelSale cancels a piece sale and clears its price
        ///
        /// Parameters: pieceId: the ID of the piece to withdraw from the sale
        ///
        pub fun cancelSale(pieceId: UInt64) {

            // First check this version of the sale
            if self.prices[pieceId] != nil {
                // Remove the price from the prices dictionary
                self.prices.remove(key: pieceId)

                // Set prices to nil for the withdrawn ID
                self.prices[pieceId] = nil

                // Emit the event to cancel a piece from the Sale
                emit PieceCanceled(id: pieceId, owner: self.owner?.address)

            }
        }

        /// purchase lets a user send tokens to purchase an NFT that is for sale
        /// the purchased NFT is returned to the transaction context that called it
        ///
        /// Parameters: pieceId: the ID of the NFT to purchase
        ///             buyTokens: the fungible tokens that are used to buy the NFT
        ///
        /// Returns: @OlympicPin.NFT: the purchased NFT
        pub fun purchase(pieceId: UInt64, buyTokens: @NWayUtilityCoin.Vault): @OlympicPin.NFT {
            pre {
                self.prices[pieceId] != nil:
                    "No token matching this ID for sale!"

                buyTokens.balance == self.prices[pieceId]!:
                    "price does not match to buy the NFT!"
            }

            // Read the price for the token
            let price = self.prices[pieceId]!

            // Set the price for the token to nil
            self.prices[pieceId] = nil

            // Take the cut of the tokens that the beneficiary gets from the sent tokens
            let beneficiaryCut <- buyTokens.withdraw(amount: price*self.cutPercentage)

            // Deposit it into the beneficiary's Vault
            self.beneficiaryCapability.borrow()!
                .deposit(from: <-beneficiaryCut)

            // Deposit the remaining tokens into the owners vault
            self.ownerCapability.borrow()!
                .deposit(from: <-buyTokens)

            emit PiecePurchased(id: pieceId, price: price, seller: self.owner?.address)

            // Return the purchased token
            let boughtPiece <- self.ownerCollection.borrow()!.withdraw(withdrawID: pieceId) as! @OlympicPin.NFT

            return <-boughtPiece
        }

        /// changePercentage changes the cut percentage of the tokens that are for sale
        ///
        /// Parameters: newPercent: The new cut percentage for the sale
        pub fun changePercentage(_ newPercent: UFix64) {
            pre {
                newPercent <= 1.0: "Cannot set cut percentage to more than 100%"
            }

            self.cutPercentage = newPercent

            emit CutPercentageChanged(newPercent: newPercent, seller: self.owner?.address)
        }

        /// changeOwnerReceiver updates the capability for the sellers fungible token Vault
        ///
        /// Parameters: newOwnerCapability: The new fungible token capability for the account
        ///                                 who received tokens for purchases
        pub fun changeOwnerReceiver(_ newOwnerCapability: Capability<&{FungibleToken.Receiver}>) {
            pre {
                newOwnerCapability.borrow() != nil:
                    "Owner's Receiver Capability is invalid!"
            }
            self.ownerCapability = newOwnerCapability
        }

        /// changeBeneficiaryReceiver updates the capability for the beneficiary of the cut of the sale
        ///
        /// Parameters: newBeneficiaryCapability the new capability for the beneficiary of the cut of the sale
        ///
        pub fun changeBeneficiaryReceiver(_ newBeneficiaryCapability: Capability<&{FungibleToken.Receiver}>) {
            pre {
                newBeneficiaryCapability.borrow() != nil:
                    "Beneficiary's Receiver Capability is invalid!"
            }
            self.beneficiaryCapability = newBeneficiaryCapability
        }

        /// getPrice returns the price of a specific piece in the sale
        ///
        /// Parameters: pieceId: The ID of the NFT whose price to get
        ///
        /// Returns: UFix64: The price of the piece
        pub fun getPrice(pieceId: UInt64): UFix64? {
            if let price = self.prices[pieceId] {
                return price
            }
            return nil
        }

        /// getIDs returns an array of piece IDs that are for sale
        pub fun getIDs(): [UInt64] {
            return self.prices.keys
        }

        /// borrowPiece Returns a borrowed reference to a Piece for sale
        /// so that the caller can read data from it
        ///
        /// Parameters: id: The ID of the piece to borrow a reference to
        ///
        /// Returns: &OlympicPin.NFT? Optional reference to a piece for sale
        ///                        so that the caller can read its data
        ///
        pub fun borrowPiece(id: UInt64): &OlympicPin.NFT? {
            // first check this collection
            if self.prices[id] != nil {
                let ref = self.ownerCollection.borrow()!.borrowPiece(id: id)
                return ref
            }
            return nil
        }
    }

    /// createCollection returns a new collection resource to the caller
    pub fun createSaleCollection(ownerCollection: Capability<&OlympicPin.Collection>,
                                 ownerCapability: Capability<&{FungibleToken.Receiver}>,
                                 beneficiaryCapability: Capability<&{FungibleToken.Receiver}>,
                                 cutPercentage: UFix64): @SaleCollection {

        return <- create SaleCollection(ownerCollection: ownerCollection,
                                        ownerCapability: ownerCapability,
                                        beneficiaryCapability: beneficiaryCapability,
                                        cutPercentage: cutPercentage)
    }

    init() {
        self.marketStoragePath = /storage/OlympicPinSaleCollection
        self.marketPublicPath = /public/OlympicPinSaleCollection
    }
}
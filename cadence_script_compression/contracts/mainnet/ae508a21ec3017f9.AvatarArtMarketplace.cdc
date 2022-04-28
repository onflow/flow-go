import FungibleToken from 0xf233dcee88fe0abe;
import NonFungibleToken from 0x1d7e57aa55817448;
import AvatarArtNFT from 0xae508a21ec3017f9;
import AvatarArtTransactionInfo from 0xae508a21ec3017f9;

pub contract AvatarArtMarketplace {
    pub let AdminStoragePath: StoragePath
    pub let SaleCollectionStoragePath: StoragePath
    pub let SaleCollectionPublicPath: PublicPath

    access(self) var feeReference: Capability<&{AvatarArtTransactionInfo.PublicFeeInfo}>?;
    access(self) var feeRecepientReference: Capability<&{AvatarArtTransactionInfo.PublicTransactionAddress}>?;

    // emitted NFT is listed for sale
    pub event TokenListed(nftID: UInt64, price: UFix64, seller: Address?, paymentType: Type)

    // emitted when the price of a listed NFT has changed
    pub event TokenPriceChanged(nftID: UInt64, newPrice: UFix64, seller: Address?)

    // emitted when a token is purchased from the market
    pub event TokenPurchased(nftID: UInt64, price: UFix64, seller: Address?, buyer: Address)

    // emitted when NFT has been withdrawn from the sale
    pub event TokenWithdrawn(nftID: UInt64, owner: Address?)

    // emitted when a token purchased from market and a small fee are charged
    pub event CuttedFee(nftID: UInt64, seller: Address?, fee: CutFee, paymentType: Type);

    // ListingDetails
    // A struct containing a Listing's data.
    //
    pub struct ListingDetails {
        // The Type of the FungibleToken that payments must be made in.
        pub let salePaymentVaultType: Type
        // The amount that must be paid in the specified FungibleToken.
        pub var salePrice: UFix64

        pub let receiver: Capability<&{FungibleToken.Receiver}>

        init (
            salePaymentVaultType: Type,
            salePrice: UFix64,
            receiver: Capability<&{FungibleToken.Receiver}>
        ) {
            self.salePaymentVaultType = salePaymentVaultType
            self.salePrice = salePrice
            self.receiver = receiver
        }

        pub fun setSalePrice(_ newPrice: UFix64) {
            self.salePrice = newPrice;
        }
    }


    pub struct CutFee {
        pub(set) var affiliate: UFix64
        pub(set) var storing: UFix64
        pub(set) var insurance: UFix64
        pub(set) var contractor: UFix64
        pub(set) var platform: UFix64
        pub(set) var author: UFix64

        init() {
            self.affiliate = 0.0
            self.storing = 0.0
            self.insurance = 0.0
            self.contractor = 0.0
            self.platform = 0.0
            self.author = 0.0
        }
    }


    // SalePublic
    // Interface that users will publish for their SaleCollection
    // that only exposes the methods that are supposed to be public
    //
    // The public can purchase a NFT from this SaleCollection, get the
    // price of a NFT, or get all the ids of all the NFT up for sale
    //
    pub resource interface SalePublic {
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault,
                receiverCap: Capability<&{NonFungibleToken.CollectionPublic}>,
                affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?)
        pub fun getDetails(tokenID: UInt64): ListingDetails?
        pub fun getIDs(): [UInt64]
        pub fun borrowNFT(tokenID: UInt64): &NonFungibleToken.NFT? {
            // If the result isn't nil, the id of the returned reference
            // should be the same as the argument to the function
            post {
                (result == nil) || (result?.id == tokenID): 
                    "Cannot borrow Art reference: The ID of the returned reference is incorrect"
            }
        }
    }

    // SaleCollection
    //
    // A Collection that acts as a marketplace for NFTs. The owner
    // can list NFTs for sale and take sales down by unlisting it.
    //
    // Other users can also purchase NFTs that are for sale
    // in this SaleCollection, check the price of a sale, or check
    // all the NFTs that are for sale by their ids.
    //
    pub resource SaleCollection: SalePublic {
        // Dictionary of the low low prices for each NFT by ID
        access(self) var listing: {UInt64: ListingDetails}
        access(self) var nfts: @{UInt64: NonFungibleToken.NFT}

        // The fungible token vault of the owner of this sale.
        // When someone buys a token, this will be used to deposit
        // tokens into the owner's account.

        init () {
            self.listing = {}
            self.nfts <- {}
        }

        destroy() {
            destroy self.nfts
        }

        // listForSale
        // listForSale lists NFT(s) for sale
        //
        pub fun listForSale(nft: @AvatarArtNFT.NFT, price: UFix64, paymentType: Type,
                receiver: Capability<&{FungibleToken.Receiver}>) {
            pre {
                price > 0.0: "Cannot list a NFT for 0.0"
                AvatarArtTransactionInfo.isCurrencyAccepted(type: paymentType):
                    "Payment type is not allow"
            }

            // Validate the receiver
            receiver.borrow() ?? panic("can not borrow receiver")

            let tokenID = nft.id

            let old <- self.nfts[tokenID] <- nft 
            assert(old == nil, message: "Should never panic this")
            destroy old

            // Set sale price
            self.listing[tokenID] = ListingDetails(salePaymentVaultType: paymentType, salePrice: price, receiver: receiver)

            emit TokenListed(nftID: tokenID, price: price, seller: self.owner?.address, paymentType: paymentType)
        }

        // unlistSale
        // simply unlists the NFT from the SaleCollection
        // so it is no longer for sale
        //
        pub fun unlistSale(tokenID: UInt64): @NonFungibleToken.NFT {
            pre {
                self.listing.containsKey(tokenID): "No token matching this ID for sale!"
                self.nfts.containsKey(tokenID): "No token matching this ID for sale!"
            }
            self.listing.remove(key: tokenID)
            emit TokenWithdrawn(nftID: tokenID, owner: self.owner?.address)

            return <- self.nfts.remove(key: tokenID)!
        }

        access(self) fun cutFee(vault: &FungibleToken.Vault, salePrice: UFix64, artId: UInt64,
                paymentType: Type, affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?) {
            if AvatarArtMarketplace.feeReference == nil || AvatarArtMarketplace.feeRecepientReference == nil {
                return ;
            }

            let feeOp = AvatarArtMarketplace.feeReference!.borrow()!.getFee(tokenId: artId);
            let feeRecepientOp = AvatarArtMarketplace.feeRecepientReference!.borrow()!.getAddress(tokenId: artId, payType: paymentType);

            if feeOp == nil || feeRecepientOp == nil {
                return 
            }

            let fee = feeOp!;
            let feeRecepient = feeRecepientOp!;

            let cutFee = CutFee();

            // Affiliate
            if fee.affiliate != nil && fee.affiliate > 0.0 && affiliateVaultCap != nil
                && affiliateVaultCap!.check() {
                let fee = salePrice * fee.affiliate / 100.0;
                let feeVault <- vault.withdraw(amount: fee);
                affiliateVaultCap!.borrow()!.deposit(from: <- feeVault);
                cutFee.affiliate = fee;
            }

            // Storage fee
            if fee.storing != nil && fee.storing > 0.0 && feeRecepient.storing != nil
                && feeRecepient.storing!.check() {
                let fee = salePrice * fee.storing / 100.0;
                let feeVault <- vault.withdraw(amount: fee);
                feeRecepient.storing!.borrow()!.deposit(from: <- feeVault);
                cutFee.storing = fee;
            }

            // Insurrance Fee
            if fee.insurance != nil && fee.insurance > 0.0 && feeRecepient.insurance != nil
                && feeRecepient.insurance!.check() {
                let fee = salePrice * fee.insurance / 100.0;
                let feeVault <- vault.withdraw(amount: fee);
                feeRecepient.insurance!.borrow()!.deposit(from: <- feeVault);
                cutFee.insurance = fee;
            }

            // Dev
            if fee.contractor != nil && fee.contractor > 0.0 && feeRecepient.contractor != nil
                && feeRecepient.contractor!.check() {
                let fee = salePrice * fee.contractor / 100.0;
                let feeVault <- vault.withdraw(amount: fee);
                feeRecepient.contractor!.borrow()!.deposit(from: <- feeVault);
                cutFee.contractor = fee;
            }

            // The Platform
            if fee.platform != nil && fee.platform > 0.0 && feeRecepient.platform != nil
                && feeRecepient.platform!.check() {
                let fee = salePrice * fee.platform / 100.0;
                let feeVault <- vault.withdraw(amount: fee);
                feeRecepient.platform!.borrow()!.deposit(from: <- feeVault);
                cutFee.platform = fee;
            }

            // Author
            if let info = AvatarArtTransactionInfo.getNFTInfo(tokenID: artId) {
                let cap = info.author[vault.getType().identifier]

                if info.authorFee != nil && info.authorFee! > 0.0 && cap != nil && cap!.check()  {
                    let fee = salePrice * info.authorFee! / 100.0;
                    let feeVault <- vault.withdraw(amount: fee);
                    cap!.borrow()!.deposit(from: <- feeVault);
                    cutFee.author = fee;
                }
            }

            emit CuttedFee(nftID: artId, seller: self.owner?.address, fee: cutFee, paymentType: paymentType);
        }

        // purchase
        // purchase lets a user send tokens to purchase a NFT that is for sale
        //
        pub fun purchase(tokenID: UInt64, buyTokens: @FungibleToken.Vault,
                receiverCap: Capability<&{NonFungibleToken.CollectionPublic}>,
                affiliateVaultCap: Capability<&{FungibleToken.Receiver}>?) {
            pre {
                self.listing[tokenID] != nil:
                    "No token matching this ID for sale!"           
                self.nfts[tokenID] != nil:
                    "No token matching this ID for sale!"           
                buyTokens.balance == self.listing[tokenID]!.salePrice:
                    "Not enough tokens to buy the NFT!"
            }

            let price = buyTokens.balance;

            // get the value out of the optional
            let details = self.listing[tokenID]!
            self.listing.remove(key: tokenID)

            assert(
                buyTokens.isInstance(details.salePaymentVaultType),
                message: "payment vault is not allow"
            )

            let vaultRef = details.receiver.borrow()
                ?? panic("Could not borrow reference to owner token vault")


            self.cutFee(vault: &buyTokens as &FungibleToken.Vault, salePrice: price, artId: tokenID,
                    paymentType: details.salePaymentVaultType, affiliateVaultCap: affiliateVaultCap)
            
            // deposit the user's tokens into the owners vault
            vaultRef.deposit(from: <-buyTokens)


            // remove the NFT dictionary 
            let nft <- self.nfts.remove(key: tokenID)!
            let receiver = receiverCap.borrow() ?? panic("Can not borrow a reference to receiver collection")

            receiver.deposit(token: <- nft)

            // Set first owner nft is false
            AvatarArtTransactionInfo.setFirstOwner(tokenID: tokenID, false)

            emit TokenPurchased(nftID: tokenID, price: price, seller: self.owner?.address, buyer: receiverCap.address)
        }
        

        pub fun changePrice(id: UInt64, newPrice: UFix64) {
            pre {
                self.listing[id] != nil: "No token matching this ID for sale!"
            }

            let details = self.listing[id]!;
            details.setSalePrice(newPrice);

            self.listing[id] = details;

            emit TokenPriceChanged(nftID: id, newPrice: newPrice, seller: self.owner?.address);
        }

        // getDetails
        // getDetails returns the details of a specific NFT in the sale
        // if it exists, otherwise nil
        //
        pub fun getDetails(tokenID: UInt64): ListingDetails? {
            return self.listing[tokenID]
        }

        // getIDs
        // getIDs returns an array of all the NFT IDs that are up for sale
        //
        pub fun getIDs(): [UInt64] {
            return self.listing.keys
        }

        pub fun borrowNFT(tokenID: UInt64): &NonFungibleToken.NFT? {
            return &self.nfts[tokenID] as &NonFungibleToken.NFT
        } 

    }

    // createSaleCollection
    // createCollection returns a new SaleCollection resource to the caller
    //
    pub fun createSaleCollection(): @SaleCollection {
        return <- create SaleCollection()
    }

    pub resource Administrator {
        pub fun setFeePreference(
            feeReference: Capability<&AvatarArtTransactionInfo.FeeInfo{AvatarArtTransactionInfo.PublicFeeInfo}>,
            feeRecepientReference: Capability<&AvatarArtTransactionInfo.TransactionAddress{AvatarArtTransactionInfo.PublicTransactionAddress}>) {
            AvatarArtMarketplace.feeRecepientReference = feeRecepientReference;
            AvatarArtMarketplace.feeReference = feeReference;
        }

    }

    init() {
        self.feeReference = nil
        self.feeRecepientReference = nil

        self.AdminStoragePath = /storage/avatarArtMarketplaceAdmin
        self.SaleCollectionStoragePath = /storage/avatarArtMarketplaceSaleCollection
        self.SaleCollectionPublicPath = /public/avatarArtMarketplaceSaleCollection

        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }
}
import RaribleFee from 0x336405ad2f289b87
import FungibleToken from 0xf233dcee88fe0abe
import NFTStorefront from 0x4eb8a10cb9f87357
import NonFungibleToken from 0x1d7e57aa55817448

// RaribleOrder
//
// Wraps the NFTStorefront.createListting
//
pub contract RaribleOrder {

    pub let BUYER_FEE: String
    pub let SELLER_FEE: String
    pub let OTHER: String
    pub let ROYALTY: String
    pub let REWARD: String

    init() {
        // market buyer fee (on top of the price)
        self.BUYER_FEE = "BUYER_FEE"

        // market seller fee
        self.SELLER_FEE = "SELLER_FEE"

        // additional payments
        self.OTHER = "OTHER"

        // royalty
        self.ROYALTY = "ROYALTY"

        // seller reward
        self.REWARD = "REWARD"
    }

    // PaymentPart
    // 
    pub struct PaymentPart {
        // receiver address
        pub let address: Address

        // payment rate
        pub let rate: UFix64

        init(address: Address, rate: UFix64) {
            self.address = address
            self.rate = rate
        }
    }

    // Payment
    // Describes payment in the event OrderAvailable
    // 
    pub struct Payment {
        // type of payment
        pub let type: String

        // receiver address
        pub let address: Address

        // payment rate
        pub let rate: UFix64

        // payment amount
        pub let amount: UFix64

        init(type: String, address: Address, rate: UFix64, amount: UFix64) {
            self.type = type
            self.address = address
            self.rate = rate
            self.amount = amount
        }
    }

    // OrderAvailable
    // Order created and available for purchase
    // 
    pub event OrderAvailable(
        orderAddress: Address,
        orderId: UInt64,
        nftType: String,
        nftId: UInt64,
        vaultType: String,
        price: UFix64, // sum of payment parts
        offerPrice: UFix64, // base for calculate rates
        payments: [Payment]
    )

    pub event OrderClosed(
        orderAddress: Address,
        orderId: UInt64,
        nftType: String,
        nftId: UInt64,
        vaultType: String,
        price: UFix64,
        buyerAddress: Address,
        cuts: [PaymentPart]
    )

    pub event OrderCancelled(
        orderAddress: Address,
        orderId: UInt64,
        nftType: String,
        nftId: UInt64,
        vaultType: String,
        price: UFix64,
        cuts: [PaymentPart]
    )

    // addOrder
    // Wrapper for NFTStorefront.createListing
    //
    pub fun addOrder(
        storefront: &NFTStorefront.Storefront,
        nftProvider: Capability<&{NonFungibleToken.Provider,NonFungibleToken.CollectionPublic}>,
        nftType: Type,
        nftId: UInt64,
        vaultPath: PublicPath,
        vaultType: Type,
        price: UFix64,
        extraCuts: [PaymentPart],
        royalties: [PaymentPart]
    ): UInt64 {
        let orderAddress = storefront.owner!.address
        let payments: [Payment] = []
        let saleCuts: [NFTStorefront.SaleCut] = []
        var percentage = 1.0
        var offerPrice = 0.0

        let addPayment = fun (type: String, address: Address, rate: UFix64) {
            assert(rate >= 0.0 && rate < 1.0, message: "Rate must be in range [0..1)")
            let amount = price * rate
            let receiver = getAccount(address).getCapability<&{FungibleToken.Receiver}>(vaultPath)
            assert(receiver.borrow() != nil, message: "Missing or mis-typed fungible token receiver")

            payments.append(Payment(type:type, address:address, rate: rate, amount: amount))
            saleCuts.append(NFTStorefront.SaleCut(receiver: receiver, amount: amount))

            offerPrice = offerPrice + amount
            percentage = percentage - (type == RaribleOrder.BUYER_FEE ? 0.0 : rate)
            assert(rate >= 0.0 && rate < 1.0, message: "Sum of payouts must be in range [0..1)")
        }

        addPayment(RaribleOrder.BUYER_FEE, RaribleFee.feeAddress(), RaribleFee.buyerFee)
        addPayment(RaribleOrder.SELLER_FEE, RaribleFee.feeAddress(), RaribleFee.sellerFee)

        for cut in extraCuts {
            addPayment(RaribleOrder.OTHER, cut.address, cut.rate)
        }

        for royalty in royalties {
            addPayment(RaribleOrder.ROYALTY, royalty.address, royalty.rate)
        }

        addPayment(RaribleOrder.REWARD, orderAddress, percentage)

        let orderId = storefront.createListing(
            nftProviderCapability: nftProvider,
            nftType: nftType,
            nftID: nftId,
            salePaymentVaultType: vaultType,
            saleCuts: saleCuts
        )

        emit OrderAvailable(
            orderAddress: orderAddress,
            orderId: orderId,
            nftType: nftType.identifier,
            nftId: nftId,
            vaultType: vaultType.identifier,
            price: price,
            offerPrice: offerPrice,
            payments: payments
        )

        return orderId
    }

    // closeOrder
    // Purchase nft by o
    //
    pub fun closeOrder(
        storefront: &NFTStorefront.Storefront{NFTStorefront.StorefrontPublic},
        orderId: UInt64,
        orderAddress: Address,
        listing: &NFTStorefront.Listing{NFTStorefront.ListingPublic},
        paymentVault: @FungibleToken.Vault,
        buyerAddress: Address
    ): @NonFungibleToken.NFT {
        let details = listing.getDetails()
        let cuts: [PaymentPart] = []
        for saleCut in details.saleCuts {
            cuts.append(PaymentPart(address: saleCut.receiver.address, rate: saleCut.amount))
        }

        emit OrderClosed(
            orderAddress: orderAddress,
            orderId: orderId,
            nftType: details.nftType.identifier,
            nftId: details.nftID,
            vaultType: details.salePaymentVaultType.identifier,
            price: details.salePrice,
            buyerAddress: buyerAddress,
            cuts: cuts
        )

        let item <- listing.purchase(payment: <-paymentVault)
        storefront.cleanup(listingResourceID: orderId)
        return <- item
    }

    // removeOrder
    // Cancel sale, dismiss order
    //
    pub fun removeOrder(
        storefront: &NFTStorefront.Storefront,
        orderId: UInt64,
        orderAddress: Address,
        listing: &NFTStorefront.Listing{NFTStorefront.ListingPublic},
    ) {
        let details = listing.getDetails()
        let cuts: [PaymentPart] = []
        for saleCut in details.saleCuts {
            cuts.append(PaymentPart(address: saleCut.receiver.address, rate: saleCut.amount))
        }

        emit OrderCancelled(
            orderAddress: orderAddress,
            orderId: orderId,
            nftType: details.nftType.identifier,
            nftId: details.nftID,
            vaultType: details.salePaymentVaultType.identifier,
            price: details.salePrice,
            cuts: cuts
        )

        storefront.removeListing(listingResourceID: orderId)
    }
}

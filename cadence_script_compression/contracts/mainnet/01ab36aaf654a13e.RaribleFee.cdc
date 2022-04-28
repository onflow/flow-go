// Simple fee manager
//
pub contract RaribleFee {

    pub let commonFeeManagerStoragePath: StoragePath

    pub event SellerFeeChanged(value: UFix64)
    pub event BuyerFeeChanged(value: UFix64)

    // Seller fee [0..1)
    pub var sellerFee: UFix64

    // BuyerFee fee [0..1)
    pub var buyerFee: UFix64

    pub resource Manager {
        pub fun setSellerFee(_ fee: UFix64) {
            RaribleFee.sellerFee = fee
            emit SellerFeeChanged(value: RaribleFee.sellerFee)
        }

        pub fun setBuyerFee(_ fee: UFix64) {
            RaribleFee.buyerFee = fee
            emit BuyerFeeChanged(value: RaribleFee.buyerFee)
        }
    }

    init() {
        self.sellerFee = 0.025
        emit SellerFeeChanged(value: RaribleFee.sellerFee)
        self.buyerFee = 0.025
        emit BuyerFeeChanged(value: RaribleFee.buyerFee)

        self.commonFeeManagerStoragePath = /storage/commonFeeManager
        self.account.save(<- create Manager(), to: self.commonFeeManagerStoragePath)
    }

    pub fun feeAddress(): Address {
        return self.account.address
    }
}

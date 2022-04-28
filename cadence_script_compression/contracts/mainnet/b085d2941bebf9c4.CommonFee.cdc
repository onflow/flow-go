// Simple fee manager
//
pub contract CommonFee {

    pub let commonFeeManagerStoragePath: StoragePath

    pub event SellerFeeChanged(value: UFix64)
    pub event BuyerFeeChanged(value: UFix64)

    // Seller fee in %
    pub var sellerFee: UFix64

    // BuyerFee fee in %
    pub var buyerFee: UFix64

    pub resource Manager {
        pub fun setBuyerFee(_ fee: UFix64) {
            CommonFee.buyerFee = fee
            emit BuyerFeeChanged(value: CommonFee.buyerFee)
        }

        pub fun setSellerFee(_ fee: UFix64) {
            CommonFee.sellerFee = fee
            emit SellerFeeChanged(value: CommonFee.sellerFee)
        }
    }

    init() {
        self.sellerFee = 2.5
        emit SellerFeeChanged(value: CommonFee.sellerFee)
        self.buyerFee = 2.5
        emit BuyerFeeChanged(value: CommonFee.buyerFee)

        self.commonFeeManagerStoragePath = /storage/commonFeeManager
        self.account.save(<- create Manager(), to: self.commonFeeManagerStoragePath)
    }

    pub fun feeAddress(): Address {
        return self.account.address
    }
}

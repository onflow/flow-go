// Gaia Fees
//
// Simple fee manager
//
pub contract GaiaFee {

    pub let commonFeeManagerStoragePath: StoragePath

    pub event SellerFeeChanged(value: UFix64)
    pub event BuyerFeeChanged(value: UFix64)
    pub event FeeAddressUpdated(label: String, address: Address)

    access(self) var feeAddresses: {String:Address}

    // Seller fee [0..1)
    pub var sellerFee: UFix64

    // BuyerFee fee [0..1)
    pub var buyerFee: UFix64

    pub resource Manager {
        pub fun setSellerFee(_ fee: UFix64) {
            GaiaFee.sellerFee = fee
            emit SellerFeeChanged(value: GaiaFee.sellerFee)
        }

        pub fun setBuyerFee(_ fee: UFix64) {
            GaiaFee.buyerFee = fee
            emit BuyerFeeChanged(value: GaiaFee.buyerFee)
        }

        pub fun setFeeAddress(_ label: String, address: Address) {
            GaiaFee.feeAddresses[label] = address
            emit FeeAddressUpdated(label: label, address: address)
        }
    }

    init() {
        self.sellerFee = 0.05 
        emit SellerFeeChanged(value: GaiaFee.sellerFee)
        self.buyerFee = 0.0 // Gaia Buyer Fee
        emit BuyerFeeChanged(value: GaiaFee.buyerFee)

        self.feeAddresses = {}

        self.commonFeeManagerStoragePath = /storage/commonFeeManager
        self.account.save(<- create Manager(), to: self.commonFeeManagerStoragePath)
    }

    pub fun feeAddress(): Address {
        return self.feeAddresses["gaia"] ?? self.account.address
    }

    pub fun feeAddressByName(_ label: String): Address {
        return self.feeAddresses[label] ?? self.account.address
    }

    pub fun addressMap(): {String:Address} {
        return GaiaFee.feeAddresses
    }
}

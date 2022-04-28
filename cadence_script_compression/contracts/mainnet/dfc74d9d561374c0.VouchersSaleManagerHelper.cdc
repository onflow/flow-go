pub contract VouchersSaleManagerHelper {
    pub var goatVoucherMaxQuantityPerMint: UInt64
    pub var goatVoucherSaleStartTime: UFix64
    pub var goatVoucherSalePrice: UFix64
    pub var curSequentialGoatEditionNumber: UInt64
    access(account) let mintedGoatVoucherEditions: {UInt64: Bool}

    pub var packVoucherMaxQuantityPerMint: UInt64
    pub var packVoucherSaleStartTime: UFix64
    pub var packVoucherSalePrice: UFix64
    pub var curSequentialPackEditionNumber: UInt64
    access(account) let mintedPackVoucherEditions: {UInt64: Bool}

    access(account) fun updateGoatVoucherSale(quantityPerMint: UInt64, price: UFix64, startTime: UFix64) {
        self.goatVoucherMaxQuantityPerMint = quantityPerMint
        self.goatVoucherSaleStartTime = startTime
        self.goatVoucherSalePrice = price
    }

    access(account) fun setSequentialGoatEditionNumber(_ edition: UInt64) {
        self.curSequentialGoatEditionNumber = edition
    }

    access(account) fun updatePackVoucherSale(quantityPerMint: UInt64, price: UFix64, startTime: UFix64) {
        self.packVoucherMaxQuantityPerMint = quantityPerMint
        self.packVoucherSaleStartTime = startTime
        self.packVoucherSalePrice = price
    }

    access(account) fun setSequentialPackEditionNumber(_ edition: UInt64) {
        self.curSequentialPackEditionNumber = edition
    }

    init() {
        self.goatVoucherMaxQuantityPerMint = 0
        self.goatVoucherSaleStartTime = 14268375033.0
        self.goatVoucherSalePrice = 100000000.0
        self.mintedGoatVoucherEditions = {}

        self.packVoucherMaxQuantityPerMint = 0
        self.packVoucherSaleStartTime = 14268375033.0
        self.packVoucherSalePrice = 100000000.0
        self.mintedPackVoucherEditions = {}

        // Why 1250? Minting was turned off prior to the
        // deployment of this updated helper. Starting at 1250
        // matches mainnet's current status in minting at the
        // time of this deployment.
        self.curSequentialGoatEditionNumber = 1250
        self.curSequentialPackEditionNumber = 1250
    }
}
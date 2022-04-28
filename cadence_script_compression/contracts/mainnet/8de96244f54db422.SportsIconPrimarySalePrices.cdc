/*
    Description: Contract for SportsIcon management of primary sale listings
*/

import SportsIconNFTStorefront from 0x03c8261a06cb1b42

pub contract SportsIconPrimarySalePrices {

    pub struct PrimarySaleListing {
        pub let totalPrice: UFix64
        access(self) let saleCuts: [SportsIconNFTStorefront.SaleCut]

        init(totalPrice: UFix64, saleCuts: [SportsIconNFTStorefront.SaleCut]) {
            self.totalPrice = totalPrice

            assert(saleCuts.length > 0, message: "Listing must have at least one payment cut recipient")
            self.saleCuts = saleCuts

            var salePrice = 0.0
            for cut in self.saleCuts {
                cut.receiver.borrow()
                    ?? panic("Cannot borrow receiver")
                salePrice = salePrice + cut.amount
            }
            assert(salePrice > 0.0, message: "Listing must have non-zero price")
            assert(salePrice == totalPrice, message: "Cuts do not line up to stored total price of listing.")
        }

        pub fun getSaleCuts(): [SportsIconNFTStorefront.SaleCut] {
            return self.saleCuts
        }
    }

    // Mapping of SportsIcon SetID to FungibleTokenType to Price
    access(self) let primarySalePrices: { UInt64: { String: PrimarySaleListing } }

    access(account) fun updateSalePrice(setID: UInt64, currency: String, primarySaleListing: PrimarySaleListing?) {
        if (self.primarySalePrices[setID] == nil) {
            self.primarySalePrices[setID] = {}
        }
        if (primarySaleListing == nil) {
            self.primarySalePrices[setID]!.remove(key: currency)
        } else {
            self.primarySalePrices[setID]!.insert(key: currency, primarySaleListing!)
        }
    }

    pub fun getListing(setID: UInt64, currency: String): PrimarySaleListing? {
        if (self.primarySalePrices[setID] == nil) {
            return nil
        }
        if (self.primarySalePrices[setID]![currency] == nil) {
            return nil
        }
        return self.primarySalePrices[setID]![currency]!
    }

    init() {
        self.primarySalePrices = {}
    }
}
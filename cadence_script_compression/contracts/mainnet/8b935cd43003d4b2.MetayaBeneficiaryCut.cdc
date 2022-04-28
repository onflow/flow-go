/**
    Description: Central Smart Contract for Metaya Beneficiary Cut
    This smart contract stores the mappings from the names of copyright owners
    to the vaults in which they'd like to receive tokens,
    as well as the cut they'd like to take from store and pack sales revenue
    and marketplace transactions.

    Copyright 2021 Metaya.io
    SPDX-License-Identifier: Apache-2.0
**/

import FungibleToken from 0xf233dcee88fe0abe

pub contract MetayaBeneficiaryCut {

    /// Emitted when the contract is created
    pub event ContractInitialized()

    /// Emitted when a FT-receiving capability for a copyright owner has been updated
    /// If address is nil, that means the capability has been removed.
    pub event CopyrightOwnerCapabilityUpdated(name: String, address: Address?)

    /// Emitted when a copyright owner's store sale cutpercentage has been updated
    /// If the copyrightOwnerAndCutPercentage is nil, that means it has been removed.
    pub event StoreCutPercentagesUpdated(saleID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?)

    /// Emitted when a copyright owner's pack sale cutpercentage has been updated
    /// If the copyrightOwnerAndCutPercentage is nil, that means it has been removed.
    pub event PackCutPercentagesUpdated(packID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?)

    /// Emitted when a copyright owner's market cutpercentage has been updated
    /// If the copyrightOwnerAndCutPercentage is nil, that means it has been removed.
    pub event MarketCutPercentagesUpdated(playID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?)

    /// Emitted when the capability of Metaya service has been updated
    pub event MetayaCapabilityUpdated(address: Address)

    /// Emitted when the market cutPercentage of Metaya service has been updated
    pub event MetayaMarketCutPercentageUpdated(cutPercentage: UFix64)

    /// Emitted when the commonweal organization has been updated
    pub event CommonwealUpdated(name: String, address: Address?, cutPercentage: UFix64)

    /// MetayaBeneficiaryCut named path
    pub let AdminStoragePath: StoragePath

    /// Copyright owners Capabilities, copyright owner's name => capability
    access(self) var copyrightOwnerCapabilities: {String: Capability<&{FungibleToken.Receiver}>}

    /// Beneficiary Cut from store sales
    ///
    /// storeCutPercentages is a mapping from saleID, to copyright owner's name, to
    /// the cut percentage that they are supposed to receive.
    access(self) var storeCutPercentages: {UInt32: {String: UFix64}}

    /// Beneficiary Cut from pack sales
    ///
    /// packCutPercentages is a mapping from packID, to copyright owner's name, to
    /// the cut percentage that they are supposed to receive.
    access(self) var packCutPercentages: {UInt32: {String: UFix64}}

    /// Beneficiary Cut from marketplace transactions
    ///
    /// marketCutPercentages is a mapping from playID, to copyright owner's name, to
    /// the cut percentage that they are supposed to receive.
    access(self) var marketCutPercentages: {UInt32: {String: UFix64}}

    /// The capability of Metaya service
    pub var metayaCapability: Capability<&{FungibleToken.Receiver}>
    /// The market cutPercentage of Metaya service
    pub var metayaMarketCutPercentage: UFix64

    /// Commonweal organization capabilities
    access(self) var commonwealCapabilities: {String: Capability<&{FungibleToken.Receiver}>}
    /// Commonweal organization cutPercentages
    access(self) var commonwealCutPercentages: {String: UFix64}

    /// Get all copyright owner names
    pub fun getAllCopyrightOwnerNames(): [String] {
        return self.copyrightOwnerCapabilities.keys
    }

    /// Get all commonweal names
    pub fun getAllCommonwealNames(): [String] {
        return self.commonwealCapabilities.keys
    }

    /// Get the saleIDs in storeCutPercentages
    pub fun getSaleIDsInStoreCutPercentages(): [UInt32] {
        return self.storeCutPercentages.keys
    }

    /// Get the packIDs in packCutPercentages
    pub fun getPackIDsInPackCutPercentages(): [UInt32] {
        return self.packCutPercentages.keys
    }

    /// Get the playIDs in marketCutPercentages
    pub fun getPlayIDsInMarketCutPercentages(): [UInt32] {
        return self.marketCutPercentages.keys
    }

    /// Get the capability for depositing accounting tokens to the copyright owner
    pub fun getCopyrightOwnerCapability(name: String): Capability<&{FungibleToken.Receiver}>? {

        if let cap = self.copyrightOwnerCapabilities[name] {
            return cap
        } else {
            return nil
        }
    }

    /// Get the copyright owners' sale cutPercentage of the store with saleID
    pub fun getStoreCutPercentage(saleID: UInt32, name: String): UFix64? {

        if let cutPercentages = self.storeCutPercentages[saleID] {
            return cutPercentages[name]
        } else {
            return nil
        }
    }

    /// Get the copyright owners' pack cutPercentage of the pack with packID
    pub fun getPackCutPercentage(packID: UInt32, name: String): UFix64? {

        if let cutPercentages = self.packCutPercentages[packID] {
            return cutPercentages[name]
        } else {
            return nil
        }
    }

    /// Get the copyright owners' market cutPercentage of the NFT with playID
    pub fun getMarketCutPercentage(playID: UInt32, name: String): UFix64? {

        if let cutPercentages = self.marketCutPercentages[playID] {
            return cutPercentages[name]
        } else {
            return nil
        }
    }

    /// Get the copyright owners' names with saleID
    pub fun getStoreCopyrightOwnerNames(saleID: UInt32): [String]? {

        if let cac = self.storeCutPercentages[saleID] {
            return cac.keys
        } else {
            return nil
        }
    }

    /// Get the copyright owners' names with packID
    pub fun getPackCopyrightOwnerNames(packID: UInt32): [String]? {

        if let cac = self.packCutPercentages[packID] {
            return cac.keys
        } else {
            return nil
        }
    }

    /// Get the copyright owners' names with playID
    pub fun getMarketCopyrightOwnerNames(playID: UInt32): [String]? {

        if let cac = self.marketCutPercentages[playID] {
            return cac.keys
        } else {
            return nil
        }
    }

    /// Get the capability for depositing accounting tokens to the commonweal organization
    pub fun getCommonwealCapability(name: String): Capability<&{FungibleToken.Receiver}>? {

        if let cap = self.commonwealCapabilities[name] {
            return cap
        } else {
            return nil
        }
    }

    /// Get the cutPercentage of commonweal organization
    pub fun getCommonwealCutPercentage(name: String): UFix64? {

        if let cutPercentage = self.commonwealCutPercentages[name] {
            return cutPercentage
        } else {
            return nil
        }
    }

    pub resource Admin {

        /// Set or update the FT-receiving capability for a copyright owner
        pub fun setCopyrightOwnerCapability(name: String, capability: Capability<&{FungibleToken.Receiver}>?) {

            if let cap = capability {
                MetayaBeneficiaryCut.copyrightOwnerCapabilities[name] = cap

                // Get the address behind a capability
                let addr = ((cap.borrow() ?? panic("Capability is empty."))
                    .owner ?? panic("Capability owner is empty."))
                    .address

                emit CopyrightOwnerCapabilityUpdated(name: name, address: addr)
            } else {
                MetayaBeneficiaryCut.copyrightOwnerCapabilities.remove(key: name)

                emit CopyrightOwnerCapabilityUpdated(name: name, address: nil)
            }
        }

        /// Set or update the store cutpercentage for the copyright owner
        pub fun setStoreCutPercentages(saleID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?) {

            if let cac = copyrightOwnerAndCutPercentage {

                for name in cac.keys {
                    assert(MetayaBeneficiaryCut.getAllCopyrightOwnerNames().contains(name), message: "Not found Copyright Owner's name in registered.")
                }

                var total: UFix64 = 0.0
                for cutPercentage in cac.values {
                    total = total + cutPercentage
                }
                assert(total == 1.0, message: "The sum of cutPercentages must be 1.0.")

                MetayaBeneficiaryCut.storeCutPercentages[saleID] = cac

                emit StoreCutPercentagesUpdated(saleID: saleID, copyrightOwnerAndCutPercentage: copyrightOwnerAndCutPercentage)
            } else {
                MetayaBeneficiaryCut.storeCutPercentages.remove(key: saleID)

                emit StoreCutPercentagesUpdated(saleID: saleID, copyrightOwnerAndCutPercentage: nil)
            }
        }

        /// Set or update the pack cutpercentage for the copyright owner
        pub fun setPackCutPercentages(packID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?) {

            if let cac = copyrightOwnerAndCutPercentage {

                for name in cac.keys {
                    assert(MetayaBeneficiaryCut.getAllCopyrightOwnerNames().contains(name), message: "Not found Copyright Owner's name in registered.")
                }

                var total: UFix64 = 0.0
                for cutPercentage in cac.values {
                    total = total + cutPercentage
                }
                assert(total == 1.0, message: "The sum of cutPercentages must be 1.0.")

                MetayaBeneficiaryCut.packCutPercentages[packID] = cac

                emit PackCutPercentagesUpdated(packID: packID, copyrightOwnerAndCutPercentage: copyrightOwnerAndCutPercentage)
            } else {
                MetayaBeneficiaryCut.packCutPercentages.remove(key: packID)

                emit PackCutPercentagesUpdated(packID: packID, copyrightOwnerAndCutPercentage: nil)
            }
        }

        /// Set or update the market cutpercentage for the copyright owner
        pub fun setMarketCutPercentages(playID: UInt32, copyrightOwnerAndCutPercentage: {String: UFix64}?) {

            if let cac = copyrightOwnerAndCutPercentage {

                for name in cac.keys {
                    assert(MetayaBeneficiaryCut.getAllCopyrightOwnerNames().contains(name), message: "Not found Copyright Owner's name in registered.")
                }

                var total: UFix64 = 0.0
                for cutPercentage in cac.values {
                    total = total + cutPercentage
                }

                total = total + MetayaBeneficiaryCut.metayaMarketCutPercentage

                assert(total < 1.0, message: "The sum of cutPercentage must be less than 1.0.")

                MetayaBeneficiaryCut.marketCutPercentages[playID] = cac

                emit MarketCutPercentagesUpdated(playID: playID, copyrightOwnerAndCutPercentage: copyrightOwnerAndCutPercentage)
            } else {
                MetayaBeneficiaryCut.marketCutPercentages.remove(key: playID)

                emit MarketCutPercentagesUpdated(playID: playID, copyrightOwnerAndCutPercentage: nil)
            }
        }

        /// Update the capability of Metaya service
        pub fun setMetayaCapability(capability: Capability<&{FungibleToken.Receiver}>) {
            MetayaBeneficiaryCut.metayaCapability = capability

            // Get the address behind a capability
            let addr = ((capability.borrow() ?? panic("Capability is empty."))
                .owner ?? panic("Capability owner is empty."))
                .address

            emit MetayaCapabilityUpdated(address: addr)
        }

        /// Update the market cutPercentage of Metaya service
        pub fun setMetayaMarketCutPercentage(cutPercentage: UFix64) {

            pre{
                cutPercentage < 1.0: "The cutPercentage must be less than 1.0."
            }

            MetayaBeneficiaryCut.metayaMarketCutPercentage = cutPercentage
            emit MetayaMarketCutPercentageUpdated(cutPercentage: cutPercentage)
        }

        /// Set or update the capability and cutPercentage of commonweal organization
        pub fun setCommonweal(name: String, capability: Capability<&{FungibleToken.Receiver}>?, cutPercentage: UFix64) {

            pre{
                cutPercentage <= 1.0: "The cutPercentage can not be greater than 1.0."
            }

            if let cap = capability {

                MetayaBeneficiaryCut.commonwealCapabilities[name] = cap
                MetayaBeneficiaryCut.commonwealCutPercentages[name] = cutPercentage

                // Get the address behind a capability
                let addr = ((cap.borrow() ?? panic("Capability is empty."))
                    .owner ?? panic("Capability owner is empty."))
                    .address

                emit CommonwealUpdated(name: name, address: addr, cutPercentage: cutPercentage)
            } else {
                MetayaBeneficiaryCut.commonwealCapabilities.remove(key: name)
                MetayaBeneficiaryCut.commonwealCutPercentages.remove(key: name)

                emit CommonwealUpdated(name: name, address: nil, cutPercentage: 0.0)
            }
        }
    }

    init() {
        // Set named paths
        self.AdminStoragePath = /storage/MetayaBeneficiaryCutAdmin

        // Initialize contract fields
        self.copyrightOwnerCapabilities = {}
        self.storeCutPercentages = {}
        self.packCutPercentages = {}
        self.marketCutPercentages = {}
        self.metayaCapability = self.account.getCapability<&{FungibleToken.Receiver}>(/public/MetayaUtilityCoinReceiver)
        self.metayaMarketCutPercentage = 0.03
        self.commonwealCapabilities = {}
        self.commonwealCutPercentages = {}

        self.account.save<@Admin>(<- create Admin(), to: self.AdminStoragePath)

        emit ContractInitialized()
    }
}
// The Influencer Registry stores the mappings from the name of an
// influencer to the vaults in which they'd like to receive tokens,
// as well as the cut they'd like to take from marketplace transactions.

import FungibleToken from 0xf233dcee88fe0abe

pub contract InfluencerRegistry {

    // Emitted when the contract is created
    pub event ContractInitialized()

    // Emitted when a FT-receiving capability for an influencer has been updated
    // If address is nil, that means the capability has been removed.
    pub event CapabilityUpdated(name: String, ftType: Type, address: Address?)

    // Emitted when an influencer's cut percentage has been updated
    // If the cutPercentage is nil, that means it has been removed.
    pub event CutPercentageUpdated(name: String, cutPercentage: UFix64?)

    // Emitted when the default cut percentage has been updated
    pub event DefaultCutPercentageUpdated(cutPercentage: UFix64?)

    // capabilities is a mapping from influencer name, to fungible token ID, to
    // the capability for a receiver for the fungible token
    pub var capabilities: {String: {String: Capability<&{FungibleToken.Receiver}>}}

    // The mappings from the name of an influencer to the cut percentage
    // that they are supposed to receive.
    pub var cutPercentages: {String: UFix64}

    // The default cut percentage
    pub var defaultCutPercentage: UFix64

    // Get the capability for depositing accounting tokens to the influencer
    pub fun getCapability(name: String, ftType: Type): Capability? {
        let ftId = ftType.identifier

        if let caps = self.capabilities[name] {
            return caps[ftId]
        } else {
            return nil
        }
    }

    // Get the current cut percentage for the influencer
    pub fun getCutPercentage(name: String): UFix64 {
        if let cut = InfluencerRegistry.cutPercentages[name] {
            return cut
        } else {
            return InfluencerRegistry.defaultCutPercentage
        }
    }

    // Admin is an authorization resource that allows the contract owner to 
    // update values in the registry.
    pub resource Admin {

        // Update the FT-receiving capability for an influencer
        pub fun setCapability(name: String, ftType: Type, capability: Capability<&{FungibleToken.Receiver}>?) {
            let ftId = ftType.identifier
            if let cap = capability {
                if let caps = InfluencerRegistry.capabilities[name] {
                    caps[ftId] = cap
                    InfluencerRegistry.capabilities[name] = caps
                } else {
                    InfluencerRegistry.capabilities[name] = {ftId: cap}
                }
                // This is the only way to get the address behind a capability from Cadence right
                // now.  It will panic if the capability is not pointing to anything, but in that
                // case we should in fact panic anyways.
                let addr = ((cap.borrow() ?? panic("Capability is empty"))
                    .owner ?? panic("Capability owner is empty"))
                    .address

                emit CapabilityUpdated(name: name, ftType: ftType, address: addr)
            } else {
                if let caps = InfluencerRegistry.capabilities[name] {
                    caps.remove(key: ftId)
                    InfluencerRegistry.capabilities[name] = caps
                }

                emit CapabilityUpdated(name: name, ftType: ftType, address: nil)
            }
        }

        // Update the cut percentage for the influencer
        pub fun setCutPercentage(name: String, cutPercentage: UFix64?) {
            InfluencerRegistry.cutPercentages[name] = cutPercentage

            emit CutPercentageUpdated(name: name, cutPercentage: cutPercentage)
        }

        // Update the default cut percentage
        pub fun setDefaultCutPercentage(cutPercentage: UFix64) {
            InfluencerRegistry.defaultCutPercentage = cutPercentage
            emit DefaultCutPercentageUpdated(cutPercentage: cutPercentage)
        }

    }

    init() {
        self.cutPercentages = {}
        self.capabilities = {}

        self.defaultCutPercentage = 0.04

        self.account.save<@Admin>(<- create Admin(), to: /storage/EternalInfluencerRegistryAdmin)
    }

}

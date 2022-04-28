
pub contract StorefrontService {

    // basic data about the storefront
    pub let version: UInt32
    pub let name: String
    pub let description: String
    pub var closed: Bool

    // paths
    access(all) let ADMIN_OBJECT_PATH: StoragePath

    // storefront events
    pub event StorefrontClosed()
    pub event ContractInitialized()

    init(storefrontName: String, storefrontDescription: String) {
        self.version = 1
        self.name = storefrontName
        self.description = storefrontDescription
        self.closed = false

        self.ADMIN_OBJECT_PATH = /storage/StorefrontAdmin

        // put the admin in storage
        self.account.save<@StorefrontAdmin>(<- create StorefrontAdmin(), to: StorefrontService.ADMIN_OBJECT_PATH)

        emit ContractInitialized()
    }

    // Returns the version of this contract
    //
    pub fun getVersion(): UInt32 {
        return self.version
    }

    // StorefrontAdmin is used for administering the Storefront
    //
    pub resource StorefrontAdmin {

        // Closes the Storefront, rendering any write access impossible
        //
        pub fun close() {
            if !StorefrontService.closed {
                StorefrontService.closed = true
                emit StorefrontClosed()
            }
        }

        // Creates a new StorefrontAdmin that allows for another account
        // to administer the Storefront
        //
        pub fun createNewStorefrontAdmin(): @StorefrontAdmin {
            return <- create StorefrontAdmin()
        }
    }
}

pub contract Whitelist {
    access(self) let whitelist: { Address: Bool }
    access(self) let bought: { Address: Bool }

    pub let AdminStoragePath: StoragePath

    pub event WhitelistUpdate(addresses: [Address], whitelisted: Bool)

    pub resource Admin {

        pub fun addWhitelist(addresses: [Address]) {
            for address in addresses {
                Whitelist.whitelist[address] = true
            }

            emit WhitelistUpdate(addresses: addresses, whitelisted: true)
        }

        pub fun unWhitelist(addresses: [Address]) {
            for address in addresses {
                Whitelist.whitelist[address] = false
            }

            emit WhitelistUpdate(addresses: addresses, whitelisted: false)
        }

    }

    access(account) fun markAsBought(address: Address) {
        self.bought[address] = true
    }

    pub fun whitelisted(address: Address): Bool {
        return self.whitelist[address] ?? false
    }

    pub fun hasBought(address: Address): Bool {
        return self.bought[address] ?? false
    }

    init() {
        self.whitelist = {}
        self.bought = {}

        self.AdminStoragePath = /storage/BNMUWhitelistAdmin

        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}
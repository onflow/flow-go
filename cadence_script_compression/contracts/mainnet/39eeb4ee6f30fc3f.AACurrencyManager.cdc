import FungibleToken from 0xf233dcee88fe0abe

pub contract AACurrencyManager {
    access(self) var acceptCurrencies: [Type]
    access(self) let paths: { String: CurPath }

    pub let AdminStoragePath: StoragePath

    pub struct CurPath {
        pub let publicPath: PublicPath
        pub let storagePath: StoragePath

        init(publicPath: PublicPath, storagePath: StoragePath) {
            self.publicPath = publicPath
            self.storagePath = storagePath
        }
    }

    pub resource Administrator {
        pub fun setAcceptCurrencies(types: [Type]) {
            for type in types {
                assert(
                type.isSubtype(of: Type<@FungibleToken.Vault>()),
                message: "Should be a sub type of FungibleToken.Vault"
                )
            }

            AACurrencyManager.acceptCurrencies = types
        }

        pub fun setPath(type: Type, path: CurPath) {
            AACurrencyManager.paths[type.identifier] = path
        }
    }

    pub fun getAcceptCurrentcies(): [Type] {
        return  self.acceptCurrencies
    }

    pub fun isCurrencyAccepted(type: Type): Bool {
        return  self.acceptCurrencies.contains(type)
    }

    pub fun getPath(type: Type): CurPath? {
        return self.paths[type.identifier]
    }

    init() {
        self.acceptCurrencies = []
        self.paths = {}

        self.AdminStoragePath = /storage/AACurrencyManagerAdmin
        self.account.save(<- create Administrator(), to: self.AdminStoragePath)
    }
}
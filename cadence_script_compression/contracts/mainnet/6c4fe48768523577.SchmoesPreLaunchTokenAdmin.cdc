import SchmoesPreLaunchToken from 0x6c4fe48768523577

pub contract SchmoesPreLaunchTokenAdmin {
    pub let AdminStoragePath: StoragePath

    pub resource Admin {
        pub fun setIsSaleActive(_ isSaleActive: Bool) {
            SchmoesPreLaunchToken.setIsSaleActive(isSaleActive)
        }

        pub fun setImageUrl(_ imageUrl: String) {
            SchmoesPreLaunchToken.setImageUrl(imageUrl)
        }
    }

    pub init() {
        self.AdminStoragePath = /storage/schmoesPreLaunchTokenAdmin
        self.account.save(<- create Admin(), to: self.AdminStoragePath)
    }
}

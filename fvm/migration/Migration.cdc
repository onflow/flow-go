

access(all)
contract Migration {

    access(all)
    resource Admin {

        access(all)
        fun migrate() {
            Migration.migrate()
        }
    }

    access(all)
    let adminStoragePath: StoragePath

    init() {
        self.adminStoragePath = /storage/migrationAdmin

        self.account.storage.save(
            <-create Admin(),
            to: self.adminStoragePath
        )
    }

    access(contract)
    fun migrate() {
        // NO-OP
    }
}

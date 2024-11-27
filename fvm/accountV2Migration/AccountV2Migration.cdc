access(all)
contract AccountV2Migration {

    access(all)
    event Migrated(
        addressStartIndex: UInt64,
        count: UInt64
    )

    access(all)
    resource Admin {
        access(all)
        fun setEnabled(_ isEnabled: Bool) {
            AccountV2Migration.isEnabled = isEnabled
        }

        access(all)
        fun setNextAddressStartIndex(_ nextAddressStartIndex: UInt64) {
            AccountV2Migration.nextAddressStartIndex = nextAddressStartIndex
        }

        access(all)
        fun setBatchSize(_ batchSize: UInt64) {
            AccountV2Migration.batchSize = batchSize
        }

        access(all)
        fun migrateNextBatch() {
            AccountV2Migration.migrateNextBatch()
        }
    }

    access(all)
    let adminStoragePath: StoragePath

    access(all)
    var isEnabled: Bool

    access(all)
    var nextAddressStartIndex: UInt64

    access(all)
    var batchSize: UInt64

    init() {
        self.adminStoragePath = /storage/accountV2MigrationAdmin
        self.isEnabled = false
        self.nextAddressStartIndex = 1
        self.batchSize = 10

        self.account.storage.save(
            <-create Admin(),
            to: self.adminStoragePath
        )
    }

    access(contract)
    fun migrateNextBatch() {
        if !self.isEnabled {
            return
        }

        let batchSize = self.batchSize
        if batchSize <= 0 {
            return
        }

        let startIndex = self.nextAddressStartIndex

        if !scheduleAccountV2Migration(
            addressStartIndex: startIndex,
            count: batchSize
        ) {
            return
        }

        self.nextAddressStartIndex = startIndex + batchSize

        emit Migrated(
            addressStartIndex: startIndex,
            count: batchSize
        )
    }
}

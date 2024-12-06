import RandomBeaconHistory from "RandomBeaconHistory"
import EVM from "EVM"
import AccountV2Migration from "AccountV2Migration"

transaction {
    prepare(serviceAccount: auth(BorrowValue) &Account) {
        let randomBeaconHistoryHeartbeat = serviceAccount.storage
            .borrow<&RandomBeaconHistory.Heartbeat>(from: RandomBeaconHistory.HeartbeatStoragePath)
            ?? panic("Couldn't borrow RandomBeaconHistory.Heartbeat Resource")
        randomBeaconHistoryHeartbeat.heartbeat(randomSourceHistory: randomSourceHistory())

        let evmHeartbeat = serviceAccount.storage
            .borrow<&EVM.Heartbeat>(from: /storage/EVMHeartbeat)
            ?? panic("Couldn't borrow EVM.Heartbeat Resource")
        evmHeartbeat.heartbeat()

         let accountV2MigrationAdmin = serviceAccount.storage
            .borrow<&AccountV2Migration.Admin>(from: AccountV2Migration.adminStoragePath)
        accountV2MigrationAdmin?.migrateNextBatch()
    }
}

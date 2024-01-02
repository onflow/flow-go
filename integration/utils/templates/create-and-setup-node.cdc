import Crypto
import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS
import FlowIDTableStaking from "FlowIDTableStaking"
import FlowStakingCollection from 0xSTAKINGCOLLECTIONADDRESS

transaction(
    stakingAcctKey: Crypto.KeyListEntry,
    stake: UFix64,
    id: String,
    role: UInt8,
    networkingAddress: String,
    networkingKey: String,
    stakingKey: String,
    machineAcctKey: Crypto.KeyListEntry?) {

    prepare(service: auth(BorrowValue) &Account) {
        // 1 - create the staking account for the new node.
        //
        let stakingAccount = Account(payer: service)
        stakingAccount.keys.add(publicKey: stakingAcctKey.publicKey, hashAlgorithm: stakingAcctKey.hashAlgorithm, weight: stakingAcctKey.weight)

        // 2 - fund the new staking account
        //
        let stakeDst = stakingAccount.capabilities.borrow<&{FungibleToken.Receiver}>(/public/flowTokenReceiver)
            ?? panic("Could not borrow receiver reference to the recipient's Vault")
        // withdraw stake from service account
        let stakeSrc = service.storage.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Could not borrow reference to the owner's Vault!")
        stakeDst.deposit(from: <-stakeSrc.withdraw(amount: stake))

        // 3 - set up the staking collection
        //
        let vaultCap = stakingAccount.capabilities.storage.issue<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(/storage/flowTokenVault)

        // Create a new Staking Collection and put it in storage
        let stakingCollection <-FlowStakingCollection.createStakingCollection(unlockedVault: vaultCap, tokenHolder: nil)
        stakingAccount.storage.save(<-stakingCollection, to: FlowStakingCollection.StakingCollectionStoragePath)

        // Reference must be taken after storing in the storage.
        // Otherwise the reference gets invalidated upon move.
        let stakingCollectionRef = stakingAccount.storage
            .borrow<auth(FlowStakingCollection.CollectionOwner) &FlowStakingCollection.StakingCollection>(
                from: FlowStakingCollection.StakingCollectionStoragePath,
            )
            ?? panic("Could not borrow reference to the staking collection")

        // Create a public link to the staking collection
        let stakingCollectionCap = stakingAccount.capabilities.storage
            .issue<&FlowStakingCollection.StakingCollection>(FlowStakingCollection.StakingCollectionStoragePath)

        stakingAccount.capabilities.publish(
            stakingCollectionCap,
            at: FlowStakingCollection.StakingCollectionPublicPath,
        )

        // 4 - register the node
        //
        if let machineAccount = stakingCollectionRef.registerNode(
            id: id,
            role: role,
            networkingAddress: networkingAddress,
            networkingKey: networkingKey,
            stakingKey: stakingKey,
            amount: stake,
            payer: service,
        ) {
            machineAccount.keys.add(publicKey: machineAcctKey!.publicKey, hashAlgorithm: machineAcctKey!.hashAlgorithm, weight: machineAcctKey!.weight)
        }
    }
}

import Crypto
import FungibleToken from 0xFUNGIBLETOKENADDRESS
import FlowToken from 0xFLOWTOKENADDRESS
import FlowIDTableStaking from 0xIDENTITYTABLEADDRESS
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

    prepare(service: AuthAccount) {
        // 1 - create the staking account for the new node.
        //
        let stakingAccount = AuthAccount(payer: service)
        stakingAccount.keys.add(publicKey: stakingAcctKey.publicKey, hashAlgorithm: stakingAcctKey.hashAlgorithm, weight: stakingAcctKey.weight)

        // 2 - fund the new staking account
        //
        let stakeDst = stakingAccount.getCapability(/public/flowTokenReceiver).borrow<&{FungibleToken.Receiver}>()
            ?? panic("Could not borrow receiver reference to the recipient's Vault")
        // withdraw stake from service account
        let stakeSrc = service.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
            ?? panic("Could not borrow reference to the owner's Vault!")
        stakeDst.deposit(from: <-stakeSrc.withdraw(amount: stake))

        // 3 - set up the staking collection
        //
        let flowToken = stakingAccount.link<&FlowToken.Vault>(/private/flowTokenVault, target: /storage/flowTokenVault)!
        // Create a new Staking Collection and put it in storage
        let stakingCollection <-FlowStakingCollection.createStakingCollection(unlockedVault: flowToken, tokenHolder: nil)
        let stakingCollectionRef = &stakingCollection as &FlowStakingCollection.StakingCollection
        stakingAccount.save(<-stakingCollection, to: FlowStakingCollection.StakingCollectionStoragePath)

        // Create a public link to the staking collection
        stakingAccount.link <&FlowStakingCollection.StakingCollection{FlowStakingCollection.StakingCollectionPublic}> (
            FlowStakingCollection.StakingCollectionPublicPath,
            target: FlowStakingCollection.StakingCollectionStoragePath
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

/*
 * The FlowStorageFees smart contract
 *
 * An account's storage capacity determines up to how much storage on chain it can use. 
 * A storage capacity is calculated by multiplying the amount of reserved flow with `StorageFee.storageMegaBytesPerReservedFLOW`
 * The minimum amount of flow tokens reserved for storage capacity is `FlowStorageFees.minimumStorageReservation` this is paid during account creation, by the creator.
 * 
 * At the end of all transactions, any account that had any value changed in their storage 
 * has their storage capacity checked against their storage used and their main flow token vault against the minimum reservation.
 * If any account fails this check the transaction wil fail.
 * 
 * An account moving/deleting its `FlowToken.Vault` resource will result 
 * in the transaction failing because the account will have no storage capacity.
 * 
 */

import FungibleToken from 0xf233dcee88fe0abe
import FlowToken from 0x1654653399040a61

pub contract FlowStorageFees {

    // Emitted when the amount of storage capacity an account has per reserved Flow token changes
    pub event StorageMegaBytesPerReservedFLOWChanged(_ storageMegaBytesPerReservedFLOW: UFix64)

    // Emitted when the minimum amount of Flow tokens that an account needs to have reserved for storage capacity changes.
    pub event MinimumStorageReservationChanged(_ minimumStorageReservation: UFix64)

    // Defines how much storage capacity every account has per reserved Flow token.
    // definition is written per unit of flow instead of the inverse, 
    // so there is no loss of precision calculating storage from flow, 
    // but there is loss of precision when calculating flow per storage.
    pub var storageMegaBytesPerReservedFLOW: UFix64

    // Defines the minimum amount of Flow tokens that every account needs to have reserved for storage capacity.
    // If an account has less then this amount reserved by the end of any transaction it participated in, the transaction will fail.
    pub var minimumStorageReservation: UFix64

    // An administrator resource that can change the parameters of the FlowStorageFees smart contract.
    pub resource Administrator {

        // Changes the amount of storage capacity an account has per accounts' reserved storage FLOW.
        pub fun setStorageMegaBytesPerReservedFLOW(_ storageMegaBytesPerReservedFLOW: UFix64) {
            if FlowStorageFees.storageMegaBytesPerReservedFLOW == storageMegaBytesPerReservedFLOW {
              return
            }
            FlowStorageFees.storageMegaBytesPerReservedFLOW = storageMegaBytesPerReservedFLOW
            emit StorageMegaBytesPerReservedFLOWChanged(storageMegaBytesPerReservedFLOW)
        }

        // Changes the minimum amount of FLOW an account has to have reserved.
        pub fun setMinimumStorageReservation(_ minimumStorageReservation: UFix64) {
            if FlowStorageFees.minimumStorageReservation == minimumStorageReservation {
              return
            }
            FlowStorageFees.minimumStorageReservation = minimumStorageReservation
            emit MinimumStorageReservationChanged(minimumStorageReservation)
        }

        access(contract) init(){}
    }

    // Returns megabytes
    pub fun calculateAccountCapacity(_ accountAddress: Address): UFix64 {
        let balanceRef = getAccount(accountAddress)
            .getCapability<&FlowToken.Vault{FungibleToken.Balance}>(/public/flowTokenBalance)!
            .borrow() ?? panic("Could not borrow FLOW balance capability")

        // get address token balance
        if balanceRef.balance < self.minimumStorageReservation {
            // if < then minimum return 0
            return 0.0
        } else {
            // return balance multiplied with megabytes per flow 
            return balanceRef.balance.saturatingMultiply(self.storageMegaBytesPerReservedFLOW)
        }
    }

    // Amount in Flow tokens
    // Returns megabytes
    pub fun flowToStorageCapacity(_ amount: UFix64): UFix64 {
        return amount.saturatingMultiply(FlowStorageFees.storageMegaBytesPerReservedFLOW)
    }

    // Amount in megabytes
    // Returns Flow tokens
    pub fun storageCapacityToFlow(_ amount: UFix64): UFix64 {
        if FlowStorageFees.storageMegaBytesPerReservedFLOW == 0.0 as UFix64 {
            return 0.0 as UFix64
        }
        // possible loss of precision
        // putting the result back into `flowToStorageCapacity` might not yield the same result
        return amount / FlowStorageFees.storageMegaBytesPerReservedFLOW
    }

    // converts storage used from UInt64 Bytes to UFix64 Megabytes.
    pub fun convertUInt64StorageBytesToUFix64Megabytes(_ storage: UInt64): UFix64 {
        // safe convert UInt64 to UFix64 (without overflow)
        let f = UFix64(storage % 100000000 as UInt64) * 0.00000001 as UFix64 + UFix64(storage / 100000000 as UInt64)
        // decimal point correction. Megabytes to bytes have a conversion of 10^-6 while UFix64 minimum value is 10^-8
        let storageMb = f.saturatingMultiply(100.0)
        return storageMb
    }

    // Gets "available" balance of an account
    // The available balance is its default token balance minus what is reserved for storage.
    pub fun defaultTokenAvailableBalance(_ accountAddress: Address): UFix64 {
        //get balance of account
        let acct = getAccount(accountAddress)
        let balanceRef = acct
            .getCapability(/public/flowTokenBalance)
            .borrow<&FlowToken.Vault{FungibleToken.Balance}>()!
        let balance = balanceRef.balance

        // get how much should be reserved for storage
        var reserved = self.storageCapacityToFlow(self.convertUInt64StorageBytesToUFix64Megabytes(acct.storageUsed))
        // at least self.minimumStorageReservation should be reserved
        if reserved < self.minimumStorageReservation {
            reserved = self.minimumStorageReservation
        }

        return balance.saturatingSubtract(reserved)
    }

    init() {
        self.storageMegaBytesPerReservedFLOW = 1.0 // 1 Mb per 1 Flow token
        self.minimumStorageReservation = 0.0 // or 0 kb of minimum storage reservation

        let admin <- create Administrator()
        self.account.save(<-admin, to: /storage/storageFeesAdmin)
    }
}
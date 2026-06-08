import "FlowToken"
import "EVM"
import "FlowTransactionScheduler"
import "FlowTransactionSchedulerUtils"

access(all) contract FlowEVMScheduler {

    access(self) var evmRegister: EVM.EVMAddress
    access(contract) var escrowedFees: {UInt64: EVM.EVMAddress}
    access(self) var evmTransactionHandlerCap: Capability<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>

    access(all) resource EVMTransactionHandler: FlowTransactionScheduler.TransactionHandler {
        access(FlowTransactionScheduler.Execute) fun executeTransaction(id: UInt64, data: AnyStruct?) {
            let coa = FlowEVMScheduler.account.storage.borrow<auth(EVM.Owner) &EVM.CadenceOwnedAccount>(
                from: /storage/coa
            ) ?? panic("Could not borrow COA")
            let txArgs = (data as? [AnyStruct])!
            let contractAddress = (txArgs[0] as? [UInt8; 20])!
            let callArgs = (txArgs[1] as? [UInt8])!

            let txRes = coa.call(
                to: EVM.EVMAddress(bytes: contractAddress),
                data: callArgs,
                gasLimit: 1_500_000,
                value: EVM.Balance(attoflow: 0)
            )
            assert(txRes.status == EVM.Status.successful, message: txRes.errorMessage)
			assert(txRes.errorCode == 0, message: "unexpected error code: \(txRes.errorCode)")
            FlowEVMScheduler.escrowedFees.remove(key: id)
        }

        access(all) view fun getViews(): [Type] {
            return [
                Type<StoragePath>(),
                Type<PublicPath>()
            ]
        }

        access(all) fun resolveView(_ viewType: Type): AnyStruct? {
            if viewType == Type<StoragePath>() {
                return /storage/evmTransactionHandler
            } else if viewType == Type<PublicPath>() {
                return /public/evmTransactionHandler
            } else {
                return nil
            }
        }
    }

    access(self) fun scheduleTransaction(
        timestamp: UFix64,
        priority: UInt8,
        executionEffort: UInt64,
        callerAddress: [UInt8; 20],
        feeAmount: UInt256,
        contractAddress: [UInt8; 20],
        callArgs: [UInt8]
    ): UInt64 {
        let manager = self._getManagerFromStorage() ?? panic("Could not borrow manager")
        let coa = self.account.storage.borrow<auth(EVM.Owner) &EVM.CadenceOwnedAccount>(
            from: /storage/coa
        ) ?? panic("Could not borrow COA")

        let fees <- coa.withdraw(balance: EVM.Balance(attoflow: UInt(feeAmount)))
        let txID = manager.schedule(
            handlerCap: self.evmTransactionHandlerCap,
            data: [contractAddress, callArgs],
            timestamp: timestamp,
            priority: FlowTransactionScheduler.Priority(rawValue: priority)!,
            executionEffort: executionEffort,
            fees: <- fees
        )
        let caller = EVM.EVMAddress(bytes: callerAddress)
        self.escrowedFees[txID] = caller

        return txID
    }

    access(self) fun cancelTransaction(id: UInt64) {
        let manager = self._getManagerFromStorage() ?? panic("Could not borrow manager")
        let fees <- manager.cancel(id: id)
        let caller = self.escrowedFees[id]!
        caller.deposit(from: <-fees)
        self.escrowedFees.remove(key: id)
    }

    access(all) view fun getTransactionStatus(id: UInt64): UInt8 {
        let manager = self._getManagerFromStorage() ?? panic("Could not borrow manager")
        let txStatus = manager.getTransactionStatus(id: id)!
        return txStatus.rawValue
    }

    access(all) fun estimate(
        timestamp: UFix64,
        priority: UInt8,
        executionEffort: UInt64,
        contractAddress: [UInt8; 20],
        callArgs: [UInt8]
    ): UFix64 {
        let estimate = FlowTransactionScheduler.estimate(
            data: [EVM.EVMAddress(bytes: contractAddress), String.encodeHex(callArgs)],
            timestamp: timestamp,
            priority: FlowTransactionScheduler.Priority(rawValue: priority)!,
            executionEffort: executionEffort,
        )
        if let error = estimate.error {
            panic(error)
        }
        return estimate.flowFee!
    }

    access(self) view fun _getManagerFromStorage():
     auth(FlowTransactionSchedulerUtils.Owner) &{FlowTransactionSchedulerUtils.Manager}? {
        return self.account.storage
            .borrow<auth(FlowTransactionSchedulerUtils.Owner) &{FlowTransactionSchedulerUtils.Manager}>
            (from: FlowTransactionSchedulerUtils.managerStoragePath)
    }

    access(all) fun setEvmRegister(address: EVM.EVMAddress) {
        self.evmRegister = address
    }

    init() {
        let manager <- FlowTransactionSchedulerUtils.createManager()
        self.account.storage.save(<-manager, to: FlowTransactionSchedulerUtils.managerStoragePath)

        let managerCapPublic = self.account.capabilities.storage
            .issue<&{FlowTransactionSchedulerUtils.Manager}>(FlowTransactionSchedulerUtils.managerStoragePath)
        self.account.capabilities.publish(managerCapPublic, at: FlowTransactionSchedulerUtils.managerPublicPath)

        let cadenceOwnedAccount <- EVM.createCadenceOwnedAccount()
        self.account.storage.save(<-cadenceOwnedAccount, to: /storage/coa)

        let evmTransactionHandler <-create EVMTransactionHandler()
        self.account.storage.save(<-evmTransactionHandler, to: /storage/evmTransactionHandler)

        self.evmTransactionHandlerCap = self.account.capabilities.storage
            .issue<auth(FlowTransactionScheduler.Execute) &{FlowTransactionScheduler.TransactionHandler}>(/storage/evmTransactionHandler)

        self.evmRegister = EVM.EVMAddress(bytes: [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0])
        self.escrowedFees = {}
    }
}

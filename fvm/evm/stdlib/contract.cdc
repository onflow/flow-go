import Crypto
import "NonFungibleToken"
import "FungibleToken"
import "FlowToken"

access(all)
contract EVM {

    // Entitlements enabling finer-grained access control on a CadenceOwnedAccount
    access(all) entitlement Validate
    access(all) entitlement Withdraw
    access(all) entitlement Call
    access(all) entitlement Deploy
    access(all) entitlement Owner
    access(all) entitlement Bridge

    /// Block executed event is emitted when a new block is created,
    /// which always happens when a transaction is executed.
    access(all)
    event BlockExecuted(
        // height or number of the block
        height: UInt64,
        // hash of the block
        hash: [UInt8; 32],
        // timestamp of the block creation
        timestamp: UInt64,
        // total Flow supply
        totalSupply: Int,
        // all gas used in the block by transactions included
        totalGasUsed: UInt64,
        // parent block hash
        parentHash: [UInt8; 32],
        // root hash of all the transaction receipts
        receiptRoot: [UInt8; 32],
        // root hash of all the transaction hashes
        transactionHashRoot: [UInt8; 32],
    )

    /// Transaction executed event is emitted every time a transaction
    /// is executed by the EVM (even if failed).
    access(all)
    event TransactionExecuted(
        // hash of the transaction
        hash: [UInt8; 32],
        // index of the transaction in a block
        index: UInt16,
        // type of the transaction
        type: UInt8,
        // RLP encoded transaction payload
        payload: [UInt8],
        // code indicating a specific validation (201-300) or execution (301-400) error
        errorCode: UInt16,
        // a human-readable message about the error (if any)
        errorMessage: String,
        // the amount of gas transaction used
        gasConsumed: UInt64,
        // if transaction was a deployment contains a newly deployed contract address
        contractAddress: String,
        // RLP encoded logs
        logs: [UInt8],
        // block height in which transaction was included
        blockHeight: UInt64,
        /// captures the hex encoded data that is returned from
        /// the evm. For contract deployments
        /// it returns the code deployed to
        /// the address provided in the contractAddress field.
        /// in case of revert, the smart contract custom error message
        /// is also returned here (see EIP-140 for more details).
        returnedData: [UInt8],
        /// captures the input and output of the calls (rlp encoded) to the extra
        /// precompiled contracts (e.g. Cadence Arch) during the transaction execution.
        /// This data helps to replay the transactions without the need to
        /// have access to the full cadence state data.
        precompiledCalls: [UInt8]
    )

    access(all)
    event CadenceOwnedAccountCreated(address: String)

    /// FLOWTokensDeposited is emitted when FLOW tokens is bridged
    /// into the EVM environment. Note that this event is not emitted
    /// for transfer of flow tokens between two EVM addresses.
    /// Similar to the FungibleToken.Deposited event
    /// this event includes a depositedUUID that captures the
    /// uuid of the source vault.
    access(all)
    event FLOWTokensDeposited(
        address: String,
        amount: UFix64,
        depositedUUID: UInt64,
        balanceAfterInAttoFlow: UInt
    )

    /// FLOWTokensWithdrawn is emitted when FLOW tokens are bridged
    /// out of the EVM environment. Note that this event is not emitted
    /// for transfer of flow tokens between two EVM addresses.
    /// similar to the FungibleToken.Withdrawn events
    /// this event includes a withdrawnUUID that captures the
    /// uuid of the returning vault.
    access(all)
    event FLOWTokensWithdrawn(
        address: String,
        amount: UFix64,
        withdrawnUUID: UInt64,
        balanceAfterInAttoFlow: UInt
    )

    /// BridgeAccessorUpdated is emitted when the BridgeAccessor Capability
    /// is updated in the stored BridgeRouter along with identifying
    /// information about both.
    access(all)
    event BridgeAccessorUpdated(
        routerType: Type,
        routerUUID: UInt64,
        routerAddress: Address,
        accessorType: Type,
        accessorUUID: UInt64,
        accessorAddress: Address
    )

    /// EVMAddress is an EVM-compatible address
    access(all)
    struct EVMAddress {

        /// Bytes of the address
        access(all)
        let bytes: [UInt8; 20]

        /// Constructs a new EVM address from the given byte representation
        view init(bytes: [UInt8; 20]) {
            self.bytes = bytes
        }

        /// Balance of the address
        access(all)
        view fun balance(): Balance {
            let balance = InternalEVM.balance(
                address: self.bytes
            )
            return Balance(attoflow: balance)
        }

        /// Nonce of the address
        access(all)
        fun nonce(): UInt64 {
            return InternalEVM.nonce(
                address: self.bytes
            )
        }

        /// Code of the address
        access(all)
        fun code(): [UInt8] {
            return InternalEVM.code(
                address: self.bytes
            )
        }

        /// CodeHash of the address
        access(all)
        fun codeHash(): [UInt8] {
            return InternalEVM.codeHash(
                address: self.bytes
            )
        }

        /// Deposits the given vault into the EVM account with the given address
        access(all)
        fun deposit(from: @FlowToken.Vault) {
            let amount = from.balance
            if amount == 0.0 {
                panic("calling deposit function with an empty vault is not allowed")
            }
            let depositedUUID = from.uuid
            InternalEVM.deposit(
                from: <-from,
                to: self.bytes
            )
            emit FLOWTokensDeposited(
                address: self.toString(),
                amount: amount,
                depositedUUID: depositedUUID,
                balanceAfterInAttoFlow: self.balance().attoflow
            )
        }

        /// Serializes the address to a hex string without the 0x prefix
        /// Future implementations should pass data to InternalEVM for native serialization
        access(all)
        view fun toString(): String {
            return String.encodeHex(self.bytes.toVariableSized())
        }

        /// Compares the address with another address
        access(all)
        view fun equals(_ other: EVMAddress): Bool {
            return self.bytes == other.bytes
        }
    }

    /// Converts a hex string to an EVM address if the string is a valid hex string
    /// Future implementations should pass data to InternalEVM for native deserialization
    access(all)
    fun addressFromString(_ asHex: String): EVMAddress {
        pre {
            asHex.length == 40 || asHex.length == 42: "Invalid hex string length for an EVM address"
        }
        // Strip the 0x prefix if it exists
        var withoutPrefix = (asHex[1] == "x" ? asHex.slice(from: 2, upTo: asHex.length) : asHex).toLower()
        let bytes = withoutPrefix.decodeHex().toConstantSized<[UInt8; 20]>()!
        return EVMAddress(bytes: bytes)
    }

    access(all)
    struct Balance {

        /// The balance in atto-FLOW
        /// Atto-FLOW is the smallest denomination of FLOW (1e18 FLOW)
        /// that is used to store account balances inside EVM
        /// similar to the way WEI is used to store ETH divisible to 18 decimal places.
        access(all)
        var attoflow: UInt

        /// Constructs a new balance
        access(all)
        view init(attoflow: UInt) {
            self.attoflow = attoflow
        }

        /// Sets the balance by a UFix64 (8 decimal points), the format
        /// that is used in Cadence to store FLOW tokens.
        access(all)
        fun setFLOW(flow: UFix64){
            self.attoflow = InternalEVM.castToAttoFLOW(balance: flow)
        }

        /// Casts the balance to a UFix64 (rounding down)
        /// Warning! casting a balance to a UFix64 which supports a lower level of precision
        /// (8 decimal points in compare to 18) might result in rounding down error.
        /// Use the toAttoFlow function if you care need more accuracy.
        access(all)
        view fun inFLOW(): UFix64 {
            return InternalEVM.castToFLOW(balance: self.attoflow)
        }

        /// Returns the balance in Atto-FLOW
        access(all)
        view fun inAttoFLOW(): UInt {
            return self.attoflow
        }

        /// Returns true if the balance is zero
        access(all)
        fun isZero(): Bool {
            return self.attoflow == 0
        }
    }

    /// reports the status of evm execution.
    access(all) enum Status: UInt8 {
        /// is (rarely) returned when status is unknown
        /// and something has gone very wrong.
        access(all) case unknown

        /// is returned when execution of an evm transaction/call
        /// has failed at the validation step (e.g. nonce mismatch).
        /// An invalid transaction/call is rejected to be executed
        /// or be included in a block.
        access(all) case invalid

        /// is returned when execution of an evm transaction/call
        /// has been successful but the vm has reported an error as
        /// the outcome of execution (e.g. running out of gas).
        /// A failed tx/call is included in a block.
        /// Note that resubmission of a failed transaction would
        /// result in invalid status in the second attempt, given
        /// the nonce would be come invalid.
        access(all) case failed

        /// is returned when execution of an evm transaction/call
        /// has been successful and no error is reported by the vm.
        access(all) case successful
    }

    /// reports the outcome of evm transaction/call execution attempt
    access(all) struct Result {
        /// status of the execution
        access(all)
        let status: Status

        /// error code (error code zero means no error)
        access(all)
        let errorCode: UInt64

        /// error message
        access(all)
        let errorMessage: String

        /// returns the amount of gas metered during
        /// evm execution
        access(all)
        let gasUsed: UInt64

        /// returns the data that is returned from
        /// the evm for the call. For coa.deploy
        /// calls it returns the code deployed to
        /// the address provided in the contractAddress field.
        /// in case of revert, the smart contract custom error message
        /// is also returned here (see EIP-140 for more details).
        access(all)
        let data: [UInt8]

        /// returns the newly deployed contract address
        /// if the transaction caused such a deployment
        /// otherwise the value is nil.
        access(all)
        let deployedContract: EVMAddress?

        init(
            status: Status,
            errorCode: UInt64,
            errorMessage: String,
            gasUsed: UInt64,
            data: [UInt8],
            contractAddress: [UInt8; 20]?
        ) {
            self.status = status
            self.errorCode = errorCode
            self.errorMessage = errorMessage
            self.gasUsed = gasUsed
            self.data = data

            if let addressBytes = contractAddress {
                self.deployedContract = EVMAddress(bytes: addressBytes)
            } else {
                self.deployedContract = nil
            }
        }
    }

    access(all)
    resource interface Addressable {
        /// The EVM address
        access(all)
        view fun address(): EVMAddress
    }

    access(all)
    resource CadenceOwnedAccount: Addressable {

        access(self)
        var addressBytes: [UInt8; 20]

        init() {
            // address is initially set to zero
            // but updated through initAddress later
            // we have to do this since we need resource id (uuid)
            // to calculate the EVM address for this cadence owned account
            self.addressBytes = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]
        }

        access(contract)
        fun initAddress(addressBytes: [UInt8; 20]) {
           // only allow set address for the first time
           // check address is empty
            for item in self.addressBytes {
                assert(item == 0, message: "address byte is not empty")
            }
           self.addressBytes = addressBytes
        }

        /// The EVM address of the cadence owned account
        access(all)
        view fun address(): EVMAddress {
            // Always create a new EVMAddress instance
            return EVMAddress(bytes: self.addressBytes)
        }

        /// Get balance of the cadence owned account
        access(all)
        view fun balance(): Balance {
            return self.address().balance()
        }

        /// Deposits the given vault into the cadence owned account's balance
        access(all)
        fun deposit(from: @FlowToken.Vault) {
            self.address().deposit(from: <-from)
        }

        /// The EVM address of the cadence owned account behind an entitlement, acting as proof of access
        access(Owner | Validate)
        view fun protectedAddress(): EVMAddress {
            return self.address()
        }

        /// Withdraws the balance from the cadence owned account's balance
        /// Note that amounts smaller than 10nF (10e-8) can't be withdrawn
        /// given that Flow Token Vaults use UFix64s to store balances.
        /// If the given balance conversion to UFix64 results in
        /// rounding error, this function would fail.
        access(Owner | Withdraw)
        fun withdraw(balance: Balance): @FlowToken.Vault {
            if balance.isZero() {
                panic("calling withdraw function with zero balance is not allowed")
            }
            let vault <- InternalEVM.withdraw(
                from: self.addressBytes,
                amount: balance.attoflow
            ) as! @FlowToken.Vault
            emit FLOWTokensWithdrawn(
                address: self.address().toString(),
                amount: balance.inFLOW(),
                withdrawnUUID: vault.uuid,
                balanceAfterInAttoFlow: self.balance().attoflow
            )
            return <-vault
        }

        /// Deploys a contract to the EVM environment.
        /// Returns the result which contains address of
        /// the newly deployed contract
        access(Owner | Deploy)
        fun deploy(
            code: [UInt8],
            gasLimit: UInt64,
            value: Balance
        ): Result {
            return InternalEVM.deploy(
                from: self.addressBytes,
                code: code,
                gasLimit: gasLimit,
                value: value.attoflow
            ) as! Result
        }

        /// Calls a function with the given data.
        /// The execution is limited by the given amount of gas
        access(Owner | Call)
        fun call(
            to: EVMAddress,
            data: [UInt8],
            gasLimit: UInt64,
            value: Balance
        ): Result {
            return InternalEVM.call(
                from: self.addressBytes,
                to: to.bytes,
                data: data,
                gasLimit: gasLimit,
                value: value.attoflow
            ) as! Result
        }

        /// Bridges the given NFT to the EVM environment, requiring a Provider from which to withdraw a fee to fulfill
        /// the bridge request
        access(all)
        fun depositNFT(
            nft: @{NonFungibleToken.NFT},
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ) {
            EVM.borrowBridgeAccessor().depositNFT(nft: <-nft, to: self.address(), feeProvider: feeProvider)
        }

        /// Bridges the given NFT from the EVM environment, requiring a Provider from which to withdraw a fee to fulfill
        /// the bridge request. Note: the caller should own the requested NFT in EVM
        access(Owner | Bridge)
        fun withdrawNFT(
            type: Type,
            id: UInt256,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ): @{NonFungibleToken.NFT} {
            return <- EVM.borrowBridgeAccessor().withdrawNFT(
                caller: &self as auth(Call) &CadenceOwnedAccount,
                type: type,
                id: id,
                feeProvider: feeProvider
            )
        }

        /// Bridges the given Vault to the EVM environment, requiring a Provider from which to withdraw a fee to fulfill
        /// the bridge request
        access(all)
        fun depositTokens(
            vault: @{FungibleToken.Vault},
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ) {
            EVM.borrowBridgeAccessor().depositTokens(vault: <-vault, to: self.address(), feeProvider: feeProvider)
        }

        /// Bridges the given fungible tokens from the EVM environment, requiring a Provider from which to withdraw a
        /// fee to fulfill the bridge request. Note: the caller should own the requested tokens & sufficient balance of
        /// requested tokens in EVM
        access(Owner | Bridge)
        fun withdrawTokens(
            type: Type,
            amount: UInt256,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ): @{FungibleToken.Vault} {
            return <- EVM.borrowBridgeAccessor().withdrawTokens(
                caller: &self as auth(Call) &CadenceOwnedAccount,
                type: type,
                amount: amount,
                feeProvider: feeProvider
            )
        }
    }

    /// Creates a new cadence owned account
    access(all)
    fun createCadenceOwnedAccount(): @CadenceOwnedAccount {
        let acc <-create CadenceOwnedAccount()
        let addr = InternalEVM.createCadenceOwnedAccount(uuid: acc.uuid)
        acc.initAddress(addressBytes: addr)

        emit CadenceOwnedAccountCreated(address: acc.address().toString())
        return <-acc
    }

    /// Runs an a RLP-encoded EVM transaction, deducts the gas fees,
    /// and deposits the gas fees into the provided coinbase address.
    access(all)
    fun run(tx: [UInt8], coinbase: EVMAddress): Result {
        return InternalEVM.run(
                tx: tx,
                coinbase: coinbase.bytes
        ) as! Result
    }

    /// mustRun runs the transaction using EVM.run yet it
    /// rollback if the tx execution status is unknown or invalid.
    /// Note that this method does not rollback if transaction
    /// is executed but an vm error is reported as the outcome
    /// of the execution (status: failed).
    access(all)
    fun mustRun(tx: [UInt8], coinbase: EVMAddress): Result {
        let runResult = self.run(tx: tx, coinbase: coinbase)
        assert(
            runResult.status == Status.failed || runResult.status == Status.successful,
            message: "tx is not valid for execution"
        )
        return runResult
    }

    /// Simulates running unsigned RLP-encoded transaction using
    /// the from address as the signer.
    /// The transaction state changes are not persisted.
    /// This is useful for gas estimation or calling view contract functions.
    access(all)
    fun dryRun(tx: [UInt8], from: EVMAddress): Result {
        return InternalEVM.dryRun(
            tx: tx,
            from: from.bytes,
        ) as! Result
    }

    /// Runs a batch of RLP-encoded EVM transactions, deducts the gas fees,
    /// and deposits the gas fees into the provided coinbase address.
    /// An invalid transaction is not executed and not included in the block.
    access(all)
    fun batchRun(txs: [[UInt8]], coinbase: EVMAddress): [Result] {
        return InternalEVM.batchRun(
            txs: txs,
            coinbase: coinbase.bytes,
        ) as! [Result]
    }

    access(all)
    fun encodeABI(_ values: [AnyStruct]): [UInt8] {
        return InternalEVM.encodeABI(values)
    }

    access(all)
    fun decodeABI(types: [Type], data: [UInt8]): [AnyStruct] {
        return InternalEVM.decodeABI(types: types, data: data)
    }

    access(all)
    fun encodeABIWithSignature(
        _ signature: String,
        _ values: [AnyStruct]
    ): [UInt8] {
        let methodID = HashAlgorithm.KECCAK_256.hash(
            signature.utf8
        ).slice(from: 0, upTo: 4)
        let arguments = InternalEVM.encodeABI(values)

        return methodID.concat(arguments)
    }

    access(all)
    fun decodeABIWithSignature(
        _ signature: String,
        types: [Type],
        data: [UInt8]
    ): [AnyStruct] {
        let methodID = HashAlgorithm.KECCAK_256.hash(
            signature.utf8
        ).slice(from: 0, upTo: 4)

        for byte in methodID {
            if byte != data.removeFirst() {
                panic("signature mismatch")
            }
        }

        return InternalEVM.decodeABI(types: types, data: data)
    }

    /// ValidationResult returns the result of COA ownership proof validation
    access(all)
    struct ValidationResult {
        access(all)
        let isValid: Bool

        access(all)
        let problem: String?

        init(isValid: Bool, problem: String?) {
            self.isValid = isValid
            self.problem = problem
        }
    }

    /// validateCOAOwnershipProof validates a COA ownership proof
    access(all)
    fun validateCOAOwnershipProof(
        address: Address,
        path: PublicPath,
        signedData: [UInt8],
        keyIndices: [UInt64],
        signatures: [[UInt8]],
        evmAddress: [UInt8; 20]
    ): ValidationResult {

        // make signature set first
        // check number of signatures matches number of key indices
        if keyIndices.length != signatures.length {
            return ValidationResult(
                isValid: false,
                problem: "key indices size doesn't match the signatures"
            )
        }

        var signatureSet: [Crypto.KeyListSignature] = []
        for signatureIndex, signature in signatures{
            signatureSet.append(Crypto.KeyListSignature(
                keyIndex: Int(keyIndices[signatureIndex]),
                signature: signature
            ))
        }

        // fetch account
        let acc = getAccount(address)

        // constructing key list
        let keyList = Crypto.KeyList()
        for signature in signatureSet {
            let keyRef = acc.keys.get(keyIndex: signature.keyIndex)
            if keyRef == nil {
                return ValidationResult(
                    isValid: false,
                    problem: "invalid key index"
                )
            }
            let key = keyRef!
            if key.isRevoked {
                return ValidationResult(
                    isValid: false,
                    problem: "account key is revoked"
                )
            }
            keyList.add(
              key.publicKey,
              hashAlgorithm: key.hashAlgorithm,
              weight: key.weight,
           )
        }

        let isValid = keyList.verify(
            signatureSet: signatureSet,
            signedData: signedData,
            domainSeparationTag: "FLOW-V0.0-user"
        )

        if !isValid{
            return ValidationResult(
                isValid: false,
                problem: "the given signatures are not valid or provide enough weight"
            )
        }

        let coaRef = acc.capabilities.borrow<&EVM.CadenceOwnedAccount>(path)
        if coaRef == nil {
             return ValidationResult(
                 isValid: false,
                 problem: "could not borrow bridge account's resource"
             )
        }

        // verify evm address matching
        var addr = coaRef!.address()
        for index, item in coaRef!.address().bytes {
            if item != evmAddress[index] {
                return ValidationResult(
                    isValid: false,
                    problem: "evm address mismatch"
                )
            }
        }

        return ValidationResult(
            isValid: true,
            problem: nil
        )
    }

    /// Block returns information about the latest executed block.
    access(all)
    struct EVMBlock {
        access(all)
        let height: UInt64

        access(all)
        let hash: String

        access(all)
        let totalSupply: Int

        access(all)
        let timestamp: UInt64

        init(height: UInt64, hash: String, totalSupply: Int, timestamp: UInt64) {
            self.height = height
            self.hash = hash
            self.totalSupply = totalSupply
            self.timestamp = timestamp
        }
    }

    /// Returns the latest executed block.
    access(all)
    fun getLatestBlock(): EVMBlock {
        return InternalEVM.getLatestBlock() as! EVMBlock
    }

    /// Interface for a resource which acts as an entrypoint to the VM bridge
    access(all)
    resource interface BridgeAccessor {

        /// Endpoint enabling the bridging of an NFT to EVM
        access(Bridge)
        fun depositNFT(
            nft: @{NonFungibleToken.NFT},
            to: EVMAddress,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        )

        /// Endpoint enabling the bridging of an NFT from EVM
        access(Bridge)
        fun withdrawNFT(
            caller: auth(Call) &CadenceOwnedAccount,
            type: Type,
            id: UInt256,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ): @{NonFungibleToken.NFT}

        /// Endpoint enabling the bridging of a fungible token vault to EVM
        access(Bridge)
        fun depositTokens(
            vault: @{FungibleToken.Vault},
            to: EVMAddress,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        )

        /// Endpoint enabling the bridging of fungible tokens from EVM
        access(Bridge)
        fun withdrawTokens(
            caller: auth(Call) &CadenceOwnedAccount,
            type: Type,
            amount: UInt256,
            feeProvider: auth(FungibleToken.Withdraw) &{FungibleToken.Provider}
        ): @{FungibleToken.Vault}
    }

    /// Interface which captures a Capability to the bridge Accessor, saving it within the BridgeRouter resource
    access(all)
    resource interface BridgeRouter {

        /// Returns a reference to the BridgeAccessor designated for internal bridge requests
        access(Bridge) view fun borrowBridgeAccessor(): auth(Bridge) &{BridgeAccessor}

        /// Sets the BridgeAccessor Capability in the BridgeRouter
        access(Bridge) fun setBridgeAccessor(_ accessor: Capability<auth(Bridge) &{BridgeAccessor}>) {
            pre {
                accessor.check(): "Invalid BridgeAccessor Capability provided"
                emit BridgeAccessorUpdated(
                    routerType: self.getType(),
                    routerUUID: self.uuid,
                    routerAddress: self.owner?.address ?? panic("Router must have an owner to be identified"),
                    accessorType: accessor.borrow()!.getType(),
                    accessorUUID: accessor.borrow()!.uuid,
                    accessorAddress: accessor.address
                )
            }
        }
    }

    /// Returns a reference to the BridgeAccessor designated for internal bridge requests
    access(self)
    view fun borrowBridgeAccessor(): auth(Bridge) &{BridgeAccessor} {
        return self.account.storage.borrow<auth(Bridge) &{BridgeRouter}>(from: /storage/evmBridgeRouter)
            ?.borrowBridgeAccessor()
            ?? panic("Could not borrow reference to the EVM bridge")
    }

    /// The Heartbeat resource controls the block production.
    /// It is stored in the storage and used in the Flow protocol to call the heartbeat function once per block.
    access(all)
    resource Heartbeat {
        /// heartbeat calls commit block proposals and forms new blocks including all the
        /// recently executed transactions.
        /// The Flow protocol makes sure to call this function once per block as a system call.
        access(all)
        fun heartbeat() {
            InternalEVM.commitBlockProposal()
        }
    }

    /// setupHeartbeat creates a heartbeat resource and saves it to storage.
    /// The function is called once during the contract initialization.
    ///
    /// The heartbeat resource is used to control the block production,
    /// and used in the Flow protocol to call the heartbeat function once per block.
    ///
    /// The function can be called by anyone, but only once:
    /// the function will fail if the resource already exists.
    ///
    /// The resulting resource is stored in the account storage,
    /// and is only accessible by the account, not the caller of the function.
    access(all)
    fun setupHeartbeat() {
        self.account.storage.save(<-create Heartbeat(), to: /storage/EVMHeartbeat)
    }

    init() {
        self.setupHeartbeat()
    }
}

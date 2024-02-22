import Crypto
import "FlowToken"

access(all)
contract EVM {
    
    // Entitlements enabling finer-graned access control on a CadenceOwnedAccount
    access(all) entitlement Validate
    access(all) entitlement Withdraw
    access(all) entitlement Call
    access(all) entitlement Deploy
    access(all) entitlement Owner

    access(all)
    event CadenceOwnedAccountCreated(addressBytes: [UInt8; 20])

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
        init(attoflow: UInt) {
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

        /// returns the amount of gas metered during
        /// evm execution
        access(all)
        let gasUsed: UInt64

        /// returns the data that is returned from
        /// the evm for the call. For coa.deploy
        /// calls it returns the address bytes of the
        /// newly deployed contract.
        access(all)
        let data: [UInt8]

        init(
            status: Status,
            errorCode: UInt64,
            gasUsed: UInt64,
            data: [UInt8]
        ) {
            self.status = status
            self.errorCode = errorCode
            self.gasUsed = gasUsed
            self.data = data
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
            InternalEVM.deposit(
                from: <-from,
                to: self.addressBytes
            )
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
            let vault <- InternalEVM.withdraw(
                from: self.addressBytes,
                amount: balance.attoflow
            ) as! @FlowToken.Vault
            return <-vault
        }

        /// Deploys a contract to the EVM environment.
        /// Returns the address of the newly deployed contract
        access(Owner | Deploy)
        fun deploy(
            code: [UInt8],
            gasLimit: UInt64,
            value: Balance
        ): EVMAddress {
            let addressBytes = InternalEVM.deploy(
                from: self.addressBytes,
                code: code,
                gasLimit: gasLimit,
                value: value.attoflow
            )
            return EVMAddress(bytes: addressBytes)
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
    }

    /// Creates a new cadence owned account
    access(all)
    fun createCadenceOwnedAccount(): @CadenceOwnedAccount {
        let acc <-create CadenceOwnedAccount()
        let addr = InternalEVM.createCadenceOwnedAccount(uuid: acc.uuid)
        acc.initAddress(addressBytes: addr)
        emit CadenceOwnedAccountCreated(addressBytes: addr)
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
            let key = acc.keys.get(keyIndex: signature.keyIndex)!
            assert(!key.isRevoked, message: "revoked key is used")
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
}

import FanTopToken from 0x86185fba578bc773
import FanTopMarket from 0x86185fba578bc773
import FanTopSerial from 0x86185fba578bc773
import Signature from 0x86185fba578bc773

pub contract FanTopPermissionV2a {
    pub event PermissionAdded(target: Address, role: String)
    pub event PermissionRemoved(target: Address, role: String)

    pub let ownerStoragePath: StoragePath
    pub let receiverStoragePath: StoragePath
    pub let receiverPublicPath: PublicPath

    pub resource interface Role {
        pub let role: String
    }

    pub resource Owner: Role {
        pub let role: String

        pub fun addAdmin(receiver: &AnyResource{Receiver}) {
            FanTopPermissionV2a.addPermission(receiver.owner!.address, role: "admin")
            receiver.receive(<- create Admin())
        }

        pub fun addPermission(_ address: Address, role: String) {
            FanTopPermissionV2a.addPermission(address, role: role)
        }

        pub fun removePermission(_ address: Address, role: String) {
            FanTopPermissionV2a.removePermission(address, role: role)
        }

        pub fun setItemMintedCount(itemId: String, mintedCount: UInt32) {
            FanTopToken.setItemMintedCount(itemId: itemId, mintedCount: mintedCount)
        }

        priv init() {
            self.role = "owner"
        }
    }

    pub resource Admin: Role {
        pub let role: String

        pub fun addOperator(receiver: &AnyResource{Receiver}) {
            FanTopPermissionV2a.addPermission(receiver.owner!.address, role: "operator")
            receiver.receive(<- create Operator())
        }

        pub fun removeOperator(_ address: Address) {
            FanTopPermissionV2a.removePermission(address, role: "operator")
        }

        pub fun addMinter(receiver: &AnyResource{Receiver}) {
            FanTopPermissionV2a.addPermission(receiver.owner!.address, role: "minter")
            receiver.receive(<- create Minter())
        }

        pub fun removeMinter(_ address: Address) {
            FanTopPermissionV2a.removePermission(address, role: "minter")
        }

        pub fun addAgent(receiver: &AnyResource{Receiver}) {
            FanTopPermissionV2a.addPermission(receiver.owner!.address, role: "agent")
            receiver.receive(<- create Agent())
        }

        pub fun removeAgent(_ address: Address) {
            FanTopPermissionV2a.removePermission(address, role: "agent")
        }

        pub fun extendMarketCapacity(_ capacity: Int) {
            FanTopMarket.extendCapacity(by: self.owner!.address, capacity: capacity)
        }

        priv init() {
            self.role = "admin"
        }
    }

    pub resource Operator: Role {
        pub let role: String

        pub fun createItem(itemId: String, version: UInt32, limit: UInt32, metadata: { String: String }, active: Bool) {
            FanTopToken.createItem(itemId: itemId, version: version, limit: limit, metadata: metadata, active: active)
        }

        pub fun updateMetadata(itemId: String, version: UInt32, metadata: { String: String }) {
            FanTopToken.updateMetadata(itemId: itemId, version: version, metadata: metadata)
        }

        pub fun updateLimit(itemId: String, limit: UInt32) {
            FanTopToken.updateLimit(itemId: itemId, limit: limit)
        }

        pub fun updateActive(itemId: String, active: Bool) {
            FanTopToken.updateActive(itemId: itemId, active: active)
        }

        pub fun truncateSerialBox(itemId: String, limit: Int) {
            let boxRef = FanTopSerial.getBoxRef(itemId: itemId) ?? panic("Boxes that do not exist cannot be truncated")
            boxRef.truncate(limit: limit)
        }

        priv init() {
            self.role = "operator"
        }
    }

    pub resource Minter: Role {
        pub let role: String

        pub fun mintToken(refId: String, itemId: String, itemVersion: UInt32, metadata: { String: String }): @FanTopToken.NFT {
            return <- FanTopToken.mintToken(refId: refId, itemId: itemId, itemVersion: itemVersion, metadata: metadata)
        }

        pub fun mintTokenWithSerialNumber(refId: String, itemId: String, itemVersion: UInt32, metadata: { String: String }, serialNumber: UInt32): @FanTopToken.NFT {
            return <- FanTopToken.mintTokenWithSerialNumber(refId: refId, itemId: itemId, itemVersion: itemVersion, metadata: metadata, serialNumber: serialNumber)
        }

        pub fun truncateSerialBox(itemId: String, limit: Int) {
            let boxRef = FanTopSerial.getBoxRef(itemId: itemId) ?? panic("Boxes that do not exist cannot be truncated")
            boxRef.truncate(limit: limit)
        }

        priv init() {
            self.role = "minter"
        }
    }

    pub resource Agent: Role {
        pub let role: String

        pub fun update(orderId: String, version: UInt32, metadata: { String: String }) {
            FanTopMarket.update(agent: self.owner!.address, orderId: orderId, version: version, metadata: metadata)
        }

        pub fun fulfill(orderId: String, version: UInt32, recipient: &AnyResource{FanTopToken.CollectionPublic}) {
            FanTopMarket.fulfill(agent: self.owner!.address, orderId: orderId, version: version, recipient: recipient)
        }

        pub fun cancel(orderId: String) {
            FanTopMarket.cancel(agent: self.owner!.address, orderId: orderId)
        }

        priv init() {
            self.role = "agent"
        }
    }

    pub struct User {
        pub fun sell(
            agent: Address,
            capability: Capability<&FanTopToken.Collection>,
            orderId: String,
            refId: String,
            nftId: UInt64,
            version: UInt32,
            metadata: [String],
            signature: [UInt8],
            keyIndex: Int
        ) {
            pre {
                keyIndex >= 0
                FanTopPermissionV2a.hasPermission(agent, role: "agent")
            }

            let account = getAccount(agent)
            var signedData = agent.toBytes()
                .concat(capability.address.toBytes())
                .concat(orderId.utf8)
                .concat(refId.utf8)
                .concat(nftId.toBigEndianBytes())
                .concat(version.toBigEndianBytes())

            let flatMetadata: { String: String } = {}
            var i = 0
            while i < metadata.length {
                let key = metadata[i]
                let value = metadata[i+1]

                signedData = signedData.concat(key.utf8).concat(value.utf8)
                flatMetadata[key] = value

                i = i + 2
            }

            signedData = signedData.concat(keyIndex.toString().utf8)

            assert(
                Signature.verify(
                    signature: signature,
                    signedData: signedData,
                    account: account,
                    keyIndex: keyIndex
                ),
                message: "Unverified orders cannot be fulfilled"
            )

            FanTopMarket.sell(
                agent: agent,
                capability: capability,
                orderId: orderId,
                refId: refId,
                nftId: nftId,
                version: version,
                metadata: flatMetadata
            )
        }
    }

    pub resource interface Receiver {
        pub fun receive(_ resource: @AnyResource{Role})
        pub fun check(address: Address): Bool
    }

    pub resource Holder: Receiver {
        priv let address: Address
        priv let resources: @{ String: AnyResource{Role} }

        pub fun check(address: Address): Bool {
            return address == self.owner?.address && address == self.address
        }

        pub fun receive(_ resource: @AnyResource{Role}) {
            assert(!self.resources.containsKey(resource.role), message: "Resources for roles that already exist cannot be received")
            self.resources[resource.role] <-! resource
        }

        priv fun borrow(by: AuthAccount, role: String): auth &AnyResource{Role} {
            pre {
                self.check(address: by.address): "Only borrowing by the owner is allowed"
                FanTopPermissionV2a.hasPermission(by.address, role: role): "Roles not on the list are not allowed"
            }
            return &self.resources[role] as! auth &AnyResource{Role}
        }

        pub fun borrowAdmin(by: AuthAccount): &Admin {
            return self.borrow(by: by, role: "admin") as! &Admin
        }

        pub fun borrowOperator(by: AuthAccount): &Operator {
            return self.borrow(by: by, role: "operator") as! &Operator
        }

        pub fun borrowMinter(by: AuthAccount): &Minter {
            return self.borrow(by: by, role: "minter") as! &Minter
        }

        pub fun borrowAgent(by: AuthAccount): &Agent {
            return self.borrow(by: by, role: "agent") as! &Agent
        }

        pub fun revoke(_ role: String) {
            pre {
                FanTopPermissionV2a.isRole(role): "Unknown role cannot be changed"
            }
            FanTopPermissionV2a.removePermission(self.address, role: role)
            destroy self.resources.remove(key: role)
        }

        access(contract) init(_ address: Address) {
            self.address = address
            self.resources <- {}
        }

        destroy() {
            for role in self.resources.keys {
                if FanTopPermissionV2a.hasPermission(self.address, role: role) {
                    FanTopPermissionV2a.removePermission(self.address, role: role)
                }
            }
            destroy self.resources
        }
    }

    pub fun createHolder(account: AuthAccount): @Holder {
        return <- create Holder(account.address)
    }

    priv let permissions: { Address: { String: Bool } }

    priv fun addPermission(_ address: Address, role: String) {
        pre {
            FanTopPermissionV2a.isRole(role): "Unknown role cannot be changed"
            role != "owner": "Owner cannot be changed"
            !self.hasPermission(address, role: role): "Permission that already exists cannot be added"
        }

        let permission = self.permissions[address] ?? {} as { String: Bool}
        permission[role] = true
        self.permissions[address] = permission

        emit PermissionAdded(target: address, role: role)
    }

    priv fun removePermission(_ address: Address, role: String) {
        pre {
            FanTopPermissionV2a.isRole(role): "Unknown role cannot be changed"
            role != "owner": "Owner cannot be changed"
            self.hasPermission(address, role: role): "Permissions that do not exist cannot be deleted"
        }

        let permission: {String: Bool} = self.permissions[address]!
        permission[role] = false
        self.permissions[address] = permission

        emit PermissionRemoved(target: address, role: role)
    }

    pub fun getAllPermissions(): { Address: { String: Bool } } {
        return self.permissions
    }

    pub fun hasPermission(_ address: Address, role: String): Bool {
        if let permission = self.permissions[address] {
            return permission[role] ?? false
        }

        return false
    }

    pub fun isRole(_ role: String): Bool {
        switch role {
        case "owner":
            return true
        case "admin":
            return true
        case "operator":
            return true
        case "minter":
            return true
        case "agent":
            return true
        default:
            return false
        }
    }

    init() {
        self.ownerStoragePath = /storage/FanTopOwnerV2a
        self.receiverStoragePath = /storage/FanTopPermissionV2a
        self.receiverPublicPath = /public/FanTopPermissionV2a

        self.permissions = {
            self.account.address: { "owner": true }
        }

        //self.account.save<@Owner>(<- create Owner(), to: self.ownerStoragePath)
    }
}

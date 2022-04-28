import DigitalContentAsset from 0xe1834889cb45b22f

pub contract DCAPermission {
    pub enum Role: UInt8 {
        pub case owner
        pub case admin
        pub case operator
        pub case minter
    }

    pub event PermissionAdded(target: Address, role: UInt8)
    pub event PermissionRemoved(target: Address, role: UInt8)

    pub let receiverStoragePath: StoragePath
    pub let receiverPublicPath: PublicPath

    pub resource Owner {
        pub fun addPermission(address: Address, as: Role) {
            DCAPermission.addPermission(address, as: as)
        }

        pub fun removePermission(address: Address, as: Role) {
            DCAPermission.removePermission(address, as: as)
        }

        pub fun addAdmin(receiver: &AnyResource{Receiver}) {
            pre {
                receiver.isValid(): "Receiver must be valid"
            }

            let recipient = receiver.owner!.address
            let capability = DCAPermission.addPermission(recipient, as: Role.admin) as! Capability<&Admin>

            receiver.receive(as: Role.admin, capability: capability)
        }

        access(contract) init() {
        }
    }

    pub resource Admin {
        pub fun addOperator(receiver: &AnyResource{Receiver}) {
            pre {
                receiver.isValid(): "Receiver must be valid"
            }

            let recipient = receiver.owner!.address
            let capability = DCAPermission.addPermission(recipient, as: Role.operator) as! Capability<&Operator>

            receiver.receive(as: Role.operator, capability: capability)
        }

        pub fun addMinter(receiver: &AnyResource{Receiver}) {
            pre {
                receiver.isValid(): "Receiver must be valid"
            }

            let recipient = receiver.owner!.address
            let capability = DCAPermission.addPermission(recipient, as: Role.minter) as! Capability<&Minter>

            receiver.receive(as: Role.minter, capability: capability)
        }

        pub fun removeOperator(_ address: Address) {
            DCAPermission.removePermission(address, as: Role.operator)
        }

        pub fun removeMinter(_ address: Address) {
            DCAPermission.removePermission(address, as: Role.minter)
        }

        access(contract) init() {
        }
    }

    pub resource Operator {
        pub fun createItem(itemId: String, version: UInt32, limit: UInt32, metadata: { String: String }, active: Bool) {
            DigitalContentAsset.createItem(itemId: itemId, version: version, limit: limit, metadata: metadata, active: active)
        }

        pub fun updateMetadata(itemId: String, version: UInt32, metadata: { String: String }) {
            DigitalContentAsset.updateMetadata(itemId: itemId, version: version, metadata: metadata)
        }

        pub fun updateLimit(itemId: String, limit: UInt32) {
            DigitalContentAsset.updateLimit(itemId: itemId, limit: limit)
        }

        pub fun updateActive(itemId: String, active: Bool) {
            DigitalContentAsset.updateActive(itemId: itemId, active: active)
        }

        access(contract) init() {
        }
    }

    pub resource Minter {
        pub fun mintToken(refId: String, itemId: String, itemVersion: UInt32, metadata: { String: String }): @DigitalContentAsset.NFT {
            return <- DigitalContentAsset.mintToken(refId: refId, itemId: itemId, itemVersion: itemVersion, metadata: metadata)
        }

        access(contract) init() {
        }
    }

    pub resource interface Receiver {
        pub fun receive(as: Role, capability: Capability)
        pub fun isValid(): Bool
    }

    pub resource Holder: Receiver {
        access(contract) let recipient: Address
        access(self) let roles: { Role: Capability }

        pub fun isValid(): Bool {
            return self.owner?.address == self.recipient
        }

        pub fun receive(as: Role, capability: Capability) {
            pre {
                !self.roles.containsKey(as): "Holder cannot receive roles it already owns"
            }
            self.roles[as] = capability
        }

        pub fun borrowAdmin(by: AuthAccount): &Admin {
            pre {
                self.isValid(): "Invalid holder cannot be used"
                self.recipient == by.address: "Only the owner can borrow"
                self.roles.containsKey(Role.admin): "Roles not given cannot be borrowed"
                DCAPermission.hasPermission(by.address, as: Role.admin): "Roles without permission cannot be used"
            }

            let role = self.roles[Role.admin]!
            return role.borrow<&Admin>()!
        }

        pub fun borrowOperator(by: AuthAccount): &Operator {
            pre {
                self.isValid(): "Invalid holder cannot be used"
                self.recipient == by.address: "Only the owner can borrow"
                self.roles.containsKey(Role.operator): "Roles not given cannot be borrowed"
                DCAPermission.hasPermission(by.address, as: Role.operator): "Roles without permission cannot be used"
            }

            let role = self.roles[Role.operator]!
            return role.borrow<&Operator>()!
        }

        pub fun borrowMinter(by: AuthAccount): &Minter {
            pre {
                self.isValid(): "Invalid holder cannot be used"
                self.recipient == by.address: "Only the owner can borrow"
                self.roles.containsKey(Role.minter): "Roles not given cannot be borrowed"
                DCAPermission.hasPermission(by.address, as: Role.minter): "Roles without permission cannot be used"
            }

            let role = self.roles[Role.minter]!
            return role.borrow<&Minter>()!
        }

        access(contract) init(recipient: Address) {
            self.recipient = recipient
            self.roles = {}
        }
    }

    access(self) let permissions: { Address: { Role: Bool } }
    access(self) let capabilities: { Role: Capability }

    access(self) fun addPermission(_ address: Address, as: Role): Capability {
        pre {
            !DCAPermission.hasPermission(address, as: as): "Existing roles are not added"
        }

        if let permission = DCAPermission.permissions[address] {
            permission.insert(key: as, true)
            DCAPermission.permissions.insert(key: address, permission)
        } else {
            DCAPermission.permissions.insert(key: address, { as: true })
        }

        emit PermissionAdded(target: address, role: as.rawValue)

        return self.capabilities[as]!
    }

    access(self) fun removePermission(_ address: Address, as: Role) {
        pre {
            DCAPermission.hasPermission(address, as: as): "Roles that do not exist cannot be removed"
        }
        let permission = DCAPermission.permissions.remove(key: address)!

        permission.remove(key: as)

        if permission.keys.length > 0 {
            DCAPermission.permissions.insert(key: address, permission)
        }

        emit PermissionRemoved(target: address, role: as.rawValue)
    }

    pub fun createHolder(account: AuthAccount): @Holder {
        return <- create Holder(recipient: account.address)
    }

    pub fun getAllPermissions(): { Address: { Role: Bool } } {
        return self.permissions
    }

    pub fun hasPermission(_ address: Address, as: Role): Bool {
        if let permission = DCAPermission.permissions[address] {
            return permission[as] ?? false
        }
        return false
    }

    init() {
        self.receiverStoragePath = /storage/DCAPermission
        self.receiverPublicPath = /public/DCAPermission

        self.permissions = {}
        self.account.save<@Owner>(<- create Owner(), to: /storage/DCAOwner)
        self.account.save<@Admin>(<- create Admin(), to: /storage/DCAAdmin)
        self.account.save<@Operator>(<- create Operator(), to: /storage/DCAOperator)
        self.account.save<@Minter>(<- create Minter(), to: /storage/DCAMinter)

        self.capabilities = {}
        self.capabilities[Role.admin] = self.account.link<&Admin>(/private/DCAAdmin, target: /storage/DCAAdmin)!
        self.capabilities[Role.operator] = self.account.link<&Operator>(/private/DCAOperator, target: /storage/DCAOperator)!
        self.capabilities[Role.minter] = self.account.link<&Minter>(/private/DCAMinter, target: /storage/DCAMinter)!
    }
}

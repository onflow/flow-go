/*

 This contract defines the Flovatar Component Templates and the Collection to manage them.
 While Components are the building blocks (lego bricks) of the final Flovatar, 
 Templates are the blueprint where all the details and characteristics of each component are defined.
 The main part is the SVG (stored on-chain as a String) and the category that can be one of the following: 
 body, hair, facialHair, eyes, nose, mouth, clothing, accessory, hat, eyeglasses, background.

 Templates are NOT using the NFT standard and will be always linked only to the contract's owner account.

 Each templates will also declare in advance the maximum amount of mintable components, so that 
 the scarcity can be enforced by the smart contract itself and so that different rarities can be guaranteed.

 Finally, Templates are organized in Series, so that in the future it will be possible to create new editions
 with different characters and styles.

 */

pub contract FlovatarComponentTemplate {

    pub let CollectionStoragePath: StoragePath
    pub let CollectionPublicPath: PublicPath


    // Counter for all the Templates ever minted
    pub var totalSupply: UInt64
    //These counters will keep track of how many Components were minted for each Template
    access(contract) let totalMintedComponents: { UInt64: UInt64 }
    access(contract) let lastComponentMintedAt: { UInt64: UFix64 }

    // Event to notify about the Template creation
    pub event ContractInitialized()
    pub event Created(id: UInt64, name: String, category: String, color: String, maxMintableComponents: UInt64)

    // The public interface providing the SVG and all the other 
    // metadata like name, category, color, series, description and 
    // the maximum mintable Components
    pub resource interface Public {
        pub let id: UInt64
        pub let name: String
        pub let category: String
        pub let color: String
        pub let description: String
        pub let svg: String
        pub let series: UInt32
        pub let maxMintableComponents: UInt64
        pub let rarity: String
    }

    // The Component resource implementing the public interface as well
    pub resource ComponentTemplate: Public {
        pub let id: UInt64
        pub let name: String
        pub let category: String
        pub let color: String
        pub let description: String
        pub let svg: String
        pub let series: UInt32
        pub let maxMintableComponents: UInt64
        pub let rarity: String

        // Initialize a Template with all the necessary data
        init(
            name: String,
            category: String,
            color: String,
            description: String,
            svg: String,
            series: UInt32,
            maxMintableComponents: UInt64,
            rarity: String
        ) {
            // increments the counter and stores it as the ID
            FlovatarComponentTemplate.totalSupply = FlovatarComponentTemplate.totalSupply + UInt64(1)
            self.id = FlovatarComponentTemplate.totalSupply
            self.name = name
            self.category = category
            self.color = color
            self.description = description
            self.svg = svg
            self.series = series
            self.maxMintableComponents = maxMintableComponents
            self.rarity = rarity
        }
    }

    // Standard CollectionPublic interface that can also borrow Component Templates
    pub resource interface CollectionPublic {
        pub fun getIDs(): [UInt64]
        pub fun borrowComponentTemplate(id: UInt64): &{FlovatarComponentTemplate.Public}?
    }

    // The main Collection that manages the Templates and that implements also the Public interface
    pub resource Collection: CollectionPublic {
        // Dictionary of Component Templates
        pub var ownedComponentTemplates: @{UInt64: FlovatarComponentTemplate.ComponentTemplate}

        init () {
            self.ownedComponentTemplates <- {}
        }

        

        // deposit takes a Component Template and adds it to the collections dictionary
        // and adds the ID to the id array
        pub fun deposit(componentTemplate: @FlovatarComponentTemplate.ComponentTemplate) {

            let id: UInt64 = componentTemplate.id

            // add the new Component Template to the dictionary which removes the old one
            let oldComponentTemplate <- self.ownedComponentTemplates[id] <- componentTemplate

            destroy oldComponentTemplate
        }

        // getIDs returns an array of the IDs that are in the collection
        pub fun getIDs(): [UInt64] {
            return self.ownedComponentTemplates.keys
        }

        // borrowComponentTemplate returns a borrowed reference to a Component Template
        // so that the caller can read data and call methods from it.
        pub fun borrowComponentTemplate(id: UInt64): &{FlovatarComponentTemplate.Public}? {
            if self.ownedComponentTemplates[id] != nil {
                let ref = &self.ownedComponentTemplates[id] as auth &FlovatarComponentTemplate.ComponentTemplate
                return ref as! &FlovatarComponentTemplate.ComponentTemplate
            } else {
                return nil
            }
        }

        destroy() {
            destroy self.ownedComponentTemplates
        }
    }

    // This function can only be called by the account owner to create an empty Collection
    access(account) fun createEmptyCollection(): @FlovatarComponentTemplate.Collection {
        return <- create Collection()
    }


    // This struct is used to send a data representation of the Templates 
    // when retrieved using the contract helper methods outside the collection.
    pub struct ComponentTemplateData {
        pub let id: UInt64
        pub let name: String
        pub let category: String
        pub let color: String
        pub let description: String
        pub let svg: String?
        pub let series: UInt32
        pub let maxMintableComponents: UInt64
        pub let totalMintedComponents: UInt64
        pub let lastComponentMintedAt: UFix64
        pub let rarity: String

        init(
            id: UInt64,
            name: String,
            category: String,
            color: String,
            description: String,
            svg: String?,
            series: UInt32,
            maxMintableComponents: UInt64,
            rarity: String
        ) {
            self.id = id
            self.name = name
            self.category = category
            self.color = color
            self.description = description
            self.svg = svg
            self.series = series
            self.maxMintableComponents = maxMintableComponents
            self.totalMintedComponents = FlovatarComponentTemplate.getTotalMintedComponents(id: id)!
            self.lastComponentMintedAt = FlovatarComponentTemplate.getLastComponentMintedAt(id: id)!
            self.rarity = rarity
        }
    }

    // Get all the Component Templates from the account. 
    // We hide the SVG field because it might be too big to execute in a script
    pub fun getComponentTemplates() : [ComponentTemplateData] {
        var componentTemplateData: [ComponentTemplateData] = []

        if let componentTemplateCollection = self.account.getCapability(self.CollectionPublicPath).borrow<&{FlovatarComponentTemplate.CollectionPublic}>()  {
            for id in componentTemplateCollection.getIDs() {
                var componentTemplate = componentTemplateCollection.borrowComponentTemplate(id: id)
                componentTemplateData.append(ComponentTemplateData(
                    id: id,
                    name: componentTemplate!.name,
                    category: componentTemplate!.category,
                    color: componentTemplate!.color,
                    description: componentTemplate!.description,
                    svg: nil,
                    series: componentTemplate!.series,
                    maxMintableComponents: componentTemplate!.maxMintableComponents,
                    rarity: componentTemplate!.rarity
                    ))
            }
        }
        return componentTemplateData
    }

    // Gets a specific Template from its ID
    pub fun getComponentTemplate(id: UInt64) : ComponentTemplateData? {
        if let componentTemplateCollection = self.account.getCapability(self.CollectionPublicPath).borrow<&{FlovatarComponentTemplate.CollectionPublic}>()  {
            if let componentTemplate = componentTemplateCollection.borrowComponentTemplate(id: id) {
                return ComponentTemplateData(
                    id: id,
                    name: componentTemplate!.name,
                    category: componentTemplate!.category,
                    color: componentTemplate!.color,
                    description: componentTemplate!.description,
                    svg: componentTemplate!.svg,
                    series: componentTemplate!.series,
                    maxMintableComponents: componentTemplate!.maxMintableComponents,
                    rarity: componentTemplate!.rarity
                    )
            }
        }
        return nil
    }

    // Returns the amount of minted Components for a specific Template
    pub fun getTotalMintedComponents(id: UInt64) : UInt64? {
        return FlovatarComponentTemplate.totalMintedComponents[id]
    }
    // Returns the timestamp of the last time a Component for a specific Template was minted
    pub fun getLastComponentMintedAt(id: UInt64) : UFix64? {
        return FlovatarComponentTemplate.lastComponentMintedAt[id]
    }

    // This function is used within the contract to set the new counter for each Template
    access(account) fun setTotalMintedComponents(id: UInt64, value: UInt64) {
        FlovatarComponentTemplate.totalMintedComponents[id] = value
    }
    // This function is used within the contract to set the timestamp 
    // when a Component for a specific Template was minted
    access(account) fun setLastComponentMintedAt(id: UInt64, value: UFix64) {
        FlovatarComponentTemplate.lastComponentMintedAt[id] = value
    }

    // It creates a new Template with the data provided.
    // This is used from the Flovatar Admin resource
    access(account) fun createComponentTemplate(
        name: String,
        category: String,
        color: String,
        description: String,
        svg: String,
        series: UInt32,
        maxMintableComponents: UInt64,
        rarity: String
    ) : @FlovatarComponentTemplate.ComponentTemplate {

        var newComponentTemplate <- create ComponentTemplate(
            name: name,
            category: category,
            color: color,
            description: description,
            svg: svg,
            series: series,
            maxMintableComponents: maxMintableComponents,
            rarity: rarity
        )

        // Emits the Created event to notify about the new Template
        emit Created(id: newComponentTemplate.id, name: newComponentTemplate.name, category: newComponentTemplate.category, color: newComponentTemplate.color, maxMintableComponents: newComponentTemplate.maxMintableComponents)

        // Set the counter for the minted Components of this Template to 0
        FlovatarComponentTemplate.setTotalMintedComponents(id: newComponentTemplate.id, value: UInt64(0))
        FlovatarComponentTemplate.setLastComponentMintedAt(id: newComponentTemplate.id, value: UFix64(0))

        return <- newComponentTemplate
    }

	init() {
        self.CollectionPublicPath=/public/FlovatarComponentTemplateCollection
        self.CollectionStoragePath=/storage/FlovatarComponentTemplateCollection

        // Initialize the total supply
        self.totalSupply = 0
        self.totalMintedComponents = {}
        self.lastComponentMintedAt = {}

        self.account.save<@FlovatarComponentTemplate.Collection>(<- FlovatarComponentTemplate.createEmptyCollection(), to: FlovatarComponentTemplate.CollectionStoragePath)
        self.account.link<&{FlovatarComponentTemplate.CollectionPublic}>(FlovatarComponentTemplate.CollectionPublicPath, target: FlovatarComponentTemplate.CollectionStoragePath)

        emit ContractInitialized()
	}
}

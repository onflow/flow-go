// This is not meant to be real working code, this is just an example 
// showing how a composable resource could be created

// In this example, there are different types of resources that 
// represent different types of vehicle parts
// The parts can have different types and materials, represented by 
// enum integers (I don't think we have enums yet)

// There is also the vehicle resourse, which stores these parts.
// Depending on the parts that are added to the vehicle
// the vehicle can be a bicycle, a motorcycle, or a car.
// The types and materials of the parts also determine 
// how fast the vehicle can go


pub resource interface Part {
    pub let type: Int
    pub let material: Int
}

pub resource Wheel: Part {
    pub let type: Int
    pub let material: Int

    init(type: Int, material: Int) {
        self.type = type
        self.material = material
    }
}

pub resource Frame: Part  {
    pub let type: Int
    pub let material: Int

    init(type: Int, material: Int) {
        self.type = type
        self.material = material
    }
}

pub resource Engine: Part  {
    pub let type: Int
    pub let material: Int

    init(type: Int, material: Int) {
        self.type = type
        self.material = material
    }
}

pub resource Decals {
    pub let type: Int
    pub let material: Int

    init(type: Int, material: Int) {
        self.type = type
        self.material = material
    }
}

pub resource PartCollection {
    pub fun store(part: <-Part) {
        destroy part
    }
}

pub resource Vehicle {

    pub var wheels: <-[Wheel]

    pub var frame: <-Frame?

    pub var engine: <-Engine?

    pub var decals: <-[Decals]

    // PartCollection isn't defined, but here we just assume
    // that it is similar to the NFTCollection resource defined in the NFT 
    // example nft.cdc
    pub var collection: &PartCollection

    init(collection: &PartCollection) {
        self.wheels <- []
        self.frame <- nil
        self.engine <- nil
        self.decals <- []
        self.collection = collection
    }

    pub fun addWheel(newWheel: <-Wheel) {
        self.wheels.append(<-newWheel)
    }

    pub fun removeWheel(wheelNum: Int) {
        pre {
            wheelNum < self.wheels.length:
                "Wheel doesn't exist!"
        }
        self.collection.store(part: <-self.wheels.remove(at: wheelNum))
    }

    pub fun addFrame(newFrame: <-Frame) {
        let oldFrame <- self.frame <- newFrame
        if let frame <- oldFrame {
            self.collection.store(part: <-frame)
        } else {
            destroy oldFrame
        }
    }

    pub fun removeFrame() {
        if let frame <- self.frame <- nil {
            self.collection.store(part: <-frame)
        }
    }

    pub fun addEngine(newEngine: <-Engine) {
        let oldEngine <- self.engine <- newEngine
        if let engine <- oldEngine {
            self.collection.store(part: <-engine)
        } else {
            destroy oldEngine
        }
    }

    pub fun removeEngine() {
        if let engine <- self.engine <- nil {
            self.collection.store(part: <-engine)
        }
    }

    // vehicleType returns the type of the vehicle
    // based on which parts are installed
    // 1 = bicycle
    // 2 = motorcycle
    // 3 = car
    pub fun vehicleType(): Int {
        // get the frame and engine types
        let frameType = self.frame?.type ?? 0
        let engineType = self.engine?.type ?? 0

        
        if self.wheels.length == 2 {
            if frameType == 1 && engineType == 0 {
                // bicycle
                return 1
            } else if frameType == 2 && engineType == 1 {
                // motorcycle
                return 2
            } else {
                // nothing
                return 0
            }
        } else if self.wheels.length == 4 {
            if frameType == 3 && (engineType == 3 || engineType == 4) {
                // car
                return 3
            }
        }

        return 0
    }

    pub fun getSpeed(): Int {
        let frameMaterial = self.frame?.material ?? 0
        let wheelMaterial = self.wheels[1].material
        let engineMaterial = self.engine?.material ?? 0

        if self.vehicleType() == 1 {
            return frameMaterial * 2 + wheelMaterial * 2
        } else if self.vehicleType() == 2 {
            return frameMaterial * 3 + wheelMaterial * 3 + engineMaterial
        } else if self.vehicleType() == 3 {
            return frameMaterial * 4 + wheelMaterial * 4 + engineMaterial * 2
        }

        return 0
    }

    destroy() {
        destroy self.wheels
        destroy self.frame
        destroy self.engine
        destroy self.decals
    }


}
